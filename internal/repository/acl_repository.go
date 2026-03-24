package repository

import (
	"context"
	"errors"

	"kafka-management-platform/internal/models"

	"gorm.io/gorm"
)

// ACLRepository ACL 数据访问接口
type ACLRepository interface {
	Create(ctx context.Context, acl *models.ACL) error
	Update(ctx context.Context, acl *models.ACL) error
	Delete(ctx context.Context, aclID int64) error
	BatchDelete(ctx context.Context, aclIDs []int64) error
	FindByID(ctx context.Context, aclID int64) (*models.ACL, error)
	List(ctx context.Context, clusterID int64, offset, limit int) ([]*models.ACL, int64, error)
	ListByCluster(ctx context.Context, clusterID int64) ([]*models.ACL, error)
	FilterByTopic(ctx context.Context, clusterID int64, topicName string, offset, limit int) ([]*models.ACL, int64, error)
	FilterByPrincipal(ctx context.Context, clusterID int64, principal string, offset, limit int) ([]*models.ACL, int64, error)
	DeleteByCluster(ctx context.Context, clusterID int64) error
}

type aclRepository struct {
	db *gorm.DB
}

// NewACLRepository 创建 ACL 仓库实例
func NewACLRepository(db *gorm.DB) ACLRepository {
	return &aclRepository{db: db}
}

// Create 创建 ACL
func (r *aclRepository) Create(ctx context.Context, acl *models.ACL) error {
	return r.db.WithContext(ctx).Create(acl).Error
}

// Update 更新 ACL
func (r *aclRepository) Update(ctx context.Context, acl *models.ACL) error {
	return r.db.WithContext(ctx).Save(acl).Error
}

// Delete 删除 ACL
func (r *aclRepository) Delete(ctx context.Context, aclID int64) error {
	return r.db.WithContext(ctx).Delete(&models.ACL{}, aclID).Error
}

// BatchDelete 批量删除 ACL
func (r *aclRepository) BatchDelete(ctx context.Context, aclIDs []int64) error {
	return r.db.WithContext(ctx).Delete(&models.ACL{}, aclIDs).Error
}

// FindByID 根据 ID 查找 ACL
func (r *aclRepository) FindByID(ctx context.Context, aclID int64) (*models.ACL, error) {
	var acl models.ACL
	err := r.db.WithContext(ctx).First(&acl, aclID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrACLNotFound
		}
		return nil, err
	}
	return &acl, nil
}

// List 获取 ACL 列表
func (r *aclRepository) List(ctx context.Context, clusterID int64, offset, limit int) ([]*models.ACL, int64, error) {
	var acls []*models.ACL
	var total int64

	query := r.db.WithContext(ctx).Model(&models.ACL{})
	if clusterID > 0 {
		query = query.Where("cluster_id = ?", clusterID)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 获取分页数据
	err := query.
		Offset(offset).
		Limit(limit).
		Order("created_at DESC").
		Find(&acls).Error

	return acls, total, err
}

// ListByCluster 获取集群的所有 ACL
func (r *aclRepository) ListByCluster(ctx context.Context, clusterID int64) ([]*models.ACL, error) {
	var acls []*models.ACL
	err := r.db.WithContext(ctx).
		Where("cluster_id = ?", clusterID).
		Find(&acls).Error
	return acls, err
}

// FilterByTopic 按 Topic 过滤 ACL
func (r *aclRepository) FilterByTopic(ctx context.Context, clusterID int64, topicName string, offset, limit int) ([]*models.ACL, int64, error) {
	var acls []*models.ACL
	var total int64

	query := r.db.WithContext(ctx).Model(&models.ACL{}).
		Where("cluster_id = ? AND resource_type = ? AND resource_name = ?",
			clusterID, models.ResourceTypeTopic, topicName)

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 获取分页数据
	err := query.
		Offset(offset).
		Limit(limit).
		Order("created_at DESC").
		Find(&acls).Error

	return acls, total, err
}

// FilterByPrincipal 按 Principal 过滤 ACL
func (r *aclRepository) FilterByPrincipal(ctx context.Context, clusterID int64, principal string, offset, limit int) ([]*models.ACL, int64, error) {
	var acls []*models.ACL
	var total int64

	query := r.db.WithContext(ctx).Model(&models.ACL{}).
		Where("cluster_id = ? AND principal = ?", clusterID, principal)

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 获取分页数据
	err := query.
		Offset(offset).
		Limit(limit).
		Order("created_at DESC").
		Find(&acls).Error

	return acls, total, err
}

// DeleteByCluster 删除集群的所有 ACL
func (r *aclRepository) DeleteByCluster(ctx context.Context, clusterID int64) error {
	return r.db.WithContext(ctx).
		Where("cluster_id = ?", clusterID).
		Delete(&models.ACL{}).Error
}
