package repository

import (
	"context"
	"errors"

	"kafka-management-platform/internal/models"

	"gorm.io/gorm"
)

// TopicRepository Topic 数据访问接口
type TopicRepository interface {
	Create(ctx context.Context, topic *models.Topic) error
	Update(ctx context.Context, topic *models.Topic) error
	Delete(ctx context.Context, topicID int64) error
	FindByID(ctx context.Context, topicID int64) (*models.Topic, error)
	FindByName(ctx context.Context, clusterID int64, topicName string) (*models.Topic, error)
	Exists(ctx context.Context, clusterID int64, topicName string) (bool, error)
	List(ctx context.Context, clusterID int64, offset, limit int) ([]*models.Topic, int64, error)
	Search(ctx context.Context, clusterID int64, keyword string, offset, limit int) ([]*models.Topic, int64, error)
	ListByCluster(ctx context.Context, clusterID int64) ([]*models.Topic, error)
	DeleteByCluster(ctx context.Context, clusterID int64) error
}

type topicRepository struct {
	db *gorm.DB
}

// NewTopicRepository 创建 Topic 仓库实例
func NewTopicRepository(db *gorm.DB) TopicRepository {
	return &topicRepository{db: db}
}

// Create 创建 Topic
func (r *topicRepository) Create(ctx context.Context, topic *models.Topic) error {
	return r.db.WithContext(ctx).Create(topic).Error
}

// Update 更新 Topic
func (r *topicRepository) Update(ctx context.Context, topic *models.Topic) error {
	return r.db.WithContext(ctx).Save(topic).Error
}

// Delete 删除 Topic
func (r *topicRepository) Delete(ctx context.Context, topicID int64) error {
	return r.db.WithContext(ctx).Delete(&models.Topic{}, topicID).Error
}

// FindByID 根据 ID 查找 Topic
func (r *topicRepository) FindByID(ctx context.Context, topicID int64) (*models.Topic, error) {
	var topic models.Topic
	err := r.db.WithContext(ctx).First(&topic, topicID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTopicNotFound
		}
		return nil, err
	}
	return &topic, nil
}

// FindByName 根据集群和名称查找 Topic
func (r *topicRepository) FindByName(ctx context.Context, clusterID int64, topicName string) (*models.Topic, error) {
	var topic models.Topic
	err := r.db.WithContext(ctx).
		Where("cluster_id = ? AND topic_name = ?", clusterID, topicName).
		First(&topic).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTopicNotFound
		}
		return nil, err
	}
	return &topic, nil
}

// Exists 检查 Topic 是否存在
func (r *topicRepository) Exists(ctx context.Context, clusterID int64, topicName string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Topic{}).
		Where("cluster_id = ? AND topic_name = ?", clusterID, topicName).
		Count(&count).Error
	return count > 0, err
}

// List 获取 Topic 列表
func (r *topicRepository) List(ctx context.Context, clusterID int64, offset, limit int) ([]*models.Topic, int64, error) {
	var topics []*models.Topic
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Topic{})
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
		Find(&topics).Error

	return topics, total, err
}

// Search 搜索 Topic
func (r *topicRepository) Search(ctx context.Context, clusterID int64, keyword string, offset, limit int) ([]*models.Topic, int64, error) {
	var topics []*models.Topic
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Topic{}).
		Where("topic_name LIKE ?", "%"+keyword+"%")

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
		Find(&topics).Error

	return topics, total, err
}

// ListByCluster 获取集群的所有 Topic
func (r *topicRepository) ListByCluster(ctx context.Context, clusterID int64) ([]*models.Topic, error) {
	var topics []*models.Topic
	err := r.db.WithContext(ctx).
		Where("cluster_id = ?", clusterID).
		Find(&topics).Error
	return topics, err
}

// DeleteByCluster 删除集群的所有 Topic
func (r *topicRepository) DeleteByCluster(ctx context.Context, clusterID int64) error {
	return r.db.WithContext(ctx).
		Where("cluster_id = ?", clusterID).
		Delete(&models.Topic{}).Error
}
