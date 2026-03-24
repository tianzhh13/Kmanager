package models

import (
	"time"
)

// UserRole 用户角色
type UserRole string

const (
	RoleSuperAdmin   UserRole = "super_admin"
	RoleClusterAdmin UserRole = "cluster_admin"
	RoleReadOnly     UserRole = "read_only"
)

// UserStatus 用户状态
type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusInactive UserStatus = "inactive"
)

// User 用户模型
type User struct {
	UserID       int64      `gorm:"column:user_id;primaryKey;autoIncrement" json:"user_id"`
	Username     string     `gorm:"column:username;type:varchar(64);not null;uniqueIndex:idx_username" json:"username"`
	PasswordHash string     `gorm:"column:password_hash;type:varchar(128);not null" json:"-"`
	Email        string     `gorm:"column:email;type:varchar(128)" json:"email"`
	Role         UserRole   `gorm:"column:role;type:varchar(32);not null;index:idx_role" json:"role"`
	Status       UserStatus `gorm:"column:status;type:varchar(32);not null;default:active" json:"status"`
	CreatedAt    time.Time  `gorm:"column:created_at;not null;autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at;not null;autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名
func (User) TableName() string {
	return "user"
}
