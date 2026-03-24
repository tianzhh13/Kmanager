package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// SyncStatus 同步状态
type SyncStatus string

const (
	SyncStatusSynced   SyncStatus = "synced"
	SyncStatusPending  SyncStatus = "pending"
	SyncStatusConflict SyncStatus = "conflict"
)

// TopicConfig Topic 配置（JSON 存储）
type TopicConfig map[string]string

// Scan 实现 sql.Scanner 接口
func (tc *TopicConfig) Scan(value interface{}) error {
	if value == nil {
		*tc = make(TopicConfig)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, tc)
}

// Value 实现 driver.Valuer 接口
func (tc TopicConfig) Value() (driver.Value, error) {
	if tc == nil {
		return nil, nil
	}
	return json.Marshal(tc)
}

// Topic Topic 模型
type Topic struct {
	TopicID           int64       `gorm:"column:topic_id;primaryKey;autoIncrement" json:"topic_id"`
	ClusterID         int64       `gorm:"column:cluster_id;not null;uniqueIndex:uk_cluster_topic" json:"cluster_id"`
	TopicName         string      `gorm:"column:topic_name;type:varchar(256);not null;uniqueIndex:uk_cluster_topic;index:idx_topic_name" json:"topic_name"`
	Partitions        int32       `gorm:"column:partitions;not null" json:"partitions"`
	ReplicationFactor int16       `gorm:"column:replication_factor;not null" json:"replication_factor"`
	Config            TopicConfig `gorm:"column:config;type:json" json:"config"`
	SyncStatus        SyncStatus  `gorm:"column:sync_status;type:varchar(32);not null;default:synced;index:idx_sync_status" json:"sync_status"`
	LastSyncAt        *time.Time  `gorm:"column:last_sync_at" json:"last_sync_at"`
	CreatedAt         time.Time   `gorm:"column:created_at;not null;autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time   `gorm:"column:updated_at;not null;autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名
func (Topic) TableName() string {
	return "topic"
}
