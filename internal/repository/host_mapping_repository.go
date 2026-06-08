package repository

import (
	"context"

	"kafka-management-platform/internal/models"

	"gorm.io/gorm"
)

// HostMappingRepository 主机映射仓储
type HostMappingRepository struct {
	db *gorm.DB
}

// NewHostMappingRepository 创建主机映射仓储实例
func NewHostMappingRepository(db *gorm.DB) *HostMappingRepository {
	return &HostMappingRepository{db: db}
}

// Create 创建主机映射
func (r *HostMappingRepository) Create(ctx context.Context, mapping *models.HostMapping) error {
	return r.db.WithContext(ctx).Create(mapping).Error
}

// Update 更新主机映射
func (r *HostMappingRepository) Update(ctx context.Context, mapping *models.HostMapping) error {
	return r.db.WithContext(ctx).Save(mapping).Error
}

// Delete 删除主机映射
func (r *HostMappingRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&models.HostMapping{}, id).Error
}

// GetByID 根据 ID 获取主机映射
func (r *HostMappingRepository) GetByID(ctx context.Context, id int64) (*models.HostMapping, error) {
	var mapping models.HostMapping
	err := r.db.WithContext(ctx).First(&mapping, id).Error
	if err != nil {
		return nil, err
	}
	return &mapping, nil
}

// GetByHostname 根据主机名获取映射
func (r *HostMappingRepository) GetByHostname(ctx context.Context, hostname string) (*models.HostMapping, error) {
	var mapping models.HostMapping
	err := r.db.WithContext(ctx).Where("hostname = ?", hostname).First(&mapping).Error
	if err != nil {
		return nil, err
	}
	return &mapping, nil
}

// List 获取所有主机映射
func (r *HostMappingRepository) List(ctx context.Context) ([]models.HostMapping, error) {
	var mappings []models.HostMapping
	err := r.db.WithContext(ctx).Order("hostname ASC").Find(&mappings).Error
	return mappings, err
}

// ListWithPagination 分页获取主机映射
func (r *HostMappingRepository) ListWithPagination(ctx context.Context, page, pageSize int, keyword string) ([]models.HostMapping, int64, error) {
	var mappings []models.HostMapping
	var total int64

	query := r.db.WithContext(ctx).Model(&models.HostMapping{})
	if keyword != "" {
		query = query.Where("hostname LIKE ? OR ip_address LIKE ? OR description LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Order("hostname ASC").Offset(offset).Limit(pageSize).Find(&mappings).Error; err != nil {
		return nil, 0, err
	}

	return mappings, total, nil
}
