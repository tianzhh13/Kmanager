package models

import (
	"time"
)

// ClusterUserRelation 集群用户关联模型
type ClusterUserRelation struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ClusterID int64     `gorm:"column:cluster_id;not null;uniqueIndex:uk_cluster_user" json:"cluster_id"`
	UserID    int64     `gorm:"column:user_id;not null;uniqueIndex:uk_cluster_user;index:idx_user_id" json:"user_id"`
	CreatedAt time.Time `gorm:"column:created_at;not null;autoCreateTime" json:"created_at"`
}

// TableName 指定表名
func (ClusterUserRelation) TableName() string {
	return "cluster_user_relation"
}
