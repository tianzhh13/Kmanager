package acl

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"kafka-management-platform/internal/logger"
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
	aclRepo         repository.ACLRepository
	clusterRepo     repository.ClusterRepository
	encryptionSvc   *encryption.Service
	kerberosBaseDir string
}

// NewService 创建 ACL 管理服务实例
func NewService(
	aclRepo repository.ACLRepository,
	clusterRepo repository.ClusterRepository,
	encryptionSvc *encryption.Service,
	kerberosBaseDir string,
) *Service {
	return &Service{
		aclRepo:         aclRepo,
		clusterRepo:     clusterRepo,
		encryptionSvc:   encryptionSvc,
		kerberosBaseDir: kerberosBaseDir,
	}
}

// CreateACLRequest 创建 ACL 请求
type CreateACLRequest struct {
	ClusterID       int64                 `json:"cluster_id" binding:"required"`
	ResourceType    models.ResourceType   `json:"resource_type" binding:"required"`
	ResourceName    string                `json:"resource_name" binding:"required"`
	ResourcePattern models.PatternType    `json:"resource_pattern"`
	Principal       string                `json:"principal" binding:"required"`
	Host            string                `json:"host"`
	Operation       models.OperationType  `json:"operation" binding:"required"`
	PermissionType  models.PermissionType `json:"permission_type" binding:"required"`
	// 兼容前端字段
	Permission string `json:"permission"`
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
	Data  []*models.ACL `json:"data"`
	Total int64         `json:"total"`
}

// CreateACL 创建 ACL 规则
func (s *Service) CreateACL(ctx context.Context, req *CreateACLRequest) error {
	logger.Info("CreateACL: starting", "request", fmt.Sprintf("%+v", req))

	// 兼容前端 permission 字段
	if req.PermissionType == "" && req.Permission != "" {
		req.PermissionType = models.PermissionType(req.Permission)
	}

	// 设置默认值
	if req.ResourcePattern == "" {
		req.ResourcePattern = models.PatternTypeLiteral
	}
	if req.Host == "" {
		req.Host = "*"
	}

	// 验证请求参数
	if err := s.validateCreateACLRequest(req); err != nil {
		return err
	}

	// 获取集群配置
	cluster, err := s.clusterRepo.FindByID(ctx, req.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}
	logger.Info("CreateACL: cluster found", "name", cluster.ClusterName)

	// 解密认证配置
	var authConfigJSON string
	if cluster.AuthConfig != "" {
		decrypted, err := s.encryptionSvc.DecryptString(cluster.AuthConfig)
		if err != nil {
			return fmt.Errorf("failed to decrypt auth config: %w", err)
		}
		authConfigJSON = decrypted
	}

	// 创建 Kafka Admin 客户端（支持 Kerberos）
	adminClient, err := kafka.NewAdminClientWithKerberos(cluster, authConfigJSON, s.kerberosBaseDir)
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

	acl := sarama.Acl{
		Principal:      req.Principal,
		Host:           req.Host,
		Operation:      s.convertOperation(req.Operation),
		PermissionType: s.convertPermissionType(req.PermissionType),
	}

	logger.Info("CreateACL: creating in kafka", "resource", fmt.Sprintf("%+v", resource), "acl", fmt.Sprintf("%+v", acl))

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
		Host:            req.Host,
		Operation:       req.Operation,
		PermissionType:  req.PermissionType,
		SyncStatus:      "synced",
	}

	if err := s.aclRepo.Create(ctx, aclModel); err != nil {
		return fmt.Errorf("failed to save acl to database: %w", err)
	}

	logger.Info("CreateACL: created successfully")
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

	// 创建 Kafka Admin 客户端（支持 Kerberos）
	adminClient, err := kafka.NewAdminClientWithKerberos(cluster, authConfigJSON, s.kerberosBaseDir)
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
	var errs []error
	for _, aclID := range aclIDs {
		if err := s.DeleteACL(ctx, aclID); err != nil {
			errMsg := fmt.Sprintf("failed to delete ACL id=%d: %v", aclID, err)
			logger.Warn("BatchDeleteACL: partial failure", "message", errMsg)
			errs = append(errs, errors.New(errMsg))
			continue
		}
		logger.Info("BatchDeleteACL: deleted", "acl_id", aclID)
	}
	return errors.Join(errs...)
}

// ListACLs 列出 ACL 规则
func (s *Service) ListACLs(ctx context.Context, req *ListACLsRequest) (*ListACLsResponse, error) {
	logger.Info("ListACLs: request", "cluster_id", req.ClusterID, "resource_type", req.ResourceType, "resource_name", req.ResourceName, "principal", req.Principal)

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
		logger.Error("ListACLs: error", "error", err)
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

	logger.Info("ListACLs: found", "count", len(acls))
	return &ListACLsResponse{
		Data:  acls,
		Total: total,
	}, nil
}

// UserACLInfo 用户 ACL 信息（用于从 Kafka 直接查询）
type UserACLInfo struct {
	ResourceType    string `json:"resource_type"`
	ResourceName    string `json:"resource_name"`
	ResourcePattern string `json:"resource_pattern"`
	Principal       string `json:"principal"`
	Host            string `json:"host"`
	Operation       string `json:"operation"`
	PermissionType  string `json:"permission_type"`
}

// ListUserACLsFromKafka 从 Kafka 直接查询用户的 ACL（实时查询）
func (s *Service) ListUserACLsFromKafka(ctx context.Context, clusterID int64, principal string) ([]*UserACLInfo, error) {
	logger.Info("ListUserACLsFromKafka: querying", "principal", principal)

	// 获取集群配置
	cluster, err := s.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	// 解密认证配置
	var authConfigJSON string
	if cluster.AuthConfig != "" {
		decrypted, err := s.encryptionSvc.DecryptString(cluster.AuthConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt auth config: %w", err)
		}
		authConfigJSON = decrypted
	}

	// 创建 Kafka Admin 客户端（支持 Kerberos）
	adminClient, err := kafka.NewAdminClientWithKerberos(cluster, authConfigJSON, s.kerberosBaseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka admin client: %w", err)
	}
	defer adminClient.Close()

	// 从 Kafka 获取该用户的所有 ACL
	filter := sarama.AclFilter{
		ResourceType:              sarama.AclResourceAny,
		ResourcePatternTypeFilter: sarama.AclPatternAny,
		Principal:                 &principal,
		Host:                      nil,
		Operation:                 sarama.AclOperationAny,
		PermissionType:            sarama.AclPermissionAny,
	}

	kafkaACLs, err := adminClient.ListACLs(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list acls from kafka: %w", err)
	}

	// 转换结果
	var result []*UserACLInfo
	for _, resourceACL := range kafkaACLs {
		for _, acl := range resourceACL.Acls {
			info := &UserACLInfo{
				ResourceType:    string(s.convertResourceTypeFromSarama(resourceACL.ResourceType)),
				ResourceName:    resourceACL.ResourceName,
				ResourcePattern: string(s.convertPatternTypeFromSarama(resourceACL.ResourcePatternType)),
				Principal:       acl.Principal,
				Host:            acl.Host,
				Operation:       string(s.convertOperationFromSarama(acl.Operation)),
				PermissionType:  string(s.convertPermissionTypeFromSarama(acl.PermissionType)),
			}
			result = append(result, info)
		}
	}

	logger.Info("ListUserACLsFromKafka: found", "count", len(result), "principal", principal)
	return result, nil
}

// DeleteACLFromKafka 从 Kafka 删除指定用户的 ACL（按条件匹配）
func (s *Service) DeleteACLFromKafka(ctx context.Context, clusterID int64, req *DeleteACLFromKafkaRequest) error {
	logger.Info("DeleteACLFromKafka: deleting", "cluster_id", clusterID, "principal", req.Principal, "resource", req.ResourceName, "operation", req.Operation)

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

	// 创建 Kafka Admin 客户端（支持 Kerberos）
	adminClient, err := kafka.NewAdminClientWithKerberos(cluster, authConfigJSON, s.kerberosBaseDir)
	if err != nil {
		return fmt.Errorf("failed to create kafka admin client: %w", err)
	}
	defer adminClient.Close()

	// 构建 ACL Filter
	filter := sarama.AclFilter{
		ResourceType:              s.convertResourceType(models.ResourceType(req.ResourceType)),
		ResourceName:              &req.ResourceName,
		ResourcePatternTypeFilter: s.convertPatternType(models.PatternType(req.ResourcePattern)),
		Principal:                 &req.Principal,
		Host:                      &req.Host,
		Operation:                 s.convertOperation(models.OperationType(req.Operation)),
		PermissionType:            s.convertPermissionType(models.PermissionType(req.PermissionType)),
	}

	// 从 Kafka 删除 ACL
	matchedACLs, err := adminClient.DeleteACL(filter, false)
	if err != nil {
		return fmt.Errorf("failed to delete acl from kafka: %w", err)
	}

	logger.Info("DeleteACLFromKafka: deleted", "count", len(matchedACLs))
	return nil
}

// DeleteACLFromKafkaRequest 从 Kafka 删除 ACL 的请求
type DeleteACLFromKafkaRequest struct {
	ResourceType    string `json:"resource_type" binding:"required"`
	ResourceName    string `json:"resource_name" binding:"required"`
	ResourcePattern string `json:"resource_pattern"`
	Principal       string `json:"principal" binding:"required"`
	Host            string `json:"host"`
	Operation       string `json:"operation" binding:"required"`
	PermissionType  string `json:"permission_type" binding:"required"`
}

// SyncACLs 同步集群的所有 ACL 数据
func (s *Service) SyncACLs(ctx context.Context, clusterID int64) error {
	logger.Info("SyncACLs: starting sync", "cluster_id", clusterID)

	// 获取集群配置
	cluster, err := s.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}
	logger.Info("SyncACLs: cluster found", "name", cluster.ClusterName, "bootstrap", cluster.BootstrapServers)

	// 解密认证配置
	var authConfigJSON string
	if cluster.AuthConfig != "" {
		decrypted, err := s.encryptionSvc.DecryptString(cluster.AuthConfig)
		if err != nil {
			return fmt.Errorf("failed to decrypt auth config: %w", err)
		}
		authConfigJSON = decrypted
	}

	// 创建 Kafka Admin 客户端（支持 Kerberos）
	adminClient, err := kafka.NewAdminClientWithKerberos(cluster, authConfigJSON, s.kerberosBaseDir)
	if err != nil {
		return fmt.Errorf("failed to create kafka admin client: %w", err)
	}
	defer adminClient.Close()
	logger.Info("SyncACLs: kafka admin client created")

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
	logger.Info("SyncACLs: found resource ACLs from kafka", "count", len(kafkaACLs))

	// 从数据库获取当前 ACL 列表
	dbACLs, err := s.aclRepo.ListByCluster(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("failed to list acls from database: %w", err)
	}
	logger.Info("SyncACLs: found ACLs in db", "count", len(dbACLs))

	// 构建 ACL 映射
	dbACLMap := make(map[string]*models.ACL)
	for _, acl := range dbACLs {
		key := fmt.Sprintf("%s-%s-%s-%s-%s-%s",
			acl.ResourceType, acl.ResourceName, acl.Principal, acl.Host, acl.Operation, acl.PermissionType)
		dbACLMap[key] = acl
	}

	// 处理新增的 ACL
	newCount := 0
	for _, resourceACL := range kafkaACLs {
		logger.Info("SyncACLs: processing resource", "type", resourceACL.ResourceType, "name", resourceACL.ResourceName, "pattern", resourceACL.ResourcePatternType)
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
					logger.Error("SyncACLs: failed to create ACL", "error", err)
					continue
				}
				newCount++
			}
		}
	}

	logger.Info("SyncACLs: completed", "cluster_id", clusterID, "new", newCount)
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
	switch strings.ToLower(string(rt)) {
	case "topic":
		return sarama.AclResourceTopic
	case "group":
		return sarama.AclResourceGroup
	case "cluster":
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
	switch strings.ToLower(string(pt)) {
	case "literal":
		return sarama.AclPatternLiteral
	case "prefixed":
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
	switch strings.ToLower(string(op)) {
	case "read":
		return sarama.AclOperationRead
	case "write":
		return sarama.AclOperationWrite
	case "create":
		return sarama.AclOperationCreate
	case "delete":
		return sarama.AclOperationDelete
	case "alter":
		return sarama.AclOperationAlter
	case "describe":
		return sarama.AclOperationDescribe
	case "all":
		return sarama.AclOperationAll
	case "describeconfigs":
		return sarama.AclOperationDescribeConfigs
	case "alterconfigs":
		return sarama.AclOperationAlterConfigs
	case "clusteraction":
		return sarama.AclOperationClusterAction
	case "idempotentwrite":
		return sarama.AclOperationIdempotentWrite
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
	case sarama.AclOperationDescribeConfigs:
		return models.OperationDescribeConfigs
	case sarama.AclOperationAlterConfigs:
		return models.OperationAlterConfigs
	case sarama.AclOperationClusterAction:
		return models.OperationClusterAction
	case sarama.AclOperationIdempotentWrite:
		return models.OperationIdempotentWrite
	default:
		return models.OperationRead
	}
}

func (s *Service) convertPermissionType(pt models.PermissionType) sarama.AclPermissionType {
	switch strings.ToLower(string(pt)) {
	case "allow":
		return sarama.AclPermissionAllow
	case "deny":
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
