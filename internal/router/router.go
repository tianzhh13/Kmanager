package router

import (
	"kafka-management-platform/internal/config"
	"kafka-management-platform/internal/handler"
	"kafka-management-platform/internal/middleware"
	"kafka-management-platform/internal/repository"
	"kafka-management-platform/internal/service/acl"
	"kafka-management-platform/internal/service/auth"
	"kafka-management-platform/internal/service/cluster"
	"kafka-management-platform/internal/service/topic"
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

	// 初始化 Service
	authSvc := auth.NewService(userRepo, jwtSvc)
	clusterSvc := cluster.NewService(clusterRepo, clusterUserRepo, encryptionSvc)
	topicSvc := topic.NewService(topicRepo, clusterRepo, encryptionSvc)
	aclSvc := acl.NewService(aclRepo, clusterRepo, encryptionSvc)

	// 初始化 Handler
	authHandler := handler.NewAuthHandler(authSvc)
	clusterHandler := handler.NewClusterHandler(clusterSvc)
	topicHandler := handler.NewTopicHandler(topicSvc)
	aclHandler := handler.NewACLHandler(aclSvc)

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
		// 认证路由（无需认证）
		authGroup := v1.Group("/auth")
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

			// 集群路由
			clusters := authenticated.Group("/clusters")
			{
				clusters.GET("", clusterHandler.ListClusters)
				clusters.POST("", clusterHandler.CreateCluster)
				clusters.GET("/:id", clusterHandler.GetCluster)
				clusters.PUT("/:id", clusterHandler.UpdateCluster)
				clusters.DELETE("/:id", clusterHandler.DeleteCluster)
				clusters.POST("/:id/test", clusterHandler.TestConnection)
				clusters.POST("/:id/grant", clusterHandler.GrantAccess)
				clusters.POST("/:id/revoke", clusterHandler.RevokeAccess)
				clusters.GET("/:id/users", clusterHandler.ListClusterUsers)
			}

			// Topic 路由
			topics := authenticated.Group("/topics")
			{
				topics.GET("", topicHandler.ListTopics)
				topics.POST("", topicHandler.CreateTopic)
				topics.GET("/:name", topicHandler.GetTopic)
				topics.DELETE("/:name", topicHandler.DeleteTopic)
				topics.PUT("/:name/config", topicHandler.UpdateTopicConfig)
				topics.POST("/sync/:id", topicHandler.SyncTopics)
			}

			// ACL 路由
			acls := authenticated.Group("/acls")
			{
				acls.GET("", aclHandler.ListACLs)
				acls.POST("", aclHandler.CreateACL)
				acls.DELETE("/:id", aclHandler.DeleteACL)
				acls.POST("/batch-delete", aclHandler.BatchDeleteACL)
				acls.POST("/sync/:id", aclHandler.SyncACLs)
			}

			// 监控路由（待实现）
			metrics := authenticated.Group("/metrics")
			{
				metrics.GET("/cluster/:id", func(c *gin.Context) {
					c.JSON(200, gin.H{"message": "cluster metrics - to be implemented"})
				})
			}

			// 审计日志路由（待实现）
			auditLogs := authenticated.Group("/audit-logs")
			{
				auditLogs.GET("", func(c *gin.Context) {
					c.JSON(200, gin.H{"message": "list audit logs - to be implemented"})
				})
			}
		}
	}

	return r
}
