package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"kafka-management-platform/internal/cache"
	"kafka-management-platform/internal/logger"
	"kafka-management-platform/internal/repository"
	"kafka-management-platform/internal/service/monitor"
)

// OverviewResponse Dashboard 概览响应
type OverviewResponse struct {
	Clusters              ClusterStats           `json:"clusters"`
	TopicsTotal           int64                  `json:"topics_total"`
	BrokersOnline         *int                   `json:"brokers_online"`   // nil when VM unreachable
	PartitionsTotal       *int                   `json:"partitions_total"` // nil when VM unreachable
	UsersTotal            int64                  `json:"users_total"`
	ConsumerGroups        *ConsumerGroupStats    `json:"consumer_groups"` // nil when VM unreachable
	ConsumerGroupDetails  []ConsumerGroupDetail  `json:"consumer_group_details"`
	AuthTypeDist          map[string]int         `json:"auth_type_distribution"`
	ClusterSizes          []ClusterSizeItem      `json:"cluster_sizes"`
}

// ClusterStats 集群统计
type ClusterStats struct {
	Total   int `json:"total"`
	Healthy int `json:"healthy"`
	Warning int `json:"warning"`
	Error   int `json:"error"`
	Unknown int `json:"unknown"`
}

// ConsumerGroupStats 消费者组统计
type ConsumerGroupStats struct {
	Total    int   `json:"total"`
	TotalLag int64 `json:"total_lag"`
}

// ConsumerGroupDetail 消费者组详情（按集群分组）
type ConsumerGroupDetail struct {
	ClusterName string `json:"cluster_name"`
	GroupID     string `json:"group_id"`
	Topic       string `json:"topic"`
	TotalLag    int64  `json:"total_lag"`
	MemberCount int    `json:"member_count"`
}

// ClusterSizeItem 集群规模条目
type ClusterSizeItem struct {
	ClusterID    int64  `json:"cluster_id"`
	ClusterName  string `json:"cluster_name"`
	BrokerCount  *int   `json:"broker_count"`   // nil when VM unreachable
	TopicCount   int64  `json:"topic_count"`
	HealthStatus string `json:"health_status"`
}

// Service Dashboard 业务逻辑
type Service struct {
	clusterRepo repository.ClusterRepository
	topicRepo   repository.TopicRepository
	userRepo    repository.UserRepository
	monitorSvc  *monitor.Service
	cache       *cache.MemoryCache
}

// NewService 创建 Dashboard 服务
func NewService(
	clusterRepo repository.ClusterRepository,
	topicRepo repository.TopicRepository,
	userRepo repository.UserRepository,
	monitorSvc *monitor.Service,
) *Service {
	return &Service{
		clusterRepo: clusterRepo,
		topicRepo:   topicRepo,
		userRepo:    userRepo,
		monitorSvc:  monitorSvc,
		cache:       cache.NewMemoryCache(30 * time.Second),
	}
}

const overviewCacheKey = "dashboard:overview"

// GetOverview 获取 Dashboard 概览数据（带 30s 缓存）
func (s *Service) GetOverview(ctx context.Context) (*OverviewResponse, error) {
	// 检查缓存
	if cached, err := s.cache.Get(ctx, overviewCacheKey); err == nil && cached != nil {
		if data, ok := cached.([]byte); ok {
			var resp OverviewResponse
			if err := json.Unmarshal(data, &resp); err == nil {
				return &resp, nil
			}
		}
	}

	resp := &OverviewResponse{
		AuthTypeDist: make(map[string]int),
	}

	// 1. MySQL 查询
	clusters, _, err := s.clusterRepo.List(ctx, 0, 10000)
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}

	for _, c := range clusters {
		resp.AuthTypeDist[string(c.AuthType)]++
	}
	resp.Clusters.Total = len(clusters)

	// Topic 总数
	topicCount, err := s.topicRepo.Count(ctx)
	if err != nil {
		logger.Warn("Failed to count topics", "error", err)
	}
	resp.TopicsTotal = topicCount

	// 活跃用户总数
	userCount, err := s.userRepo.CountActive(ctx)
	if err != nil {
		logger.Warn("Failed to count active users", "error", err)
	}
	resp.UsersTotal = userCount

	// 2. VM 聚合数据
	clusterIDs := make([]int64, len(clusters))
	for i, c := range clusters {
		clusterIDs[i] = c.ClusterID
	}

	vmData := s.monitorSvc.GetDashboardVMData(ctx, clusterIDs)
	if vmData != nil {
		if vmData.TotalBrokers > 0 {
			resp.BrokersOnline = &vmData.TotalBrokers
		}
		if vmData.TotalPartitions > 0 {
			resp.PartitionsTotal = &vmData.TotalPartitions
		}
		if vmData.CGTotal > 0 || vmData.CGLag > 0 {
			resp.ConsumerGroups = &ConsumerGroupStats{
				Total:    vmData.CGTotal,
				TotalLag: vmData.CGLag,
			}
		}
		// 转换 CGDetails（需要从 clusters 映射 cluster_name）
		if len(vmData.CGDetails) > 0 {
			clusterNameMap := make(map[int64]string, len(clusters))
			for _, c := range clusters {
				clusterNameMap[c.ClusterID] = c.ClusterName
			}
			details := make([]ConsumerGroupDetail, len(vmData.CGDetails))
			for i, d := range vmData.CGDetails {
				details[i] = ConsumerGroupDetail{
					ClusterName: clusterNameMap[d.ClusterID],
					GroupID:     d.GroupID,
					Topic:       d.Topic,
					TotalLag:    d.TotalLag,
					MemberCount: d.MemberCount,
				}
			}
			resp.ConsumerGroupDetails = details
		}
	}

	// 2.1 如果 VM 无数据或数据不完整，通过 AdminClient 补充
	// 只要 VM 的 broker/partition/CG 任一为空就触发 AdminClient 兜底
	needAdminClientFallback := resp.BrokersOnline == nil || resp.PartitionsTotal == nil || resp.ConsumerGroups == nil
	var adminClientBrokerCounts map[int64]int
	if needAdminClientFallback {
		// Broker 兜底
		if resp.BrokersOnline == nil {
			adminClientBrokerCounts = s.monitorSvc.GetAdminClientBrokerCounts(ctx, clusterIDs)
			totalBrokers := 0
			for _, bc := range adminClientBrokerCounts {
				totalBrokers += bc
			}
			if totalBrokers > 0 {
				resp.BrokersOnline = &totalBrokers
			}
		}
		// Partition 兜底
		if resp.PartitionsTotal == nil {
			partitionCount := s.monitorSvc.GetAdminClientPartitionCounts(ctx, clusterIDs)
			if partitionCount > 0 {
				resp.PartitionsTotal = &partitionCount
			}
		}
		// Consumer Group 兜底
		if resp.ConsumerGroups == nil {
			cgTotal, cgLag := s.monitorSvc.GetAdminClientConsumerGroupStats(ctx, clusterIDs)
			if cgTotal > 0 || cgLag > 0 {
				resp.ConsumerGroups = &ConsumerGroupStats{
					Total:    cgTotal,
					TotalLag: cgLag,
				}
			}
		}
	}

	// 3. Per-cluster topic count
	topicCountByCluster, err := s.topicRepo.CountByCluster(ctx)
	if err != nil {
		logger.Warn("Failed to count topics by cluster", "error", err)
		topicCountByCluster = make(map[int64]int64)
	}

	// 4. 构建 cluster_sizes + 统计健康状态
	healthy, warning, errorN, unknown := 0, 0, 0, 0
	resp.ClusterSizes = make([]ClusterSizeItem, 0, len(clusters))

	for _, c := range clusters {
		item := ClusterSizeItem{
			ClusterID:   c.ClusterID,
			ClusterName: c.ClusterName,
			TopicCount:  topicCountByCluster[c.ClusterID],
		}

		if vmData != nil {
			if bc, ok := vmData.BrokerCountByCluster[c.ClusterID]; ok {
				item.BrokerCount = &bc
			}
			item.HealthStatus = vmData.HealthStatusByCluster[c.ClusterID]
			// VM 没有该集群的健康数据时，用 MySQL 数据兜底
			if item.HealthStatus == "" || item.HealthStatus == "unknown" {
				if item.TopicCount > 0 {
					item.HealthStatus = "healthy"
				}
			}
		} else {
			// VM 完全不可达：用 MySQL + AdminClient 数据判定
			if item.TopicCount > 0 {
				item.HealthStatus = "healthy"
			} else {
				item.HealthStatus = "unknown"
			}
		}
		// VM 没有 broker_count 时，用 AdminClient 数据补充
		if item.BrokerCount == nil && adminClientBrokerCounts != nil {
			if bc, ok := adminClientBrokerCounts[c.ClusterID]; ok && bc > 0 {
				item.BrokerCount = &bc
			}
		}

		switch item.HealthStatus {
		case "healthy":
			healthy++
		case "warning":
			warning++
		case "error":
			errorN++
		default:
			unknown++
		}

		resp.ClusterSizes = append(resp.ClusterSizes, item)
	}

	resp.Clusters.Healthy = healthy
	resp.Clusters.Warning = warning
	resp.Clusters.Error = errorN
	resp.Clusters.Unknown = unknown

	// 写入缓存
	if data, err := json.Marshal(resp); err == nil {
		_ = s.cache.Set(ctx, overviewCacheKey, data, 0)
	}

	return resp, nil
}
