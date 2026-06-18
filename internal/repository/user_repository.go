package repository

import (
	"context"
	"errors"

	"kafka-management-platform/internal/models"

	"gorm.io/gorm"
)

// UserRepository 用户数据访问接口
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	Update(ctx context.Context, user *models.User) error
	Delete(ctx context.Context, userID int64) error
	FindByID(ctx context.Context, userID int64) (*models.User, error)
	FindByUsername(ctx context.Context, username string) (*models.User, error)
	List(ctx context.Context, offset, limit int) ([]*models.User, int64, error)
	Search(ctx context.Context, keyword string, offset, limit int) ([]*models.User, int64, error)
	CountActive(ctx context.Context) (int64, error)
	CountByRole(ctx context.Context) (map[string]int64, error)
}

type userRepository struct {
	db *gorm.DB
}

// NewUserRepository 创建用户仓库实例
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// Create 创建用户
func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// Update 更新用户
func (r *userRepository) Update(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

// Delete 删除用户
func (r *userRepository) Delete(ctx context.Context, userID int64) error {
	return r.db.WithContext(ctx).Delete(&models.User{}, userID).Error
}

// FindByID 根据 ID 查找用户
func (r *userRepository) FindByID(ctx context.Context, userID int64) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).First(&user, userID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// FindByUsername 根据用户名查找用户
func (r *userRepository) FindByUsername(ctx context.Context, username string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// List 获取用户列表
func (r *userRepository) List(ctx context.Context, offset, limit int) ([]*models.User, int64, error) {
	var users []*models.User
	var total int64

	// 获取总数
	if err := r.db.WithContext(ctx).Model(&models.User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 获取分页数据
	err := r.db.WithContext(ctx).
		Offset(offset).
		Limit(limit).
		Order("created_at DESC").
		Find(&users).Error

	return users, total, err
}

// Search 搜索用户
func (r *userRepository) Search(ctx context.Context, keyword string, offset, limit int) ([]*models.User, int64, error) {
	var users []*models.User
	var total int64

	safeKeyword := escapeLikeKeyword(keyword)
	query := r.db.WithContext(ctx).Model(&models.User{}).
		Where("username LIKE ? OR email LIKE ?", "%"+safeKeyword+"%", "%"+safeKeyword+"%")

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 获取分页数据
	err := query.
		Offset(offset).
		Limit(limit).
		Order("created_at DESC").
		Find(&users).Error

	return users, total, err
}

// CountActive 统计活跃用户数量
func (r *userRepository) CountActive(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.User{}).
		Where("status = ?", models.UserStatusActive).
		Count(&count).Error
	return count, err
}

// CountByRole 按角色统计活跃用户数量
func (r *userRepository) CountByRole(ctx context.Context) (map[string]int64, error) {
	type result struct {
		Role  string `json:"role"`
		Count int64  `json:"count"`
	}
	var results []result
	err := r.db.WithContext(ctx).
		Model(&models.User{}).
		Select("role, COUNT(*) as count").
		Where("status = ?", models.UserStatusActive).
		Group("role").
		Find(&results).Error
	if err != nil {
		return nil, err
	}
	m := make(map[string]int64, len(results))
	for _, r := range results {
		m[r.Role] = r.Count
	}
	return m, nil
}
