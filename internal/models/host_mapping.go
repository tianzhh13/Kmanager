package models

import (
	"time"
)

// HostMapping 主机名映射（hostname → IP）
// 用于 Kafka 集群 hostname 解析，避免依赖 /etc/hosts
type HostMapping struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Hostname    string    `gorm:"column:hostname;type:varchar(255);not null;uniqueIndex" json:"hostname"`
	IPAddress   string    `gorm:"column:ip_address;type:varchar(45);not null" json:"ip_address"`
	Description string    `gorm:"column:description;type:varchar(500)" json:"description"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null;autoUpdateTime" json:"updated_at"`
}

func (HostMapping) TableName() string {
	return "host_mappings"
}
