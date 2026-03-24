package router

import (
	"kafka-management-platform/internal/config"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Setup 设置路由
func Setup(cfg *config.Config, db *gorm.DB) *gin.Engine {
	// 设置 Gin 模式
	gin.SetMode(cfg.Server.Mode)

	r := gin.New()

	// 中间件
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
			"message": "Kafka Management Platform is running",
		})
	})

	// API v1 路由组
	v1 := r.Group("/api/v1")
	{
		// 认证路由（后续实现）
		auth := v1.Group("/auth")
		{
			auth.POST("/login", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "login endpoint - to be implemented"})
			})
			auth.POST("/refresh", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "refresh endpoint - to be implemented"})
			})
		}

		// 用户路由（后续实现）
		users := v1.Group("/users")
		{
			users.GET("", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "list users - to be implemented"})
			})
		}

		// 集群路由（后续实现）
		clusters := v1.Group("/clusters")
		{
			clusters.GET("", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "list clusters - to be implemented"})
			})
		}

		// Topic 路由（后续实现）
		topics := v1.Group("/topics")
		{
			topics.GET("", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "list topics - to be implemented"})
			})
		}

		// ACL 路由（后续实现）
		acls := v1.Group("/acls")
		{
			acls.GET("", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "list acls - to be implemented"})
			})
		}

		// 监控路由（后续实现）
		metrics := v1.Group("/metrics")
		{
			metrics.GET("/cluster/:id", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "cluster metrics - to be implemented"})
			})
		}

		// 审计日志路由（后续实现）
		auditLogs := v1.Group("/audit-logs")
		{
			auditLogs.GET("", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "list audit logs - to be implemented"})
			})
		}
	}

	return r
}
