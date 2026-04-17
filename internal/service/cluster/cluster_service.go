package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/repository"
	"kafka-management-platform/pkg/encryption"
	"kafka-management-platform/pkg/kafka"
)

var (
	// ErrClusterNameExists 集群名称已存在
	ErrClusterNameExists = errors.New("cluster name already exists")
	// ErrConnectionTestFailed 连接测试失败
	ErrConnectionTestFailed = errors.New("connection test failed")
)

// Service 集群管理服务
type Service struct {
	clusterRepo     repository.ClusterRepository
	clusterUserRepo repository.ClusterUserRepository
	encryptionSvc   *encryption.Service
}

// NewService 创建集群管理服务实例
func NewService(
	clusterRepo repository.ClusterRepository,
	clusterUserRepo repository.ClusterUserRepository,
	encryptionSvc *encryption.Service,
) *Service {
	return &Service{
		clusterRepo:     clusterRepo,
		clusterUserRepo: clusterUserRepo,
		encryptionSvc:   encryptionSvc,
	}
}

// CreateClusterRequest 创建集群请求
type CreateClusterRequest struct {
	ClusterName      string                 `json:"cluster_name" binding:"required"`
	BootstrapServers string                 `json:"bootstrap_servers" binding:"required"`
	AuthType         models.AuthType        `json:"auth_type" binding:"required"`
	AuthConfig       map[string]interface{} `json:"auth_config"`
	PrometheusURL    string                 `json:"prometheus_url"`
	Description      string                 `json:"description"`
	CreatedBy        int64                  `json:"-"`
}

// UpdateClusterRequest 更新集群请求
type UpdateClusterRequest struct {
	ClusterName      string                 `json:"cluster_name"`
	BootstrapServers string                 `json:"bootstrap_servers"`
	AuthType         models.AuthType        `json:"auth_type"`
	AuthConfig       map[string]interface{} `json:"auth_config"`
	PrometheusURL    string                 `json:"prometheus_url"`
	Description      string                 `json:"description"`
	Status           models.ClusterStatus   `json:"status"`
}

// CreateCluster 创建集群
// 在创建前会先测试 Kafka 集群连接，只有连接成功才会保存集群配置
func (s *Service) CreateCluster(ctx context.Context, req *CreateClusterRequest) (*models.Cluster, error) {
	// 检查集群名称是否已存在
	existing, err := s.clusterRepo.FindByName(ctx, req.ClusterName)
	if err == nil && existing != nil {
		return nil, ErrClusterNameExists
	}

	// 准备认证配置 JSON
	var authConfigJSON string
	if req.AuthConfig != nil && len(req.AuthConfig) > 0 {
		jsonBytes, err := json.Marshal(req.AuthConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal auth config: %w", err)
		}
		authConfigJSON = string(jsonBytes)
	}

	// 创建临时集群对象用于测试连接
	tempCluster := &models.Cluster{
		BootstrapServers: req.BootstrapServers,
		AuthType:         req.AuthType,
	}

	// 测试 Kafka 集群连接
	if err := s.testKafkaConnection(tempCluster, authConfigJSON); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConnectionTestFailed, err)
	}

	// 连接测试成功，加密认证配置
	var authConfigEncrypted string
	if authConfigJSON != "" {
		authConfigEncrypted, err = s.encryptionSvc.EncryptString(authConfigJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt auth config: %w", err)
		}
	}

	// 创建集群记录
	cluster := &models.Cluster{
		ClusterName:      req.ClusterName,
		BootstrapServers: req.BootstrapServers,
		AuthType:         req.AuthType,
		AuthConfig:       authConfigEncrypted,
		PrometheusURL:    req.PrometheusURL,
		Description:      req.Description,
		Status:           models.ClusterStatusActive,
		CreatedBy:        req.CreatedBy,
	}

	if err := s.clusterRepo.Create(ctx, cluster); err != nil {
		return nil, err
	}

	return cluster, nil
}

// UpdateCluster 更新集群
// 在更新前会先测试 Kafka 集群连接，只有连接成功才会更新集群配置
func (s *Service) UpdateCluster(ctx context.Context, clusterID int64, req *UpdateClusterRequest) error {
	// 获取现有集群
	cluster, err := s.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return err
	}

	// 更新字段
	if req.ClusterName != "" {
		cluster.ClusterName = req.ClusterName
	}
	if req.BootstrapServers != "" {
		cluster.BootstrapServers = req.BootstrapServers
	}
	if req.AuthType != "" {
		cluster.AuthType = req.AuthType
	}
	if req.PrometheusURL != "" {
		cluster.PrometheusURL = req.PrometheusURL
	}
	if req.Description != "" {
		cluster.Description = req.Description
	}
	if req.Status != "" {
		cluster.Status = req.Status
	}

	// 准备认证配置用于测试连接
	var authConfigJSON string
	if req.AuthConfig != nil && len(req.AuthConfig) > 0 {
		authConfigBytes, err := json.Marshal(req.AuthConfig)
		if err != nil {
			return fmt.Errorf("failed to marshal auth config: %w", err)
		}
		authConfigJSON = string(authConfigBytes)
	} else if cluster.AuthConfig != "" {
		// 使用现有的认证配置
		decrypted, err := s.encryptionSvc.DecryptString(cluster.AuthConfig)
		if err != nil {
			return fmt.Errorf("failed to decrypt auth config: %w", err)
		}
		authConfigJSON = decrypted
	}

	// 测试 Kafka 集群连接
	if err := s.testKafkaConnection(cluster, authConfigJSON); err != nil {
		return fmt.Errorf("%w: %v", ErrConnectionTestFailed, err)
	}

	// 连接测试成功，加密认证配置
	if req.AuthConfig != nil && len(req.AuthConfig) > 0 {
		authConfigEncrypted, err := s.encryptionSvc.EncryptString(authConfigJSON)
		if err != nil {
			return fmt.Errorf("failed to encrypt auth config: %w", err)
		}
		cluster.AuthConfig = authConfigEncrypted
	}

	return s.clusterRepo.Update(ctx, cluster)
}

// DeleteCluster 删除集群
func (s *Service) DeleteCluster(ctx context.Context, clusterID int64) error {
	return s.clusterRepo.Delete(ctx, clusterID)
}

// GetCluster 获取集群详情
func (s *Service) GetCluster(ctx context.Context, clusterID int64) (*models.Cluster, error) {
	cluster, err := s.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	// 解密认证配置（用于管理员查看）
	if cluster.AuthConfig != "" {
		decrypted, err := s.encryptionSvc.DecryptString(cluster.AuthConfig)
		if err != nil {
			// 解密失败，返回空配置
			cluster.AuthConfig = ""
		} else {
			cluster.AuthConfig = decrypted
		}
	}

	return cluster, nil
}

// ListClusters 获取集群列表
func (s *Service) ListClusters(ctx context.Context, userID int64, role models.UserRole, offset, limit int) ([]*models.Cluster, int64, error) {
	return s.clusterRepo.ListByUser(ctx, userID, role, offset, limit)
}

// GrantClusterAccess 授予用户集群访问权限
func (s *Service) GrantClusterAccess(ctx context.Context, clusterID, userID int64) error {
	relation := &models.ClusterUserRelation{
		ClusterID: clusterID,
		UserID:    userID,
	}
	return s.clusterUserRepo.Create(ctx, relation)
}

// RevokeClusterAccess 撤销用户集群访问权限
func (s *Service) RevokeClusterAccess(ctx context.Context, clusterID, userID int64) error {
	return s.clusterUserRepo.Delete(ctx, clusterID, userID)
}

// ListClusterUsers 获取集群的授权用户列表
func (s *Service) ListClusterUsers(ctx context.Context, clusterID int64) ([]*models.User, error) {
	return s.clusterUserRepo.ListUsersByCluster(ctx, clusterID)
}

// TestConnection 测试集群连接
func (s *Service) TestConnection(ctx context.Context, clusterID int64) error {
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
	
	return s.testKafkaConnection(cluster, authConfigJSON)
}

// TestConnectionForCreate 在创建集群前测试连接配置
// 用于前端在提交创建请求前验证连接
func (s *Service) TestConnectionForCreate(ctx context.Context, req *CreateClusterRequest) error {
	// 调试日志：打印请求参数
	fmt.Printf("[DEBUG] TestConnectionForCreate - ClusterName: %s\n", req.ClusterName)
	fmt.Printf("[DEBUG] TestConnectionForCreate - BootstrapServers: %s\n", req.BootstrapServers)
	fmt.Printf("[DEBUG] TestConnectionForCreate - AuthType: %s\n", req.AuthType)
	fmt.Printf("[DEBUG] TestConnectionForCreate - AuthConfig: %+v\n", req.AuthConfig)
	
	// 准备认证配置 JSON
	var authConfigJSON string
	if req.AuthConfig != nil && len(req.AuthConfig) > 0 {
		jsonBytes, err := json.Marshal(req.AuthConfig)
		if err != nil {
			return fmt.Errorf("failed to marshal auth config: %w", err)
		}
		authConfigJSON = string(jsonBytes)
		fmt.Printf("[DEBUG] TestConnectionForCreate - AuthConfigJSON: %s\n", authConfigJSON)
	}

	// 创建临时集群对象用于测试连接
	tempCluster := &models.Cluster{
		BootstrapServers: req.BootstrapServers,
		AuthType:         req.AuthType,
	}

	return s.testKafkaConnection(tempCluster, authConfigJSON)
}

// testKafkaConnection 测试 Kafka 集群连接的内部方法
func (s *Service) testKafkaConnection(cluster *models.Cluster, authConfigJSON string) error {
	// 调试日志：打印连接参数
	fmt.Printf("[DEBUG] testKafkaConnection - BootstrapServers: %s\n", cluster.BootstrapServers)
	fmt.Printf("[DEBUG] testKafkaConnection - AuthType: %s\n", cluster.AuthType)
	fmt.Printf("[DEBUG] testKafkaConnection - AuthConfigJSON: %s\n", authConfigJSON)
	
	// 创建 Kafka Admin 客户端
	adminClient, err := kafka.NewAdminClient(cluster, authConfigJSON)
	if err != nil {
		fmt.Printf("[DEBUG] Failed to create kafka admin client: %v\n", err)
		return fmt.Errorf("failed to create kafka admin client: %w", err)
	}
	defer adminClient.Close()
	
	// 测试连接
	if err := adminClient.TestConnection(); err != nil {
		fmt.Printf("[DEBUG] Connection test failed: %v\n", err)
		return fmt.Errorf("connection test failed: %w", err)
	}
	
	fmt.Printf("[DEBUG] Connection test successful\n")
	return nil
}
