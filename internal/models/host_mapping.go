package models

import (
	"time"
)

// HostMapping 主机名映射（hostname → IP）
// 用于 Kafka 集群 hostname 解析，避免依赖 /etc/hosts
// cluster_name 为空时表示全局映射，非空时表示集群专属映射
type HostMapping struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Hostname    string    `gorm:"column:hostname;type:varchar(255);not null;uniqueIndex:idx_hostname_cluster" json:"hostname"`
	ClusterName string    `gorm:"column:cluster_name;type:varchar(128);not null;default:'';uniqueIndex:idx_hostname_cluster" json:"cluster_name"`
	IPAddress   string    `gorm:"column:ip_address;type:varchar(45);not null" json:"ip_address"`
	Description string    `gorm:"column:description;type:varchar(500)" json:"description"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null;autoUpdateTime" json:"updated_at"`
}

func (HostMapping) TableName() string {
	return "host_mappings"
}
