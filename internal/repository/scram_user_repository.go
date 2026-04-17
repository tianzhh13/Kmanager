package repository

import (
	"context"

	"kafka-management-platform/internal/models"

	"gorm.io/gorm"
)

// ScramUserRepository SCRAM 用户仓库接口
type ScramUserRepository interface {
	Create(ctx context.Context, user *models.ScramUser) error
	Update(ctx context.Context, user *models.ScramUser) error
	Delete(ctx context.Context, userID int64) error
	FindByID(ctx context.Context, userID int64) (*models.ScramUser, error)
	FindByUsername(ctx context.Context, clusterID int64, username string) (*models.ScramUser, error)
	List(ctx context.Context, clusterID int64, offset, limit int) ([]*models.ScramUser, int64, error)
	ListByCluster(ctx context.Context, clusterID int64) ([]*models.ScramUser, error)
	Exists(ctx context.Context, clusterID int64, username string) (bool, error)
}

// scramUserRepository SCRAM 用户仓库实现
type scramUserRepository struct {
	db *gorm.DB
}

// NewScramUserRepository 创建 SCRAM 用户仓库实例
func NewScramUserRepository(db *gorm.DB) ScramUserRepository {
	return &scramUserRepository{db: db}
}

func (r *scramUserRepository) Create(ctx context.Context, user *models.ScramUser) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *scramUserRepository) Update(ctx context.Context, user *models.ScramUser) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *scramUserRepository) Delete(ctx context.Context, userID int64) error {
	return r.db.WithContext(ctx).Delete(&models.ScramUser{}, userID).Error
}

func (r *scramUserRepository) FindByID(ctx context.Context, userID int64) (*models.ScramUser, error) {
	var user models.ScramUser
	err := r.db.WithContext(ctx).First(&user, userID).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *scramUserRepository) FindByUsername(ctx context.Context, clusterID int64, username string) (*models.ScramUser, error) {
	var user models.ScramUser
	err := r.db.WithContext(ctx).Where("cluster_id = ? AND username = ?", clusterID, username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *scramUserRepository) List(ctx context.Context, clusterID int64, offset, limit int) ([]*models.ScramUser, int64, error) {
	var users []*models.ScramUser
	var total int64

	query := r.db.WithContext(ctx).Model(&models.ScramUser{}).Where("cluster_id = ?", clusterID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func (r *scramUserRepository) ListByCluster(ctx context.Context, clusterID int64) ([]*models.ScramUser, error) {
	var users []*models.ScramUser
	err := r.db.WithContext(ctx).Where("cluster_id = ?", clusterID).Find(&users).Error
	return users, err
}

func (r *scramUserRepository) Exists(ctx context.Context, clusterID int64, username string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.ScramUser{}).Where("cluster_id = ? AND username = ?", clusterID, username).Count(&count).Error
	return count > 0, err
}
