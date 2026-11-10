package message

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"kafka-management-platform/internal/logger"
	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/repository"
	"kafka-management-platform/internal/service/auth"
	"kafka-management-platform/pkg/encryption"
	"kafka-management-platform/pkg/kafka"
)

// 查询模式
const (
	QueryTypeOffset    = "offset"    // 按 offset 范围
	QueryTypeTimestamp = "timestamp" // 按时间戳
	QueryTypeKey       = "key"       // 按 key 过滤（在 offset 范围内客户端过滤）
	QueryTypeLatest    = "latest"    // 最新 N 条
	QueryTypeEarliest  = "earliest"  // 最旧 N 条
)

// MaxMessageCount 单次查询条数上限
const MaxMessageCount = 500

var (
	// ErrPermissionDenied 无权限访问该 topic
	ErrPermissionDenied = errors.New("permission denied: no access to this topic")
	// ErrInvalidRequest 请求参数无效
	ErrInvalidRequest = errors.New("invalid request parameters")
)

// QueryMessagesRequest 消息查询请求
type QueryMessagesRequest struct {
	Partition   int32  `json:"partition"`
	QueryType   string `json:"query_type"`
	StartOffset int64  `json:"start_offset"`
	EndOffset   int64  `json:"end_offset"`
	Timestamp   int64  `json:"timestamp"` // 毫秒，query_type=timestamp 时生效
	Key         string `json:"key"`       // query_type=key 时生效
	MaxCount    int64  `json:"max_count"`
}

// QueryMessagesResponse 消息查询响应
type QueryMessagesResponse struct {
	Data             []kafka.MessageRecord `json:"data"`
	PartitionOffsets PartitionOffsets       `json:"partition_offsets"`
	Total            int                    `json:"total"`
}

// PartitionOffsets 分区 offset 信息
type PartitionOffsets struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

// Service 消息查询服务
type Service struct {
	clusterRepo     repository.ClusterRepository
	encryptionSvc   *encryption.Service
	permissionSvc   *auth.PermissionService
	kerberosBaseDir string
}

// NewService 创建消息查询服务实例
func NewService(
	clusterRepo repository.ClusterRepository,
	encryptionSvc *encryption.Service,
	permissionSvc *auth.PermissionService,
	kerberosBaseDir string,
) *Service {
	return &Service{
		clusterRepo:     clusterRepo,
		encryptionSvc:   encryptionSvc,
		permissionSvc:   permissionSvc,
		kerberosBaseDir: kerberosBaseDir,
	}
}

// QueryMessages 查询消息。
// 权限校验在任何 Kafka 连接之前执行：管理员角色查所有 topic，
// 普通用户必须通过 CheckTopicPermission（user_topic_permission 表）。
func (s *Service) QueryMessages(ctx context.Context, userID int64, role string, clusterID int64, topicName string, req *QueryMessagesRequest) (*QueryMessagesResponse, error) {
	// 1. 权限校验（入口第一件事，在任何 Kafka 连接之前，避免无权限用户探测 topic）
	if role != string(models.RoleSuperAdmin) && role != string(models.RoleClusterAdmin) {
		allowed, err := s.permissionSvc.CheckTopicPermission(ctx, userID, clusterID, topicName)
		if err != nil {
			logger.Error("failed to check topic permission",
				"userID", userID, "clusterID", clusterID, "topic", topicName, "error", err)
			return nil, ErrPermissionDenied
		}
		if !allowed {
			return nil, ErrPermissionDenied
		}
	}

	// 2. 参数规范化（服务端强制 clamp，不信任客户端）
	if req.QueryType == "" {
		req.QueryType = QueryTypeLatest
	}
	if req.Partition < 0 {
		return nil, fmt.Errorf("%w: partition must be >= 0", ErrInvalidRequest)
	}
	if req.MaxCount <= 0 {
		req.MaxCount = 100
	}
	if req.MaxCount > MaxMessageCount {
		req.MaxCount = MaxMessageCount
	}

	// 3. 建立连接（权限已通过）
	adminClient, err := s.getAdminClient(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	defer adminClient.Close()

	// 4. 获取分区 offset 范围
	partStart, partEnd, err := adminClient.GetPartitionOffsets(topicName, req.Partition)
	if err != nil {
		return nil, fmt.Errorf("failed to get partition offsets: %w", err)
	}
	if partEnd <= partStart {
		// 空分区
		return &QueryMessagesResponse{
			Data:             []kafka.MessageRecord{},
			PartitionOffsets: PartitionOffsets{Start: partStart, End: partEnd},
			Total:            0,
		}, nil
	}

	// 5. timestamp 模式：先转换为起始 offset
	if req.QueryType == QueryTypeTimestamp {
		if req.Timestamp <= 0 {
			return nil, fmt.Errorf("%w: timestamp is required for timestamp query", ErrInvalidRequest)
		}
		offset, err := adminClient.OffsetForTime(topicName, req.Partition, req.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("failed to convert timestamp to offset: %w", err)
		}
		req.StartOffset = offset
	}

	// 6. 计算实际 fetch 范围
	fetchStart, fetchCount := computeFetchRange(req, partStart, partEnd)
	if fetchCount <= 0 {
		return &QueryMessagesResponse{
			Data:             []kafka.MessageRecord{},
			PartitionOffsets: PartitionOffsets{Start: partStart, End: partEnd},
			Total:            0,
		}, nil
	}

	// 7. 拉取消息（只读 FetchRequest）
	records, err := adminClient.FetchMessages(topicName, req.Partition, fetchStart, fetchCount)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}

	// 8. key 模式：客户端按 key 子串过滤
	if req.QueryType == QueryTypeKey && req.Key != "" {
		filtered := records[:0]
		for _, r := range records {
			if strings.Contains(r.Key, req.Key) {
				filtered = append(filtered, r)
			}
		}
		records = filtered
	}

	return &QueryMessagesResponse{
		Data:             records,
		PartitionOffsets: PartitionOffsets{Start: partStart, End: partEnd},
		Total:            len(records),
	}, nil
}

// computeFetchRange 根据查询模式计算 fetch 起始 offset 和拉取条数（均 clamp 到分区可用范围内）
func computeFetchRange(req *QueryMessagesRequest, partStart, partEnd int64) (int64, int64) {
	switch req.QueryType {
	case QueryTypeLatest:
		count := min64(req.MaxCount, partEnd-partStart)
		return partEnd - count, count

	case QueryTypeEarliest:
		count := min64(req.MaxCount, partEnd-partStart)
		return partStart, count

	case QueryTypeTimestamp:
		start := clampInt64(req.StartOffset, partStart, partEnd)
		count := min64(req.MaxCount, partEnd-start)
		return start, count

	case QueryTypeKey:
		// 在 [start_offset, end_offset) 范围内扫描并按 key 过滤
		start := max64(req.StartOffset, partStart)
		end := req.EndOffset
		if end <= 0 || end > partEnd {
			end = partEnd
		}
		if start >= end {
			return 0, 0
		}
		return start, min64(req.MaxCount, end-start)

	case QueryTypeOffset:
		fallthrough
	default:
		// offset 模式：[start_offset, end_offset) 范围
		start := max64(req.StartOffset, partStart)
		end := req.EndOffset
		if end <= 0 || end > partEnd {
			end = partEnd
		}
		if start >= end {
			return 0, 0
		}
		return start, min64(req.MaxCount, end-start)
	}
}

// getAdminClient 获取 Kafka AdminClient（取集群 + 解密认证信息，与 topic_service 的 getClusterWithAuth 相同模板）
func (s *Service) getAdminClient(ctx context.Context, clusterID int64) (*kafka.AdminClient, error) {
	cluster, err := s.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	var authConfigJSON string
	if cluster.AuthConfig != "" {
		decrypted, err := s.encryptionSvc.DecryptString(cluster.AuthConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt auth config: %w", err)
		}
		authConfigJSON = decrypted
	}

	adminClient, err := kafka.NewAdminClientWithKerberos(cluster, authConfigJSON, s.kerberosBaseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create admin client: %w", err)
	}
	return adminClient, nil
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func clampInt64(v, lo, hi int64) int64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
