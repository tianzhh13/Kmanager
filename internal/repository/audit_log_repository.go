package repository

import (
	"context"
	"time"

	"kafka-management-platform/internal/models"

	"gorm.io/gorm"
)

// AuditLogRepository 审计日志数据访问接口
type AuditLogRepository interface {
	Create(ctx context.Context, log *models.AuditLog) error
	FindByID(ctx context.Context, logID int64) (*models.AuditLog, error)
	List(ctx context.Context, filter *AuditLogFilter, offset, limit int) ([]*models.AuditLog, int64, error)
	Query(ctx context.Context, userID *int64, username, action, resourceType, status *string, startTime, endTime *time.Time, limit, offset int) ([]models.AuditLog, int64, error)
	DeleteBefore(ctx context.Context, before time.Time) (int64, error)
}

// AuditLogFilter 审计日志过滤条件
type AuditLogFilter struct {
	UserID     *int64
	Action     string
	Resource   string
	ClusterID  *int64
	Status     models.AuditStatus
	StartTime  *time.Time
	EndTime    *time.Time
}

type auditLogRepository struct {
	db *gorm.DB
}

// NewAuditLogRepository 创建审计日志仓库实例
func NewAuditLogRepository(db *gorm.DB) AuditLogRepository {
	return &auditLogRepository{db: db}
}

// Create 创建审计日志
func (r *auditLogRepository) Create(ctx context.Context, log *models.AuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// FindByID 根据 ID 查找审计日志
func (r *auditLogRepository) FindByID(ctx context.Context, logID int64) (*models.AuditLog, error) {
	var log models.AuditLog
	err := r.db.WithContext(ctx).First(&log, logID).Error
	return &log, err
}

// List 获取审计日志列表
func (r *auditLogRepository) List(ctx context.Context, filter *AuditLogFilter, offset, limit int) ([]*models.AuditLog, int64, error) {
	var logs []*models.AuditLog
	var total int64

	query := r.db.WithContext(ctx).Model(&models.AuditLog{})

	// 应用过滤条件
	if filter != nil {
		if filter.UserID != nil {
			query = query.Where("user_id = ?", *filter.UserID)
		}
		if filter.Action != "" {
			query = query.Where("action = ?", filter.Action)
		}
		if filter.Resource != "" {
			query = query.Where("resource = ?", filter.Resource)
		}
		if filter.ClusterID != nil {
			query = query.Where("cluster_id = ?", *filter.ClusterID)
		}
		if filter.Status != "" {
			query = query.Where("status = ?", filter.Status)
		}
		if filter.StartTime != nil {
			query = query.Where("created_at >= ?", *filter.StartTime)
		}
		if filter.EndTime != nil {
			query = query.Where("created_at <= ?", *filter.EndTime)
		}
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
		Find(&logs).Error

	return logs, total, err
}

// DeleteBefore 删除指定时间之前的审计日志
func (r *auditLogRepository) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("created_at < ?", before).
		Delete(&models.AuditLog{})
	return result.RowsAffected, result.Error
}

// Query 查询审计日志（兼容服务层调用）
func (r *auditLogRepository) Query(ctx context.Context, userID *int64, username, action, resourceType, status *string, startTime, endTime *time.Time, limit, offset int) ([]models.AuditLog, int64, error) {
	var logs []models.AuditLog
	var total int64

	query := r.db.WithContext(ctx).Model(&models.AuditLog{})

	// 应用过滤条件
	if userID != nil && *userID > 0 {
		query = query.Where("user_id = ?", *userID)
	}
	if username != nil && *username != "" {
		query = query.Where("username = ?", *username)
	}
	if action != nil && *action != "" {
		query = query.Where("action = ?", *action)
	}
	if resourceType != nil && *resourceType != "" {
		query = query.Where("resource = ?", *resourceType)
	}
	if status != nil && *status != "" {
		query = query.Where("status = ?", *status)
	}
	if startTime != nil {
		query = query.Where("created_at >= ?", *startTime)
	}
	if endTime != nil {
		query = query.Where("created_at <= ?", *endTime)
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
		Find(&logs).Error

	return logs, total, err
}