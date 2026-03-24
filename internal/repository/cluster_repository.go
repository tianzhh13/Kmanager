package repository

import (
	"context"
	"errors"

	"kafka-management-platform/internal/models"

	"gorm.io/gorm"
)

// ClusterRepository 集群数据访问接口
type ClusterRepository interface {
	Create(ctx context.Context, cluster *models.Cluster) error
	Update(ctx context.Context, cluster *models.Cluster) error
	Delete(ctx context.Context, clusterID int64) error
	FindByID(ctx context.Context, clusterID int64) (*models.Cluster, error)
	FindByName(ctx context.Context, clusterName string) (*models.Cluster, error)
	List(ctx context.Context, offset, limit int) ([]*models.Cluster, int64, error)
	ListByUser(ctx context.Context, userID int64, role models.UserRole, offset, limit int) ([]*models.Cluster, int64, error)
}

type clusterRepository struct {
	db *gorm.DB
}

// NewClusterRepository 创建集群仓库实例
func NewClusterRepository(db *gorm.DB) ClusterRepository {
	return &clusterRepository{db: db}
}

// Create 创建集群
func (r *clusterRepository) Create(ctx context.Context, cluster *models.Cluster) error {
	return r.db.WithContext(ctx).Create(cluster).Error
}

// Update 更新集群
func (r *clusterRepository) Update(ctx context.Context, cluster *models.Cluster) error {
	return r.db.WithContext(ctx).Save(cluster).Error
}

// Delete 删除集群
func (r *clusterRepository) Delete(ctx context.Context, clusterID int64) error {
	return r.db.WithContext(ctx).Delete(&models.Cluster{}, clusterID).Error
}

// FindByID 根据 ID 查找集群
func (r *clusterRepository) FindByID(ctx context.Context, clusterID int64) (*models.Cluster, error) {
	var cluster models.Cluster
	err := r.db.WithContext(ctx).First(&cluster, clusterID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrClusterNotFound
		}
		return nil, err
	}
	return &cluster, nil
}

// FindByName 根据名称查找集群
func (r *clusterRepository) FindByName(ctx context.Context, clusterName string) (*models.Cluster, error) {
	var cluster models.Cluster
	err := r.db.WithContext(ctx).Where("cluster_name = ?", clusterName).First(&cluster).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrClusterNotFound
		}
		return nil, err
	}
	return &cluster, nil
}

// List 获取集群列表
func (r *clusterRepository) List(ctx context.Context, offset, limit int) ([]*models.Cluster, int64, error) {
	var clusters []*models.Cluster
	var total int64

	// 获取总数
	if err := r.db.WithContext(ctx).Model(&models.Cluster{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 获取分页数据
	err := r.db.WithContext(ctx).
		Offset(offset).
		Limit(limit).
		Order("created_at DESC").
		Find(&clusters).Error

	return clusters, total, err
}

// ListByUser 根据用户获取集群列表
func (r *clusterRepository) ListByUser(ctx context.Context, userID int64, role models.UserRole, offset, limit int) ([]*models.Cluster, int64, error) {
	var clusters []*models.Cluster
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Cluster{})

	// 如果不是超级管理员，只返回有权限的集群
	if role != models.RoleSuperAdmin {
		query = query.Joins("INNER JOIN cluster_user_relation ON cluster.cluster_id = cluster_user_relation.cluster_id").
			Where("cluster_user_relation.user_id = ?", userID)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 获取分页数据
	err := query.
		Offset(offset).
		Limit(limit).
		Order("cluster.created_at DESC").
		Find(&clusters).Error

	return clusters, total, err
}
