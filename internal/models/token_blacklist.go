package models

import (
	"time"
)

// TokenBlacklist Token 黑名单模型（持久化）
type TokenBlacklist struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TokenHash string    `gorm:"column:token_hash;type:varchar(64);not null;uniqueIndex:uk_token_hash" json:"-"`
	ExpiresAt time.Time `gorm:"column:expires_at;not null;index:idx_expires_at" json:"expires_at"`
	CreatedAt time.Time `gorm:"column:created_at;not null;autoCreateTime" json:"created_at"`
}

// TableName 指定表名
func (TokenBlacklist) TableName() string {
	return "token_blacklist"
}
