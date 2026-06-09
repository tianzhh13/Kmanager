package scram

import (
	"context"
	"errors"
	"fmt"
	"time"

	"kafka-management-platform/internal/logger"
	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/repository"
	"kafka-management-platform/pkg/encryption"
	"kafka-management-platform/pkg/kafka"
)

var (
	// ErrUserAlreadyExists 用户已存在
	ErrUserAlreadyExists = errors.New("user already exists")
	// ErrUserNotFound 用户不存在
	ErrUserNotFound = errors.New("user not found")
)

// Service SCRAM 用户管理服务
type Service struct {
	userRepo        repository.ScramUserRepository
	clusterRepo     repository.ClusterRepository
	encryptSvc      *encryption.Service
	kerberosBaseDir string
}

// NewService 创建 SCRAM 用户管理服务实例
func NewService(
	userRepo repository.ScramUserRepository,
	clusterRepo repository.ClusterRepository,
	encryptSvc *encryption.Service,
	kerberosBaseDir string,
) *Service {
	return &Service{
		userRepo:        userRepo,
		clusterRepo:     clusterRepo,
		encryptSvc:      encryptSvc,
		kerberosBaseDir: kerberosBaseDir,
	}
}

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	ClusterID int64  `json:"cluster_id" binding:"required"`
	Username  string `json:"username" binding:"required"`
	Password  string `json:"password" binding:"required"`
	Mechanism string `json:"mechanism"` // SCRAM-SHA-256 或 SCRAM-SHA-512
}

// ListUsersRequest 列出用户请求
type ListUsersRequest struct {
	ClusterID int64 `json:"cluster_id"`
	Offset    int   `json:"offset"`
	Limit     int   `json:"limit"`
}

// ListUsersResponse 列出用户响应
type ListUsersResponse struct {
	Data  []*models.ScramUser `json:"data"`
	Total int64               `json:"total"`
}

// CreateUser 创建 SCRAM 用户
func (s *Service) CreateUser(ctx context.Context, req *CreateUserRequest) error {
	logger.Info("CreateUser: creating SCRAM user", "cluster_id", req.ClusterID, "username", req.Username, "mechanism", req.Mechanism)

	// 检查用户是否已存在
	exists, err := s.userRepo.Exists(ctx, req.ClusterID, req.Username)
	if err != nil {
		return fmt.Errorf("failed to check user existence: %w", err)
	}
	if exists {
		return ErrUserAlreadyExists
	}

	// 获取集群配置
	cluster, err := s.clusterRepo.FindByID(ctx, req.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}

	// 解密认证配置
	var authConfigJSON string
	if cluster.AuthConfig != "" {
		decrypted, err := s.encryptSvc.DecryptString(cluster.AuthConfig)
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

	// 设置默认机制
	mechanism := req.Mechanism
	if mechanism == "" {
		mechanism = "SCRAM-SHA-256"
	}

	// 在 Kafka 中创建用户
	if err := adminClient.CreateUser(req.Username, req.Password, mechanism); err != nil {
		return fmt.Errorf("failed to create user in kafka: %w", err)
	}

	// 保存用户到数据库
	now := time.Now()
	user := &models.ScramUser{
		ClusterID:  req.ClusterID,
		Username:   req.Username,
		Mechanism:  mechanism,
		SyncStatus: "synced",
		LastSyncAt: &now,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return fmt.Errorf("failed to save user to database: %w", err)
	}

	logger.Info("CreateUser: user created successfully", "username", req.Username)
	return nil
}

// DeleteUser 删除 SCRAM 用户
func (s *Service) DeleteUser(ctx context.Context, clusterID int64, username string) error {
	logger.Info("DeleteUser: deleting SCRAM user", "cluster_id", clusterID, "username", username)

	// 获取用户信息
	user, err := s.userRepo.FindByUsername(ctx, clusterID, username)
	if err != nil {
		return ErrUserNotFound
	}

	// 获取集群配置
	cluster, err := s.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}

	// 解密认证配置
	var authConfigJSON string
	if cluster.AuthConfig != "" {
		decrypted, err := s.encryptSvc.DecryptString(cluster.AuthConfig)
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

	// 从 Kafka 删除用户（使用用户记录中的 mechanism）
	if err := adminClient.DeleteUser(username, user.Mechanism); err != nil {
		return fmt.Errorf("failed to delete user from kafka: %w", err)
	}

	// 从数据库删除用户
	if err := s.userRepo.Delete(ctx, user.UserID); err != nil {
		return fmt.Errorf("failed to delete user from database: %w", err)
	}

	logger.Info("DeleteUser: user deleted successfully", "username", username)
	return nil
}

// ListUsers 列出 SCRAM 用户
func (s *Service) ListUsers(ctx context.Context, req *ListUsersRequest) (*ListUsersResponse, error) {
	logger.Info("ListUsers: listing users", "cluster_id", req.ClusterID, "offset", req.Offset, "limit", req.Limit)

	if req.Limit == 0 {
		req.Limit = 20
	}

	users, total, err := s.userRepo.List(ctx, req.ClusterID, req.Offset, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	logger.Info("ListUsers: found users", "count", len(users))
	return &ListUsersResponse{
		Data:  users,
		Total: total,
	}, nil
}

// SyncUsers 同步 Kafka 用户到数据库
func (s *Service) SyncUsers(ctx context.Context, clusterID int64) error {
	logger.Info("SyncUsers: syncing users", "cluster_id", clusterID)

	// 获取集群配置
	cluster, err := s.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}

	// 解密认证配置
	var authConfigJSON string
	if cluster.AuthConfig != "" {
		decrypted, err := s.encryptSvc.DecryptString(cluster.AuthConfig)
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

	// 从 Kafka 获取用户列表
	kafkaUsers, err := adminClient.ListUsers()
	if err != nil {
		return fmt.Errorf("failed to list users from kafka: %w", err)
	}
	logger.Info("SyncUsers: found users from kafka", "count", len(kafkaUsers))

	// 从数据库获取当前用户列表
	dbUsers, err := s.userRepo.ListByCluster(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("failed to list users from database: %w", err)
	}

	// 构建用户名映射
	dbUserMap := make(map[string]*models.ScramUser)
	for _, user := range dbUsers {
		dbUserMap[user.Username] = user
	}

	// 处理新增的用户
	now := time.Now()
	newCount := 0
	for _, kafkaUser := range kafkaUsers {
		if _, exists := dbUserMap[kafkaUser.User]; !exists {
			// 获取用户的 mechanism 信息
			mechanism := "SCRAM-SHA-256" // 默认值
			if len(kafkaUser.CredentialInfos) > 0 {
				mechanism = kafkaUser.CredentialInfos[0].Mechanism.String()
			}

			// 新用户，插入数据库
			newUser := &models.ScramUser{
				ClusterID:  clusterID,
				Username:   kafkaUser.User,
				Mechanism:  mechanism,
				SyncStatus: "synced",
				LastSyncAt: &now,
			}
			if err := s.userRepo.Create(ctx, newUser); err != nil {
				logger.Error("SyncUsers: failed to create user", "username", kafkaUser.User, "error", err)
				continue
			}
			newCount++
		}
	}

	logger.Info("SyncUsers: completed", "new", newCount)
	return nil
}
