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
}

// NewPermissionService 创建权限服务实例
func NewPermissionService(
	userRepo repository.UserRepository,
	clusterUserRepo repository.ClusterUserRepository,
) *PermissionService {
	return &PermissionService{
		userRepo:        userRepo,
		clusterUserRepo: clusterUserRepo,
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

	// 只读用户仅有查询权限
	if user.Role == models.RoleReadOnly {
		return isReadOperation(action), nil
	}

	// 集群管理员需要进一步检查集群权限
	return false, nil
}

// CheckClusterPermission 检查用户是否有指定集群的管理权限
func (s *PermissionService) CheckClusterPermission(ctx context.Context, userID, clusterID int64) (bool, error) {
	// 获取用户信息
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return false, err
	}

	// 超级管理员拥有所有集群权限
	if user.Role == models.RoleSuperAdmin {
		return true, nil
	}

	// 只读用户无集群管理权限
	if user.Role == models.RoleReadOnly {
		return false, nil
	}

	// 集群管理员需要检查是否被授权管理该集群
	if user.Role == models.RoleClusterAdmin {
		return s.clusterUserRepo.HasAccess(ctx, clusterID, userID)
	}

	return false, nil
}

// CheckClusterReadPermission 检查用户是否有集群的读权限
func (s *PermissionService) CheckClusterReadPermission(ctx context.Context, userID, clusterID int64) (bool, error) {
	// 获取用户信息
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return false, err
	}

	// 超级管理员拥有所有权限
	if user.Role == models.RoleSuperAdmin {
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

// IsReadOnly 检查用户是否为只读用户
func (s *PermissionService) IsReadOnly(ctx context.Context, userID int64) (bool, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return false, err
	}
	return user.Role == models.RoleReadOnly, nil
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
