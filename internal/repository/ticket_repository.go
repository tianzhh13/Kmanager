package repository

import (
	"context"

	"kafka-management-platform/internal/models"

	"gorm.io/gorm"
)

// TicketListFilter 工单列表过滤条件
type TicketListFilter struct {
	ApplicantID *int64  // 按申请人过滤
	ClusterIDs  []int64 // 按集群集合过滤（空切片表示无匹配，nil 表示不过滤）
	Status      string
	Type        string
	ClusterID   *int64
	Offset      int
	Limit       int
}

// TicketRepository 工单仓库接口
type TicketRepository interface {
	Create(ctx context.Context, ticket *models.Ticket) error
	Update(ctx context.Context, ticket *models.Ticket) error
	FindByID(ctx context.Context, ticketID int64) (*models.Ticket, error)
	List(ctx context.Context, filter *TicketListFilter) ([]*models.Ticket, int64, error)
	CountByStatus(ctx context.Context, applicantID *int64) (map[string]int64, error)
	// MaxTicketNoByPrefix 获取指定前缀（如 TK-20261105-）下最大的工单号，用于生成当天序号
	MaxTicketNoByPrefix(ctx context.Context, prefix string) (string, error)
}

// ticketRepository 工单仓库实现
type ticketRepository struct {
	db *gorm.DB
}

// NewTicketRepository 创建工单仓库实例
func NewTicketRepository(db *gorm.DB) TicketRepository {
	return &ticketRepository{db: db}
}

func (r *ticketRepository) Create(ctx context.Context, ticket *models.Ticket) error {
	return r.db.WithContext(ctx).Create(ticket).Error
}

func (r *ticketRepository) Update(ctx context.Context, ticket *models.Ticket) error {
	return r.db.WithContext(ctx).Save(ticket).Error
}

func (r *ticketRepository) FindByID(ctx context.Context, ticketID int64) (*models.Ticket, error) {
	var ticket models.Ticket
	err := r.db.WithContext(ctx).First(&ticket, ticketID).Error
	if err != nil {
		return nil, err
	}
	return &ticket, nil
}

func (r *ticketRepository) List(ctx context.Context, filter *TicketListFilter) ([]*models.Ticket, int64, error) {
	var tickets []*models.Ticket
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Ticket{})

	if filter.ApplicantID != nil {
		query = query.Where("applicant_id = ?", *filter.ApplicantID)
	}
	if filter.ClusterIDs != nil {
		if len(filter.ClusterIDs) == 0 {
			return []*models.Ticket{}, 0, nil
		}
		query = query.Where("cluster_id IN ?", filter.ClusterIDs)
	}
	if filter.ClusterID != nil {
		query = query.Where("cluster_id = ?", *filter.ClusterID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("ticket_id DESC").Offset(filter.Offset).Limit(filter.Limit).Find(&tickets).Error; err != nil {
		return nil, 0, err
	}

	return tickets, total, nil
}

func (r *ticketRepository) CountByStatus(ctx context.Context, applicantID *int64) (map[string]int64, error) {
	var rows []struct {
		Status string
		Count  int64
	}

	query := r.db.WithContext(ctx).Model(&models.Ticket{})
	if applicantID != nil {
		query = query.Where("applicant_id = ?", *applicantID)
	}

	err := query.
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]int64)
	for _, row := range rows {
		result[row.Status] = row.Count
	}
	return result, nil
}

func (r *ticketRepository) MaxTicketNoByPrefix(ctx context.Context, prefix string) (string, error) {
	var ticketNo string
	err := r.db.WithContext(ctx).Model(&models.Ticket{}).
		Where("ticket_no LIKE ?", prefix+"%").
		Select("MAX(ticket_no)").
		Scan(&ticketNo).Error
	if err != nil {
		return "", err
	}
	return ticketNo, nil
}
