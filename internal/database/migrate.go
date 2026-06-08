package database

import (
	"strings"

	"kafka-management-platform/internal/models"

	"gorm.io/gorm"
)

// AutoMigrate 自动迁移数据库表
func AutoMigrate(db *gorm.DB) error {
	// 按依赖顺序迁移表
	// 注意：GORM AutoMigrate 在索引变更时可能会报错，但不影响表结构
	err := db.AutoMigrate(
		&models.User{},
		&models.Cluster{},
		&models.ClusterUserRelation{},
		&models.Topic{},
		&models.ACL{},
		&models.AuditLog{},
		&models.ScramUser{},
		&models.HostMapping{},
	)

	if err != nil {
		// 检查是否是索引相关的错误，如果是则忽略
		errStr := err.Error()
		if isIndexRelatedError(errStr) {
			return nil
		}
		return err
	}

	return nil
}

// isIndexRelatedError 判断是否是索引相关的可忽略错误
func isIndexRelatedError(errStr string) bool {
	// MySQL 索引不存在错误: "Can't DROP 'xxx'; check that column/key exists"
	// MySQL 索引已存在错误: "Duplicate key name 'xxx'"
	return strings.Contains(errStr, "Can't DROP") ||
		strings.Contains(errStr, "Duplicate key name") ||
		strings.Contains(errStr, "check that column/key exists")
}
