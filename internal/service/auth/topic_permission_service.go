package auth

import (
	"context"
	"errors"
	"fmt"

	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/repository"

	"gorm.io/gorm"
)

// TopicPermissionService Topic 权限服务
type TopicPermissionService struct {
	topicPermRepo   repository.TopicPermissionRepository
	clusterUserRepo repository.ClusterUserRepository
	userRepo        repository.UserRepository
	clusterRepo     repository.ClusterRepository
}

// NewTopicPermissionService 创建 Topic 权限服务实例
func NewTopicPermissionService(
	topicPermRepo repository.TopicPermissionRepository,
	clusterUserRepo repository.ClusterUserRepository,
	userRepo repository.UserRepository,
	clusterRepo repository.ClusterRepository,
) *TopicPermissionService {
	return &TopicPermissionService{
		topicPermRepo:   topicPermRepo,
		clusterUserRepo: clusterUserRepo,
		userRepo:        userRepo,
		clusterRepo:     clusterRepo,
	}
}

// AssignTopicPermissionRequest 分配 Topic 权限请求
type AssignTopicPermissionRequest struct {
	UserID    int64  `json:"user_id" binding:"required"`
	ClusterID int64  `json:"cluster_id" binding:"required"`
	TopicName string `json:"topic_name" binding:"required"`
}

// BatchAssignTopicPermissionRequest 批量分配 Topic 权限请求
type BatchAssignTopicPermissionRequest struct {
	UserID     int64    `json:"user_id" binding:"required"`
	ClusterID  int64    `json:"cluster_id" binding:"required"`
	TopicNames []string `json:"topic_names" binding:"required,min=1,max=100"`
}

// RevokeTopicPermissionRequest 撤销 Topic 权限请求
type RevokeTopicPermissionRequest struct {
	UserID    int64  `json:"user_id" binding:"required"`
	ClusterID int64  `json:"cluster_id" binding:"required"`
	TopicName string `json:"topic_name" binding:"required"`
}

// TopicPermissionResponse Topic 权限响应
type TopicPermissionResponse struct {
	ID          int64  `json:"id"`
	UserID      int64  `json:"user_id"`
	Username    string `json:"username"`
	ClusterID   int64  `json:"cluster_id"`
	ClusterName string `json:"cluster_name"`
	TopicName   string `json:"topic_name"`
	CreatedAt   string `json:"created_at"`
	CreatedBy   int64  `json:"created_by"`
}

// AssignTopicPermission 分配 Topic 权限
func (s *TopicPermissionService) AssignTopicPermission(ctx context.Context, req *AssignTopicPermissionRequest, operatorID int64) error {
	// 验证用户是否存在
	user, err := s.userRepo.FindByID(ctx, req.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("user not found")
		}
		return err
	}

	// 只能给普通用户分配 Topic 权限
	if user.Role != models.RoleNormalUser {
		return errors.New("can only assign topic permissions to normal users")
	}

	// 验证集群是否存在
	_, err = s.clusterRepo.FindByID(ctx, req.ClusterID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("cluster not found")
		}
		return err
	}

	// 如果用户没有集群访问权限，自动授权
	hasAccess, err := s.clusterUserRepo.HasAccess(ctx, req.ClusterID, req.UserID)
	if err != nil {
		return err
	}
	if !hasAccess {
		// 自动为用户分配集群访问权限
		relation := &models.ClusterUserRelation{
			ClusterID: req.ClusterID,
			UserID:    req.UserID,
		}
		if err := s.clusterUserRepo.Create(ctx, relation); err != nil {
			return fmt.Errorf("failed to grant cluster access: %w", err)
		}
	}

	// 创建权限
	perm := &models.UserTopicPermission{
		UserID:    req.UserID,
		ClusterID: req.ClusterID,
		TopicName: req.TopicName,
		CreatedBy: operatorID,
	}

	return s.topicPermRepo.Create(ctx, perm)
}

// BatchAssignTopicPermission 批量分配 Topic 权限
func (s *TopicPermissionService) BatchAssignTopicPermission(ctx context.Context, req *BatchAssignTopicPermissionRequest, operatorID int64) error {
	// 验证用户是否存在
	user, err := s.userRepo.FindByID(ctx, req.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("user not found")
		}
		return err
	}

	// 只能给普通用户分配 Topic 权限
	if user.Role != models.RoleNormalUser {
		return errors.New("can only assign topic permissions to normal users")
	}

	// 验证集群是否存在
	_, err = s.clusterRepo.FindByID(ctx, req.ClusterID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("cluster not found")
		}
		return err
	}

	// 如果用户没有集群访问权限，自动授权
	hasAccess, err := s.clusterUserRepo.HasAccess(ctx, req.ClusterID, req.UserID)
	if err != nil {
		return err
	}
	if !hasAccess {
		relation := &models.ClusterUserRelation{
			ClusterID: req.ClusterID,
			UserID:    req.UserID,
		}
		if err := s.clusterUserRepo.Create(ctx, relation); err != nil {
			return fmt.Errorf("failed to grant cluster access: %w", err)
		}
	}

	// 批量创建权限
	perms := make([]*models.UserTopicPermission, 0, len(req.TopicNames))
	for _, topicName := range req.TopicNames {
		perms = append(perms, &models.UserTopicPermission{
			UserID:    req.UserID,
			ClusterID: req.ClusterID,
			TopicName: topicName,
			CreatedBy: operatorID,
		})
	}

	return s.topicPermRepo.BatchCreate(ctx, perms)
}

// RevokeTopicPermission 撤销 Topic 权限
func (s *TopicPermissionService) RevokeTopicPermission(ctx context.Context, req *RevokeTopicPermissionRequest) error {
	return s.topicPermRepo.Delete(ctx, req.UserID, req.ClusterID, req.TopicName)
}

// GetUserTopicPermissions 获取用户的 Topic 权限列表
func (s *TopicPermissionService) GetUserTopicPermissions(ctx context.Context, userID int64) ([]*TopicPermissionResponse, error) {
	perms, err := s.topicPermRepo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	var responses []*TopicPermissionResponse
	for _, perm := range perms {
		// 获取集群名称
		cluster, err := s.clusterRepo.FindByID(ctx, perm.ClusterID)
		if err != nil {
			continue
		}

		responses = append(responses, &TopicPermissionResponse{
			ID:          perm.ID,
			UserID:      perm.UserID,
			ClusterID:   perm.ClusterID,
			ClusterName: cluster.ClusterName,
			TopicName:   perm.TopicName,
			CreatedAt:   perm.CreatedAt.Format("2006-01-02 15:04:05"),
			CreatedBy:   perm.CreatedBy,
		})
	}

	return responses, nil
}

// GetUserClusterTopicPermissions 获取用户在指定集群的 Topic 权限列表
func (s *TopicPermissionService) GetUserClusterTopicPermissions(ctx context.Context, userID, clusterID int64) ([]string, error) {
	return s.topicPermRepo.ListTopicsByUserCluster(ctx, userID, clusterID)
}

// CheckTopicPermission 检查用户是否有指定 Topic 的权限
func (s *TopicPermissionService) CheckTopicPermission(ctx context.Context, userID, clusterID int64, topicName string) (bool, error) {
	return s.topicPermRepo.HasAccess(ctx, userID, clusterID, topicName)
}

// RevokeAllUserTopicPermissions 撤销用户的所有 Topic 权限
func (s *TopicPermissionService) RevokeAllUserTopicPermissions(ctx context.Context, userID int64) error {
	return s.topicPermRepo.DeleteByUser(ctx, userID)
}

// RevokeUserClusterTopicPermissions 撤销用户在指定集群的所有 Topic 权限
func (s *TopicPermissionService) RevokeUserClusterTopicPermissions(ctx context.Context, userID, clusterID int64) error {
	return s.topicPermRepo.DeleteByUserCluster(ctx, userID, clusterID)
}
