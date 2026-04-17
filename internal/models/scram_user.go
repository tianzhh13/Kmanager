package models

import "time"

// ScramUser SCRAM 用户模型
type ScramUser struct {
	UserID     int64      `gorm:"primaryKey;autoIncrement" json:"user_id"`
	ClusterID  int64      `gorm:"not null;index" json:"cluster_id"`
	Username   string     `gorm:"size:256;not null" json:"username"`
	Mechanism  string     `gorm:"size:32;not null;default:'SCRAM-SHA-256'" json:"mechanism"` // SCRAM-SHA-256 或 SCRAM-SHA-512
	SyncStatus string     `gorm:"size:32;default:'synced'" json:"sync_status"`
	LastSyncAt *time.Time `json:"last_sync_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// TableName 指定表名
func (ScramUser) TableName() string {
	return "scram_users"
}
