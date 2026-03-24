package repository

import (
	"context"

	"kafka-management-platform/internal/models"

	"gorm.io/gorm"
)

// ClusterUserRepository 集群用户关联数据访问接口
type ClusterUserRepository interface {
	Create(ctx context.Context, relation *models.ClusterUserRelation) error
	Delete(ctx context.Context, clusterID, userID int64) error
	HasAccess(ctx context.Context, clusterID, userID int64) (bool, error)
	ListUsersByCluster(ctx context.Context, clusterID int64) ([]*models.User, error)
	ListClustersByUser(ctx context.Context, userID int64) ([]*models.Cluster, error)
	DeleteByCluster(ctx context.Context, clusterID int64) error
	DeleteByUser(ctx context.Context, userID int64) error
}

type clusterUserRepository struct {
	db *gorm.DB
}

// NewClusterUserRepository 创建集群用户关联仓库实例
func NewClusterUserRepository(db *gorm.DB) ClusterUserRepository {
	return &clusterUserRepository{db: db}
}

// Create 创建集群用户关联
func (r *clusterUserRepository) Create(ctx context.Context, relation *models.ClusterUserRelation) error {
	return r.db.WithContext(ctx).Create(relation).Error
}

// Delete 删除集群用户关联
func (r *clusterUserRepository) Delete(ctx context.Context, clusterID, userID int64) error {
	return r.db.WithContext(ctx).
		Where("cluster_id = ? AND user_id = ?", clusterID, userID).
		Delete(&models.ClusterUserRelation{}).Error
}

// HasAccess 检查用户是否有集群访问权限
func (r *clusterUserRepository) HasAccess(ctx context.Context, clusterID, userID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.ClusterUserRelation{}).
		Where("cluster_id = ? AND user_id = ?", clusterID, userID).
		Count(&count).Error
	return count > 0, err
}

// ListUsersByCluster 获取集群的授权用户列表
func (r *clusterUserRepository) ListUsersByCluster(ctx context.Context, clusterID int64) ([]*models.User, error) {
	var users []*models.User
	err := r.db.WithContext(ctx).
		Table("user").
		Joins("INNER JOIN cluster_user_relation ON user.user_id = cluster_user_relation.user_id").
		Where("cluster_user_relation.cluster_id = ?", clusterID).
		Find(&users).Error
	return users, err
}

// ListClustersByUser 获取用户有权限的集群列表
func (r *clusterUserRepository) ListClustersByUser(ctx context.Context, userID int64) ([]*models.Cluster, error) {
	var clusters []*models.Cluster
	err := r.db.WithContext(ctx).
		Table("cluster").
		Joins("INNER JOIN cluster_user_relation ON cluster.cluster_id = cluster_user_relation.cluster_id").
		Where("cluster_user_relation.user_id = ?", userID).
		Find(&clusters).Error
	return clusters, err
}

// DeleteByCluster 删除集群的所有用户关联
func (r *clusterUserRepository) DeleteByCluster(ctx context.Context, clusterID int64) error {
	return r.db.WithContext(ctx).
		Where("cluster_id = ?", clusterID).
		Delete(&models.ClusterUserRelation{}).Error
}

// DeleteByUser 删除用户的所有集群关联
func (r *clusterUserRepository) DeleteByUser(ctx context.Context, userID int64) error {
	return r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&models.ClusterUserRelation{}).Error
}
