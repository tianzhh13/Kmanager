package router

import (
	"kafka-management-platform/internal/config"
	"kafka-management-platform/internal/handler"
	"kafka-management-platform/internal/middleware"
	"kafka-management-platform/internal/repository"
	"kafka-management-platform/internal/service/acl"
	"kafka-management-platform/internal/service/audit"
	"kafka-management-platform/internal/service/auth"
	"kafka-management-platform/internal/service/cluster"
	"kafka-management-platform/internal/service/monitor"
	"kafka-management-platform/internal/service/topic"
	"kafka-management-platform/internal/service/user"
	"kafka-management-platform/pkg/encryption"
	"kafka-management-platform/pkg/jwt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Setup 设置路由
func Setup(cfg *config.Config, db *gorm.DB) *gin.Engine {
	// 设置 Gin 模式
	gin.SetMode(cfg.Server.Mode)

	r := gin.New()

	// 全局中间件
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.SecurityHeadersMiddleware())
	r.Use(middleware.RequestBodySizeLimitMiddleware(10 * 1024 * 1024)) // 10MB
	r.Use(middleware.RequestIDMiddleware())
	r.Use(middleware.RateLimitMiddleware())

	// 初始化服务
	jwtSvc := jwt.NewService(
		cfg.JWT.Secret,
		cfg.JWT.Issuer,
		cfg.JWT.AccessTokenExpire,
		cfg.JWT.RefreshTokenExpire,
	)

	encryptionSvc, err := encryption.NewService(cfg.Encryption.Key)
	if err != nil {
		panic("failed to initialize encryption service: " + err.Error())
	}

	// 初始化 Repository
	userRepo := repository.NewUserRepository(db)
	clusterRepo := repository.NewClusterRepository(db)
	clusterUserRepo := repository.NewClusterUserRepository(db)
	topicRepo := repository.NewTopicRepository(db)
	aclRepo := repository.NewACLRepository(db)
	auditLogRepo := repository.NewAuditLogRepository(db)

	// 初始化 Service
	authSvc := auth.NewService(userRepo, jwtSvc)
	permissionSvc := auth.NewPermissionService(userRepo, clusterUserRepo)
	clusterSvc := cluster.NewService(clusterRepo, clusterUserRepo, encryptionSvc)
	topicSvc := topic.NewService(topicRepo, clusterRepo, encryptionSvc)
	aclSvc := acl.NewService(aclRepo, clusterRepo, encryptionSvc)
	auditSvc := audit.NewService(auditLogRepo)
	monitorSvc := monitor.NewService(clusterRepo)
	userSvc := user.NewService(userRepo)

	// 初始化 Handler
	authHandler := handler.NewAuthHandler(authSvc)
	clusterHandler := handler.NewClusterHandler(clusterSvc)
	topicHandler := handler.NewTopicHandler(topicSvc)
	aclHandler := handler.NewACLHandler(aclSvc)
	userHandler := handler.NewUserHandler(userSvc)
	auditLogHandler := audit.NewHandler(auditSvc)
	monitorHandler := monitor.NewHandler(monitorSvc)

	// 初始化中间件
	permissionMiddleware := middleware.NewPermissionMiddleware(permissionSvc)
	clusterPermissionMiddleware := middleware.NewClusterPermissionMiddleware(permissionSvc)
	_ = middleware.NewAuditMiddleware(auditSvc) // 审计中间件（当前未使用）

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "Kafka Management Platform is running",
		})
	})

	// API v1 路由组
	v1 := r.Group("/api/v1")
	{
		// 认证路由（无需认证，但需要限流）
		authGroup := v1.Group("/auth")
		authGroup.Use(middleware.IPRateLimitMiddleware(20)) // 登录接口每 IP 每分钟 20 次
		{
			authGroup.POST("/login", authHandler.Login)
			authGroup.POST("/refresh", authHandler.RefreshToken)
		}

		// 需要认证的路由
		authenticated := v1.Group("")
		authenticated.Use(middleware.AuthMiddleware(jwtSvc))
		{
			// 当前用户信息
			authenticated.GET("/auth/me", authHandler.GetCurrentUser)

			// 用户路由 - 需要超级管理员权限
			users := authenticated.Group("/users")
			users.Use(permissionMiddleware.RequireSuperAdmin())
			{
				users.GET("", userHandler.ListUsers)
				users.POST("", userHandler.CreateUser)
				users.GET("/:id", userHandler.GetUser)
				users.PUT("/:id", userHandler.UpdateUser)
				users.DELETE("/:id", userHandler.DeleteUser)
				users.PUT("/:id/password", userHandler.UpdatePassword)
				users.POST("/:id/disable", userHandler.DisableUser)
				users.POST("/:id/enable", userHandler.EnableUser)
			}

			// 集群路由
			clusters := authenticated.Group("/clusters")
			{
				clusters.GET("", clusterHandler.ListClusters)
				clusters.POST("", permissionMiddleware.RequireSuperAdmin(), clusterHandler.CreateCluster)
				clusters.GET("/:id", clusterPermissionMiddleware.RequireClusterAccess(), clusterHandler.GetCluster)
				clusters.PUT("/:id", permissionMiddleware.RequireSuperAdmin(), clusterHandler.UpdateCluster)
				clusters.DELETE("/:id", permissionMiddleware.RequireSuperAdmin(), clusterHandler.DeleteCluster)
				clusters.POST("/:id/test", clusterPermissionMiddleware.RequireClusterAccess(), clusterHandler.TestConnection)
				clusters.POST("/:id/grant", permissionMiddleware.RequireSuperAdmin(), clusterHandler.GrantAccess)
				clusters.POST("/:id/revoke", permissionMiddleware.RequireSuperAdmin(), clusterHandler.RevokeAccess)
				clusters.GET("/:id/users", permissionMiddleware.RequireSuperAdmin(), clusterHandler.ListClusterUsers)
			}

			// Topic 路由
			topics := authenticated.Group("/topics")
			{
				topics.GET("", clusterPermissionMiddleware.RequireClusterAccess(), topicHandler.ListTopics)
				topics.POST("", clusterPermissionMiddleware.RequireClusterWriteAccess(), topicHandler.CreateTopic)
				topics.GET("/:name", clusterPermissionMiddleware.RequireClusterAccess(), topicHandler.GetTopic)
				topics.DELETE("/:name", clusterPermissionMiddleware.RequireClusterWriteAccess(), topicHandler.DeleteTopic)
				topics.PUT("/:name/config", clusterPermissionMiddleware.RequireClusterWriteAccess(), topicHandler.UpdateTopicConfig)
				topics.POST("/sync/:id", clusterPermissionMiddleware.RequireClusterWriteAccess(), topicHandler.SyncTopics)
			}

			// ACL 路由
			acls := authenticated.Group("/acls")
			{
				acls.GET("", clusterPermissionMiddleware.RequireClusterAccess(), aclHandler.ListACLs)
				acls.POST("", clusterPermissionMiddleware.RequireClusterWriteAccess(), aclHandler.CreateACL)
				acls.DELETE("/:id", clusterPermissionMiddleware.RequireClusterWriteAccess(), aclHandler.DeleteACL)
				acls.POST("/batch-delete", clusterPermissionMiddleware.RequireClusterWriteAccess(), aclHandler.BatchDeleteACL)
				acls.POST("/sync/:id", clusterPermissionMiddleware.RequireClusterWriteAccess(), aclHandler.SyncACLs)
			}

			// 监控路由
			metrics := authenticated.Group("/metrics")
			{
				// 集群级别指标
				metrics.GET("/cluster/:id", clusterPermissionMiddleware.RequireClusterAccess(), monitorHandler.GetClusterMetrics)
				// Broker 级别指标
				metrics.GET("/broker/:id", clusterPermissionMiddleware.RequireClusterAccess(), monitorHandler.GetBrokerMetrics)
				// Topic 级别指标
				metrics.GET("/topic/:id", clusterPermissionMiddleware.RequireClusterAccess(), monitorHandler.GetTopicMetrics)
				// 消费组指标
				metrics.GET("/consumer-group/:id", clusterPermissionMiddleware.RequireClusterAccess(), monitorHandler.GetConsumerGroupMetrics)
				// 自定义 PromQL 查询
				metrics.GET("/query/:id", clusterPermissionMiddleware.RequireClusterAccess(), monitorHandler.QueryPrometheus)
			}

			// 审计日志路由
			auditLogs := authenticated.Group("/audit-logs")
			{
				auditLogs.GET("", auditLogHandler.ListAuditLogs)
				auditLogs.GET("/export", auditLogHandler.CleanLogs)  // 暂用 CleanLogs 替代导出
				auditLogs.GET("/:id", auditLogHandler.ListAuditLogs) // 暂用列表替代详情
			}
		}
	}

	return r
}
