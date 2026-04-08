package acl

import (
	"context"
	"errors"
	"fmt"

	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/repository"
	"kafka-management-platform/pkg/encryption"
	"kafka-management-platform/pkg/kafka"

	"github.com/IBM/sarama"
)

var (
	// ErrACLNotFound ACL 不存在
	ErrACLNotFound = errors.New("acl not found")
	// ErrInvalidACLParams 无效的 ACL 参数
	ErrInvalidACLParams = errors.New("invalid acl parameters")
)

// Service ACL 管理服务
type Service struct {
	aclRepo       repository.ACLRepository
	clusterRepo   repository.ClusterRepository
	encryptionSvc *encryption.Service
}

// NewService 创建 ACL 管理服务实例
func NewService(
	aclRepo repository.ACLRepository,
	clusterRepo repository.ClusterRepository,
	encryptionSvc *encryption.Service,
) *Service {
	return &Service{
		aclRepo:       aclRepo,
		clusterRepo:   clusterRepo,
		encryptionSvc: encryptionSvc,
	}
}

// CreateACLRequest 创建 ACL 请求
type CreateACLRequest struct {
	ClusterID       int64                 `json:"cluster_id" binding:"required"`
	ResourceType    models.ResourceType   `json:"resource_type" binding:"required"`
	ResourceName    string                `json:"resource_name" binding:"required"`
	ResourcePattern models.PatternType    `json:"resource_pattern" binding:"required"`
	Principal       string                `json:"principal" binding:"required"`
	Host            string                `json:"host"`
	Operation       models.OperationType  `json:"operation" binding:"required"`
	PermissionType  models.PermissionType `json:"permission_type" binding:"required"`
}

// ListACLsRequest 列出 ACL 请求
type ListACLsRequest struct {
	ClusterID    int64  `json:"cluster_id"`
	ResourceType string `json:"resource_type"`
	ResourceName string `json:"resource_name"`
	Principal    string `json:"principal"`
	Offset       int    `json:"offset"`
	Limit        int    `json:"limit"`
}

// ListACLsResponse 列出 ACL 响应
type ListACLsResponse struct {
	ACLs  []*models.ACL `json:"acls"`
	Total int64         `json:"total"`
}

// CreateACL 创建 ACL 规则
func (s *Service) CreateACL(ctx context.Context, req *CreateACLRequest) error {
	// 验证请求参数
	if err := s.validateCreateACLRequest(req); err != nil {
		return err
	}

	// 获取集群配置
	cluster, err := s.clusterRepo.FindByID(ctx, req.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}

	// 解密认证配置
	var authConfigJSON string
	if cluster.AuthConfig != "" {
		decrypted, err := s.encryptionSvc.DecryptString(cluster.AuthConfig)
		if err != nil {
			return fmt.Errorf("failed to decrypt auth config: %w", err)
		}
		authConfigJSON = decrypted
	}

	// 创建 Kafka Admin 客户端
	adminClient, err := kafka.NewAdminClient(cluster, authConfigJSON)
	if err != nil {
		return fmt.Errorf("failed to create kafka admin client: %w", err)
	}
	defer adminClient.Close()

	// 转换为 Sarama ACL 格式
	resource := sarama.Resource{
		ResourceType:        s.convertResourceType(req.ResourceType),
		ResourceName:        req.ResourceName,
		ResourcePatternType: s.convertPatternType(req.ResourcePattern),
	}

	host := req.Host
	if host == "" {
		host = "*"
	}

	acl := sarama.Acl{
		Principal:      req.Principal,
		Host:           host,
		Operation:      s.convertOperation(req.Operation),
		PermissionType: s.convertPermissionType(req.PermissionType),
	}

	// 调用 Kafka API 创建 ACL
	if err := adminClient.CreateACL(resource, acl); err != nil {
		return fmt.Errorf("failed to create acl in kafka: %w", err)
	}

	// 保存 ACL 到数据库
	aclModel := &models.ACL{
		ClusterID:       req.ClusterID,
		ResourceType:    req.ResourceType,
		ResourceName:    req.ResourceName,
		ResourcePattern: req.ResourcePattern,
		Principal:       req.Principal,
		Host:            host,
		Operation:       req.Operation,
		PermissionType:  req.PermissionType,
		SyncStatus:      "synced",
	}

	if err := s.aclRepo.Create(ctx, aclModel); err != nil {
		return fmt.Errorf("failed to save acl to database: %w", err)
	}

	return nil
}

// DeleteACL 删除 ACL 规则
func (s *Service) DeleteACL(ctx context.Context, aclID int64) error {
	// 获取 ACL 信息
	aclModel, err := s.aclRepo.FindByID(ctx, aclID)
	if err != nil {
		return err
	}

	// 获取集群配置
	cluster, err := s.clusterRepo.FindByID(ctx, aclModel.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}

	// 解密认证配置
	var authConfigJSON string
	if cluster.AuthConfig != "" {
		decrypted, err := s.encryptionSvc.DecryptString(cluster.AuthConfig)
		if err != nil {
			return fmt.Errorf("failed to decrypt auth config: %w", err)
		}
		authConfigJSON = decrypted
	}

	// 创建 Kafka Admin 客户端
	adminClient, err := kafka.NewAdminClient(cluster, authConfigJSON)
	if err != nil {
		return fmt.Errorf("failed to create kafka admin client: %w", err)
	}
	defer adminClient.Close()

	// 转换为 Sarama ACL Filter 格式
	filter := sarama.AclFilter{
		ResourceType:              s.convertResourceType(aclModel.ResourceType),
		ResourceName:              &aclModel.ResourceName,
		ResourcePatternTypeFilter: s.convertPatternType(aclModel.ResourcePattern),
		Principal:                 &aclModel.Principal,
		Host:                      &aclModel.Host,
		Operation:                 s.convertOperation(aclModel.Operation),
		PermissionType:            s.convertPermissionType(aclModel.PermissionType),
	}

	// 从 Kafka 删除 ACL
	if _, err := adminClient.DeleteACL(filter, false); err != nil {
		return fmt.Errorf("failed to delete acl from kafka: %w", err)
	}

	// 从数据库删除 ACL
	if err := s.aclRepo.Delete(ctx, aclID); err != nil {
		return fmt.Errorf("failed to delete acl from database: %w", err)
	}

	return nil
}

// BatchDeleteACL 批量删除 ACL 规则
func (s *Service) BatchDeleteACL(ctx context.Context, aclIDs []int64) error {
	for _, aclID := range aclIDs {
		if err := s.DeleteACL(ctx, aclID); err != nil {
			// 记录错误但继续处理其他 ACL
			continue
		}
	}
	return nil
}

// ListACLs 列出 ACL 规则
func (s *Service) ListACLs(ctx context.Context, req *ListACLsRequest) (*ListACLsResponse, error) {
	var acls []*models.ACL
	var total int64
	var err error

	// 根据过滤条件选择不同的查询方法
	if req.ResourceName != "" {
		acls, total, err = s.aclRepo.FilterByTopic(ctx, req.ClusterID, req.ResourceName, req.Offset, req.Limit)
	} else if req.Principal != "" {
		acls, total, err = s.aclRepo.FilterByPrincipal(ctx, req.ClusterID, req.Principal, req.Offset, req.Limit)
	} else {
		acls, total, err = s.aclRepo.List(ctx, req.ClusterID, req.Offset, req.Limit)
	}

	if err != nil {
		return nil, err
	}

	// 如果有 resourceType 过滤，在内存中过滤
	if req.ResourceType != "" {
		filtered := make([]*models.ACL, 0)
		for _, acl := range acls {
			if string(acl.ResourceType) == req.ResourceType {
				filtered = append(filtered, acl)
			}
		}
		acls = filtered
		total = int64(len(filtered))
	}

	return &ListACLsResponse{
		ACLs:  acls,
		Total: total,
	}, nil
}

// SyncACLs 同步集群的所有 ACL 数据
func (s *Service) SyncACLs(ctx context.Context, clusterID int64) error {
	// 获取集群配置
	cluster, err := s.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}

	// 解密认证配置
	var authConfigJSON string
	if cluster.AuthConfig != "" {
		decrypted, err := s.encryptionSvc.DecryptString(cluster.AuthConfig)
		if err != nil {
			return fmt.Errorf("failed to decrypt auth config: %w", err)
		}
		authConfigJSON = decrypted
	}

	// 创建 Kafka Admin 客户端
	adminClient, err := kafka.NewAdminClient(cluster, authConfigJSON)
	if err != nil {
		return fmt.Errorf("failed to create kafka admin client: %w", err)
	}
	defer adminClient.Close()

	// 从 Kafka 获取所有 ACL 列表
	filter := sarama.AclFilter{
		ResourceType:              sarama.AclResourceAny,
		ResourcePatternTypeFilter: sarama.AclPatternAny,
		Operation:                 sarama.AclOperationAny,
		PermissionType:            sarama.AclPermissionAny,
	}

	kafkaACLs, err := adminClient.ListACLs(filter)
	if err != nil {
		return fmt.Errorf("failed to list acls from kafka: %w", err)
	}

	// 从数据库获取当前 ACL 列表
	dbACLs, err := s.aclRepo.ListByCluster(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("failed to list acls from database: %w", err)
	}

	// 构建 ACL 映射
	dbACLMap := make(map[string]*models.ACL)
	for _, acl := range dbACLs {
		key := fmt.Sprintf("%s-%s-%s-%s-%s-%s",
			acl.ResourceType, acl.ResourceName, acl.Principal, acl.Host, acl.Operation, acl.PermissionType)
		dbACLMap[key] = acl
	}

	// 处理新增的 ACL
	for _, resourceACL := range kafkaACLs {
		for _, acl := range resourceACL.Acls {
			key := fmt.Sprintf("%s-%s-%s-%s-%s-%s",
				s.convertResourceTypeFromSarama(resourceACL.ResourceType),
				resourceACL.ResourceName,
				acl.Principal,
				acl.Host,
				s.convertOperationFromSarama(acl.Operation),
				s.convertPermissionTypeFromSarama(acl.PermissionType))

			if _, exists := dbACLMap[key]; !exists {
				// 新 ACL，插入数据库
				newACL := &models.ACL{
					ClusterID:       clusterID,
					ResourceType:    s.convertResourceTypeFromSarama(resourceACL.ResourceType),
					ResourceName:    resourceACL.ResourceName,
					ResourcePattern: s.convertPatternTypeFromSarama(resourceACL.ResourcePatternType),
					Principal:       acl.Principal,
					Host:            acl.Host,
					Operation:       s.convertOperationFromSarama(acl.Operation),
					PermissionType:  s.convertPermissionTypeFromSarama(acl.PermissionType),
					SyncStatus:      "synced",
				}
				if err := s.aclRepo.Create(ctx, newACL); err != nil {
					// 记录错误但继续处理其他 ACL
					continue
				}
			}
		}
	}

	return nil
}

// validateCreateACLRequest 验证创建 ACL 请求
func (s *Service) validateCreateACLRequest(req *CreateACLRequest) error {
	if req.ClusterID <= 0 {
		return fmt.Errorf("invalid cluster_id")
	}
	if req.ResourceName == "" {
		return fmt.Errorf("resource_name is required")
	}
	if req.Principal == "" {
		return fmt.Errorf("principal is required")
	}
	return nil
}

// 类型转换辅助函数
func (s *Service) convertResourceType(rt models.ResourceType) sarama.AclResourceType {
	switch rt {
	case models.ResourceTypeTopic:
		return sarama.AclResourceTopic
	case models.ResourceTypeGroup:
		return sarama.AclResourceGroup
	case models.ResourceTypeCluster:
		return sarama.AclResourceCluster
	default:
		return sarama.AclResourceAny
	}
}

func (s *Service) convertResourceTypeFromSarama(rt sarama.AclResourceType) models.ResourceType {
	switch rt {
	case sarama.AclResourceTopic:
		return models.ResourceTypeTopic
	case sarama.AclResourceGroup:
		return models.ResourceTypeGroup
	case sarama.AclResourceCluster:
		return models.ResourceTypeCluster
	default:
		return models.ResourceTypeTopic
	}
}

func (s *Service) convertPatternType(pt models.PatternType) sarama.AclResourcePatternType {
	switch pt {
	case models.PatternTypeLiteral:
		return sarama.AclPatternLiteral
	case models.PatternTypePrefixed:
		return sarama.AclPatternPrefixed
	default:
		return sarama.AclPatternLiteral
	}
}

func (s *Service) convertPatternTypeFromSarama(pt sarama.AclResourcePatternType) models.PatternType {
	switch pt {
	case sarama.AclPatternLiteral:
		return models.PatternTypeLiteral
	case sarama.AclPatternPrefixed:
		return models.PatternTypePrefixed
	default:
		return models.PatternTypeLiteral
	}
}

func (s *Service) convertOperation(op models.OperationType) sarama.AclOperation {
	switch op {
	case models.OperationRead:
		return sarama.AclOperationRead
	case models.OperationWrite:
		return sarama.AclOperationWrite
	case models.OperationCreate:
		return sarama.AclOperationCreate
	case models.OperationDelete:
		return sarama.AclOperationDelete
	case models.OperationAlter:
		return sarama.AclOperationAlter
	case models.OperationDescribe:
		return sarama.AclOperationDescribe
	case models.OperationAll:
		return sarama.AclOperationAll
	default:
		return sarama.AclOperationAny
	}
}

func (s *Service) convertOperationFromSarama(op sarama.AclOperation) models.OperationType {
	switch op {
	case sarama.AclOperationRead:
		return models.OperationRead
	case sarama.AclOperationWrite:
		return models.OperationWrite
	case sarama.AclOperationCreate:
		return models.OperationCreate
	case sarama.AclOperationDelete:
		return models.OperationDelete
	case sarama.AclOperationAlter:
		return models.OperationAlter
	case sarama.AclOperationDescribe:
		return models.OperationDescribe
	case sarama.AclOperationAll:
		return models.OperationAll
	default:
		return models.OperationRead
	}
}

func (s *Service) convertPermissionType(pt models.PermissionType) sarama.AclPermissionType {
	switch pt {
	case models.PermissionTypeAllow:
		return sarama.AclPermissionAllow
	case models.PermissionTypeDeny:
		return sarama.AclPermissionDeny
	default:
		return sarama.AclPermissionAny
	}
}

func (s *Service) convertPermissionTypeFromSarama(pt sarama.AclPermissionType) models.PermissionType {
	switch pt {
	case sarama.AclPermissionAllow:
		return models.PermissionTypeAllow
	case sarama.AclPermissionDeny:
		return models.PermissionTypeDeny
	default:
		return models.PermissionTypeAllow
	}
}
