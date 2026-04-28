package models

import (
	"time"
)

// AuthType 认证类型
type AuthType string

const (
	AuthTypeNone      AuthType = "none"      // 无认证
	AuthTypePlaintext AuthType = "plaintext" // 明文传输
	AuthTypeSCRAM     AuthType = "scram"     // SCRAM 认证
	AuthTypeKerberos  AuthType = "kerberos"  // Kerberos 认证
)

// ClusterStatus 集群状态
type ClusterStatus string

const (
	ClusterStatusActive      ClusterStatus = "active"
	ClusterStatusInactive    ClusterStatus = "inactive"
	ClusterStatusUnreachable ClusterStatus = "unreachable"
)

// Cluster 集群模型
type Cluster struct {
	ClusterID        int64         `gorm:"column:cluster_id;primaryKey;autoIncrement" json:"cluster_id"`
	ClusterName      string        `gorm:"column:cluster_name;type:varchar(128);not null;index:idx_cluster_name" json:"cluster_name"`
	BootstrapServers string        `gorm:"column:bootstrap_servers;type:text;not null" json:"bootstrap_servers"`
	AuthType         AuthType      `gorm:"column:auth_type;type:varchar(32);not null;default:none" json:"auth_type"`
	SASLMechanism    string        `gorm:"column:sasl_mechanism;type:varchar(32)" json:"sasl_mechanism"`      // SASL 机制：PLAIN/SCRAM-SHA-256/SCRAM-SHA-512，用于前端过滤
	AuthConfig       string        `gorm:"column:auth_config;type:text" json:"-"`                             // 加密存储的认证配置（JSON）
	JMXExporterURL   string        `gorm:"column:jmx_exporter_url;type:varchar(256)" json:"jmx_exporter_url"` // JMX Exporter HTTP 地址（如 http://broker1:7071）
	Status           ClusterStatus `gorm:"column:status;type:varchar(32);not null;default:active;index:idx_status" json:"status"`
	Description      string        `gorm:"column:description;type:text" json:"description"`
	CreatedBy        int64         `gorm:"column:created_by;not null" json:"created_by"`
	CreatedAt        time.Time     `gorm:"column:created_at;not null;autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time     `gorm:"column:updated_at;not null;autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名
func (Cluster) TableName() string {
	return "cluster"
}
