package models

import (
	"time"
)

// TicketType 工单类型
type TicketType string

const (
	// TicketTypeTopicCreate 申请创建 Topic
	TicketTypeTopicCreate TicketType = "topic_create"
	// TicketTypePermissionGrant 申请权限（新建 SCRAM 用户并授权 / 已有用户增加权限）
	TicketTypePermissionGrant TicketType = "permission_grant"
)

// TicketStatus 工单状态
type TicketStatus string

const (
	// TicketStatusPending 待审批
	TicketStatusPending TicketStatus = "pending"
	// TicketStatusExecuting 审批通过，执行中
	TicketStatusExecuting TicketStatus = "executing"
	// TicketStatusSucceeded 执行完成
	TicketStatusSucceeded TicketStatus = "succeeded"
	// TicketStatusFailed 执行失败（可重试）
	TicketStatusFailed TicketStatus = "failed"
	// TicketStatusRejected 已驳回
	TicketStatusRejected TicketStatus = "rejected"
	// TicketStatusCancelled 已撤回（仅待审批状态、申请人本人）
	TicketStatusCancelled TicketStatus = "cancelled"
)

// Ticket 工单模型
type Ticket struct {
	TicketID     int64       `gorm:"column:ticket_id;primaryKey;autoIncrement" json:"ticket_id"`
	TicketNo     string      `gorm:"column:ticket_no;type:varchar(32);not null;uniqueIndex" json:"ticket_no"`
	Type         TicketType  `gorm:"column:type;type:varchar(32);not null;index" json:"type"`
	ClusterID    int64       `gorm:"column:cluster_id;not null;index" json:"cluster_id"`
	ApplicantID  int64       `gorm:"column:applicant_id;not null;index" json:"applicant_id"`
	Title        string      `gorm:"column:title;type:varchar(128);not null" json:"title"`
	// Form / Result 存 JSON 文本；用 text 而非 json 列，避免空值插入被数据库拒绝
	Form   string `gorm:"column:form;type:text;not null" json:"form"`
	Status TicketStatus `gorm:"column:status;type:varchar(32);not null;default:pending;index" json:"status"`
	ApproverID   *int64      `gorm:"column:approver_id" json:"approver_id"`
	ApproveRemark string     `gorm:"column:approve_remark;type:text" json:"approve_remark"`
	// Result 执行结果摘要 JSON（跳过项、创建明细等），不含凭证
	Result string `gorm:"column:result;type:text" json:"result"`
	// Credential 加密存储的新建 SCRAM 用户凭证（一次性查看后清空）
	Credential       string     `gorm:"column:credential;type:text" json:"-"`
	CredentialViewedAt *time.Time `gorm:"column:credential_viewed_at" json:"credential_viewed_at"`
	ErrorMsg         string     `gorm:"column:error_msg;type:text" json:"error_msg"`
	AppliedAt        time.Time  `gorm:"column:applied_at;not null;autoCreateTime" json:"applied_at"`
	ApprovedAt       *time.Time `gorm:"column:approved_at" json:"approved_at"`
	CompletedAt      *time.Time `gorm:"column:completed_at" json:"completed_at"`
	CreatedAt        time.Time  `gorm:"column:created_at;not null;autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;not null;autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名
func (Ticket) TableName() string {
	return "ticket"
}
