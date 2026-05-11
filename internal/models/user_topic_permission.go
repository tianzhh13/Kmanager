package models

import (
	"time"
)

// UserTopicPermission 用户 Topic 权限模型
type UserTopicPermission struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID    int64     `gorm:"column:user_id;not null;uniqueIndex:uk_user_cluster_topic" json:"user_id"`
	ClusterID int64     `gorm:"column:cluster_id;not null;uniqueIndex:uk_user_cluster_topic" json:"cluster_id"`
	TopicName string    `gorm:"column:topic_name;type:varchar(255);not null;uniqueIndex:uk_user_cluster_topic" json:"topic_name"`
	CreatedAt time.Time `gorm:"column:created_at;not null;autoCreateTime" json:"created_at"`
	CreatedBy int64     `gorm:"column:created_by;not null" json:"created_by"`
}

// TableName 指定表名
func (UserTopicPermission) TableName() string {
	return "user_topic_permission"
}
