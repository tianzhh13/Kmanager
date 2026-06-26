package repository

import (
	"context"
	"time"

	"kafka-management-platform/internal/models"

	"gorm.io/gorm"
)

// TokenBlacklistRepository Token 黑名单数据访问接口
type TokenBlacklistRepository interface {
	Create(ctx context.Context, entry *models.TokenBlacklist) error
	IsBlacklisted(ctx context.Context, tokenHash string) (bool, error)
	DeleteExpired(ctx context.Context) (int64, error)
	LoadActive(ctx context.Context) ([]*models.TokenBlacklist, error)
}

type tokenBlacklistRepository struct {
	db *gorm.DB
}

// NewTokenBlacklistRepository 创建 Token 黑名单仓库实例
func NewTokenBlacklistRepository(db *gorm.DB) TokenBlacklistRepository {
	return &tokenBlacklistRepository{db: db}
}

// Create 创建黑名单记录
func (r *tokenBlacklistRepository) Create(ctx context.Context, entry *models.TokenBlacklist) error {
	return r.db.WithContext(ctx).Create(entry).Error
}

// IsBlacklisted 检查 token hash 是否在黑名单中
func (r *tokenBlacklistRepository) IsBlacklisted(ctx context.Context, tokenHash string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.TokenBlacklist{}).
		Where("token_hash = ? AND expires_at > ?", tokenHash, time.Now()).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// DeleteExpired 删除过期的黑名单记录
func (r *tokenBlacklistRepository) DeleteExpired(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("expires_at <= ?", time.Now()).
		Delete(&models.TokenBlacklist{})
	return result.RowsAffected, result.Error
}

// LoadActive 加载所有未过期的黑名单记录（启动时预热缓存）
func (r *tokenBlacklistRepository) LoadActive(ctx context.Context) ([]*models.TokenBlacklist, error) {
	var entries []*models.TokenBlacklist
	err := r.db.WithContext(ctx).
		Where("expires_at > ?", time.Now()).
		Find(&entries).Error
	return entries, err
}
