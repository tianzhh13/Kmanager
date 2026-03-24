package models

import (
	"time"
)

// AuditStatus 审计日志状态
type AuditStatus string

const (
	AuditStatusSuccess AuditStatus = "success"
	AuditStatusFailed  AuditStatus = "failed"
)

// AuditLog 审计日志模型
type AuditLog struct {
	LogID      int64       `gorm:"column:log_id;primaryKey;autoIncrement" json:"log_id"`
	UserID     int64       `gorm:"column:user_id;not null;index:idx_user_id" json:"user_id"`
	Username   string      `gorm:"column:username;type:varchar(64);not null" json:"username"`
	Action     string      `gorm:"column:action;type:varchar(64);not null;index:idx_action" json:"action"`
	Resource   string      `gorm:"column:resource;type:varchar(64);not null;index:idx_resource" json:"resource"`
	ResourceID string      `gorm:"column:resource_id;type:varchar(256)" json:"resource_id"`
	ClusterID  *int64      `gorm:"column:cluster_id;index:idx_cluster_id" json:"cluster_id"`
	Details    string      `gorm:"column:details;type:text" json:"details"`
	IPAddress  string      `gorm:"column:ip_address;type:varchar(64)" json:"ip_address"`
	UserAgent  string      `gorm:"column:user_agent;type:varchar(256)" json:"user_agent"`
	Status     AuditStatus `gorm:"column:status;type:varchar(32);not null;index:idx_status" json:"status"`
	ErrorMsg   string      `gorm:"column:error_msg;type:text" json:"error_msg"`
	CreatedAt  time.Time   `gorm:"column:created_at;not null;autoCreateTime;index:idx_created_at" json:"created_at"`
}

// TableName 指定表名
func (AuditLog) TableName() string {
	return "audit_log"
}
