package auth

import (
	"context"

	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/repository"
)

// PermissionService 权限服务
type PermissionService struct {
	userRepo        repository.UserRepository
	clusterUserRepo repository.ClusterUserRepository
	topicPermRepo   repository.TopicPermissionRepository
}

// NewPermissionService 创建权限服务实例
func NewPermissionService(
	userRepo repository.UserRepository,
	clusterUserRepo repository.ClusterUserRepository,
	topicPermRepo repository.TopicPermissionRepository,
) *PermissionService {
	return &PermissionService{
		userRepo:        userRepo,
		clusterUserRepo: clusterUserRepo,
		topicPermRepo:   topicPermRepo,
	}
}

// CheckPermission 检查用户是否有指定操作权限
func (s *PermissionService) CheckPermission(ctx context.Context, userID int64, resource, action string) (bool, error) {
	// 获取用户信息
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return false, err
	}

	// 超级管理员拥有所有权限
	if user.Role == models.RoleSuperAdmin {
		return true, nil
	}

	// 普通用户仅有查询权限
	if user.Role == models.RoleNormalUser {
		return isReadOperation(action), nil
	}

	// 集群管理员需要进一步检查集群权限
	return false, nil
}

// CheckClusterPermission 检查用户是否有指定集群的管理权限
func (s *PermissionService) CheckClusterPermission(ctx context.Context, userID, clusterID int64, role string) (bool, error) {
	// 超级管理员拥有所有集群权限
	if role == string(models.RoleSuperAdmin) {
		return true, nil
	}

	// 普通用户无集群管理权限
	if role == string(models.RoleNormalUser) {
		return false, nil
	}

	// 集群管理员需要检查是否被授权管理该集群
	if role == string(models.RoleClusterAdmin) {
		return s.clusterUserRepo.HasAccess(ctx, clusterID, userID)
	}

	return false, nil
}

// CheckClusterReadPermission 检查用户是否有集群的读权限
func (s *PermissionService) CheckClusterReadPermission(ctx context.Context, userID, clusterID int64, role string) (bool, error) {
	// 超级管理员拥有所有权限
	if role == string(models.RoleSuperAdmin) {
		return true, nil
	}

	// 集群管理员和只读用户都需要检查是否被授权访问该集群
	return s.clusterUserRepo.HasAccess(ctx, clusterID, userID)
}

// IsSuperAdmin 检查用户是否为超级管理员
func (s *PermissionService) IsSuperAdmin(ctx context.Context, userID int64) (bool, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return false, err
	}
	return user.Role == models.RoleSuperAdmin, nil
}

// IsClusterAdmin 检查用户是否为集群管理员
func (s *PermissionService) IsClusterAdmin(ctx context.Context, userID int64) (bool, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return false, err
	}
	return user.Role == models.RoleClusterAdmin, nil
}

// IsReadOnly 检查用户是否为普通用户
func (s *PermissionService) IsReadOnly(ctx context.Context, userID int64) (bool, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return false, err
	}
	return user.Role == models.RoleNormalUser, nil
}

// CheckTopicPermission 检查用户是否有指定 Topic 的权限
func (s *PermissionService) CheckTopicPermission(ctx context.Context, userID, clusterID int64, topicName string) (bool, error) {
	// 获取用户信息
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return false, err
	}

	// 超级管理员拥有所有权限
	if user.Role == models.RoleSuperAdmin {
		return true, nil
	}

	// 集群管理员拥有分配集群的所有 Topic 权限
	if user.Role == models.RoleClusterAdmin {
		return s.clusterUserRepo.HasAccess(ctx, clusterID, userID)
	}

	// 普通用户需要检查 Topic 权限
	if user.Role == models.RoleNormalUser {
		if s.topicPermRepo == nil {
			return false, nil
		}
		return s.topicPermRepo.HasAccess(ctx, userID, clusterID, topicName)
	}

	return false, nil
}

// GetAllowedTopics 获取用户在指定集群有权限的 Topic 列表
func (s *PermissionService) GetAllowedTopics(ctx context.Context, userID, clusterID int64) ([]string, error) {
	// 获取用户信息
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 超级管理员和集群管理员可以访问所有 Topic（返回空列表表示无限制）
	if user.Role == models.RoleSuperAdmin || user.Role == models.RoleClusterAdmin {
		return nil, nil
	}

	// 普通用户返回有权限的 Topic 列表
	if s.topicPermRepo == nil {
		return []string{}, nil
	}
	return s.topicPermRepo.ListTopicsByUserCluster(ctx, userID, clusterID)
}

// GetAllowedClusters 获取用户有权限的集群 ID 列表
func (s *PermissionService) GetAllowedClusters(ctx context.Context, userID int64) ([]int64, error) {
	// 获取用户信息
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 超级管理员可以访问所有集群（返回空列表表示无限制）
	if user.Role == models.RoleSuperAdmin {
		return nil, nil
	}

	// 集群管理员和普通用户返回有权限的集群列表
	clusters, err := s.clusterUserRepo.ListClustersByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	var clusterIDs []int64
	for _, cluster := range clusters {
		clusterIDs = append(clusterIDs, cluster.ClusterID)
	}
	return clusterIDs, nil
}

// isReadOperation 判断是否为只读操作
func isReadOperation(action string) bool {
	readOperations := map[string]bool{
		"list":     true,
		"get":      true,
		"describe": true,
		"query":    true,
		"view":     true,
		"read":     true,
	}
	return readOperations[action]
}
