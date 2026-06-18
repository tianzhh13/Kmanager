package models

import (
	"time"
)

// ResourceType ACL 资源类型
type ResourceType string

const (
	ResourceTypeTopic   ResourceType = "topic"
	ResourceTypeGroup   ResourceType = "group"
	ResourceTypeCluster ResourceType = "cluster"
)

// PatternType ACL 模式类型
type PatternType string

const (
	PatternTypeLiteral  PatternType = "literal"
	PatternTypePrefixed PatternType = "prefixed"
)

// OperationType ACL 操作类型
type OperationType string

const (
	OperationRead             OperationType = "read"
	OperationWrite            OperationType = "write"
	OperationCreate           OperationType = "create"
	OperationDelete           OperationType = "delete"
	OperationAlter            OperationType = "alter"
	OperationDescribe         OperationType = "describe"
	OperationAll              OperationType = "all"
	OperationDescribeConfigs  OperationType = "describeconfigs"
	OperationAlterConfigs     OperationType = "alterconfigs"
	OperationClusterAction    OperationType = "clusteraction"
	OperationIdempotentWrite  OperationType = "idempotentwrite"
)

// PermissionType ACL 权限类型
type PermissionType string

const (
	PermissionTypeAllow PermissionType = "allow"
	PermissionTypeDeny  PermissionType = "deny"
)

// ACL ACL 模型
type ACL struct {
	ACLID           int64          `gorm:"column:acl_id;primaryKey;autoIncrement" json:"acl_id"`
	ClusterID       int64          `gorm:"column:cluster_id;not null;index:idx_cluster_resource" json:"cluster_id"`
	ResourceType    ResourceType   `gorm:"column:resource_type;type:varchar(32);not null;index:idx_cluster_resource" json:"resource_type"`
	ResourceName    string         `gorm:"column:resource_name;type:varchar(256);not null;index:idx_cluster_resource" json:"resource_name"`
	ResourcePattern PatternType    `gorm:"column:resource_pattern;type:varchar(32);not null" json:"resource_pattern"`
	Principal       string         `gorm:"column:principal;type:varchar(256);not null;index:idx_principal" json:"principal"`
	Host            string         `gorm:"column:host;type:varchar(128);not null;default:*" json:"host"`
	Operation       OperationType  `gorm:"column:operation;type:varchar(32);not null" json:"operation"`
	PermissionType  PermissionType `gorm:"column:permission_type;type:varchar(32);not null" json:"permission_type"`
	SyncStatus      SyncStatus     `gorm:"column:sync_status;type:varchar(32);not null;default:synced;index:idx_sync_status" json:"sync_status"`
	LastSyncAt      *time.Time     `gorm:"column:last_sync_at" json:"last_sync_at"`
	CreatedAt       time.Time      `gorm:"column:created_at;not null;autoCreateTime" json:"created_at"`
}

// TableName 指定表名
func (ACL) TableName() string {
	return "acl"
}
