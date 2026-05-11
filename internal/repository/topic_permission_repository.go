package repository

import (
	"context"

	"kafka-management-platform/internal/models"

	"gorm.io/gorm"
)

// TopicPermissionRepository Topic 权限数据访问接口
type TopicPermissionRepository interface {
	// 创建权限
	Create(ctx context.Context, perm *models.UserTopicPermission) error
	// 批量创建权限
	BatchCreate(ctx context.Context, perms []*models.UserTopicPermission) error
	// 删除权限
	Delete(ctx context.Context, userID, clusterID int64, topicName string) error
	// 删除用户的所有 Topic 权限
	DeleteByUser(ctx context.Context, userID int64) error
	// 删除用户在指定集群的所有 Topic 权限
	DeleteByUserCluster(ctx context.Context, userID, clusterID int64) error
	// 删除集群的所有 Topic 权限
	DeleteByCluster(ctx context.Context, clusterID int64) error
	// 查询用户的 Topic 权限
	ListByUser(ctx context.Context, userID int64) ([]*models.UserTopicPermission, error)
	// 查询用户在指定集群的 Topic 权限
	ListByUserCluster(ctx context.Context, userID, clusterID int64) ([]*models.UserTopicPermission, error)
	// 查询有权限的 Topic 名称列表
	ListTopicsByUserCluster(ctx context.Context, userID, clusterID int64) ([]string, error)
	// 检查用户是否有指定 Topic 的权限
	HasAccess(ctx context.Context, userID, clusterID int64, topicName string) (bool, error)
	// 检查用户是否有任意 Topic 的权限
	HasAnyTopicAccess(ctx context.Context, userID, clusterID int64) (bool, error)
}

type topicPermissionRepository struct {
	db *gorm.DB
}

// NewTopicPermissionRepository 创建 Topic 权限仓库实例
func NewTopicPermissionRepository(db *gorm.DB) TopicPermissionRepository {
	return &topicPermissionRepository{db: db}
}

// Create 创建 Topic 权限
func (r *topicPermissionRepository) Create(ctx context.Context, perm *models.UserTopicPermission) error {
	return r.db.WithContext(ctx).Create(perm).Error
}

// BatchCreate 批量创建 Topic 权限
func (r *topicPermissionRepository) BatchCreate(ctx context.Context, perms []*models.UserTopicPermission) error {
	if len(perms) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(perms, 100).Error
}

// Delete 删除 Topic 权限
func (r *topicPermissionRepository) Delete(ctx context.Context, userID, clusterID int64, topicName string) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND cluster_id = ? AND topic_name = ?", userID, clusterID, topicName).
		Delete(&models.UserTopicPermission{}).Error
}

// DeleteByUser 删除用户的所有 Topic 权限
func (r *topicPermissionRepository) DeleteByUser(ctx context.Context, userID int64) error {
	return r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&models.UserTopicPermission{}).Error
}

// DeleteByUserCluster 删除用户在指定集群的所有 Topic 权限
func (r *topicPermissionRepository) DeleteByUserCluster(ctx context.Context, userID, clusterID int64) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND cluster_id = ?", userID, clusterID).
		Delete(&models.UserTopicPermission{}).Error
}

// DeleteByCluster 删除集群的所有 Topic 权限
func (r *topicPermissionRepository) DeleteByCluster(ctx context.Context, clusterID int64) error {
	return r.db.WithContext(ctx).
		Where("cluster_id = ?", clusterID).
		Delete(&models.UserTopicPermission{}).Error
}

// ListByUser 查询用户的 Topic 权限
func (r *topicPermissionRepository) ListByUser(ctx context.Context, userID int64) ([]*models.UserTopicPermission, error) {
	var perms []*models.UserTopicPermission
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Find(&perms).Error
	return perms, err
}

// ListByUserCluster 查询用户在指定集群的 Topic 权限
func (r *topicPermissionRepository) ListByUserCluster(ctx context.Context, userID, clusterID int64) ([]*models.UserTopicPermission, error) {
	var perms []*models.UserTopicPermission
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND cluster_id = ?", userID, clusterID).
		Find(&perms).Error
	return perms, err
}

// ListTopicsByUserCluster 查询有权限的 Topic 名称列表
func (r *topicPermissionRepository) ListTopicsByUserCluster(ctx context.Context, userID, clusterID int64) ([]string, error) {
	var topics []string
	err := r.db.WithContext(ctx).
		Model(&models.UserTopicPermission{}).
		Where("user_id = ? AND cluster_id = ?", userID, clusterID).
		Pluck("topic_name", &topics).Error
	return topics, err
}

// HasAccess 检查用户是否有指定 Topic 的权限
func (r *topicPermissionRepository) HasAccess(ctx context.Context, userID, clusterID int64, topicName string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.UserTopicPermission{}).
		Where("user_id = ? AND cluster_id = ? AND topic_name = ?", userID, clusterID, topicName).
		Count(&count).Error
	return count > 0, err
}

// HasAnyTopicAccess 检查用户是否有任意 Topic 的权限
func (r *topicPermissionRepository) HasAnyTopicAccess(ctx context.Context, userID, clusterID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.UserTopicPermission{}).
		Where("user_id = ? AND cluster_id = ?", userID, clusterID).
		Count(&count).Error
	return count > 0, err
}
