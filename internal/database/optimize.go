package database

import (
	"fmt"
	"time"

	"kafka-management-platform/internal/logger"

	"gorm.io/gorm"
)

// OptimizeDatabase 优化数据库性能
func OptimizeDatabase(db *gorm.DB) error {
	// 设置连接池参数
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}

	// 配置连接池
	sqlDB.SetMaxOpenConns(50)               // 最大打开连接数
	sqlDB.SetMaxIdleConns(10)               // 最大空闲连接数
	sqlDB.SetConnMaxLifetime(1 * time.Hour) // 连接最大生命周期

	logger.Info("Database connection pool optimized",
		"max_open_conns", 50,
		"max_idle_conns", 10,
		"conn_max_lifetime", "1h",
	)

	return nil
}

// AddIndexes 添加数据库索引
func AddIndexes(db *gorm.DB) error {
	indexes := []struct {
		table  string
		name   string
		fields string
	}{
		// user 表索引
		{"user", "idx_users_username", "username"},
		{"user", "idx_users_email", "email"},
		{"user", "idx_users_status", "status"},

		// cluster 表索引
		{"cluster", "idx_clusters_name", "name"},
		{"cluster", "idx_clusters_status", "status"},

		// cluster_user_relation 表索引
		{"cluster_user_relation", "idx_cur_user_id", "user_id"},
		{"cluster_user_relation", "idx_cur_cluster_id", "cluster_id"},

		// topic 表索引
		{"topic", "idx_topics_cluster_id", "cluster_id"},
		{"topic", "idx_topics_name", "name"},
		{"topic", "idx_topics_cluster_name", "cluster_id, name"},

		// acl 表索引
		{"acl", "idx_acls_cluster_id", "cluster_id"},
		{"acl", "idx_acls_principal", "principal"},
		{"acl", "idx_acls_resource_type_name", "resource_type, resource_name"},

		// audit_log 表索引
		{"audit_log", "idx_audit_user_id", "user_id"},
		{"audit_log", "idx_audit_action", "action"},
		{"audit_log", "idx_audit_resource_type", "resource_type"},
		{"audit_log", "idx_audit_created_at", "created_at"},
	}

	for _, idx := range indexes {
		query := fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s (%s)", idx.name, idx.table, idx.fields)
		if err := db.Exec(query).Error; err != nil {
			logger.Warn("Failed to create index",
				"index", idx.name,
				"error", err.Error(),
			)
			// 继续创建其他索引，不中断
		} else {
			logger.Info("Index created",
				"index", idx.name,
				"table", idx.table,
			)
		}
	}

	return nil
}

// SetupAutoMigrate 自动迁移并优化
func SetupAutoMigrate(db *gorm.DB, models ...interface{}) error {
	// 自动迁移
	if err := db.AutoMigrate(models...); err != nil {
		return fmt.Errorf("failed to auto migrate: %w", err)
	}

	// 优化数据库
	if err := OptimizeDatabase(db); err != nil {
		return fmt.Errorf("failed to optimize database: %w", err)
	}

	// 添加索引
	if err := AddIndexes(db); err != nil {
		logger.Warn("Failed to add indexes", "error", err.Error())
		// 索引创建失败不影响启动
	}

	logger.Info("Database setup completed")
	return nil
}
