package database

import (
	"kafka-management-platform/internal/config"

	"gorm.io/gorm"
)

// Init 初始化数据库连接
func Init(cfg *config.Config) (*gorm.DB, error) {
	return initDatabase(cfg)
}
