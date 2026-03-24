package database

import (
	"kafka-management-platform/internal/models"

	"gorm.io/gorm"
)

// AutoMigrate 自动迁移数据库表
func AutoMigrate(db *gorm.DB) error {
	// 按依赖顺序迁移表
	return db.AutoMigrate(
		&models.User{},
		&models.Cluster{},
		&models.ClusterUserRelation{},
		&models.Topic{},
		&models.ACL{},
		&models.AuditLog{},
	)
}
