package router

import (
	"context"
	"net/http"
	"path"
	"strings"
	"time"

	"kafka-management-platform/internal/cache"
	"kafka-management-platform/internal/config"
	"kafka-management-platform/internal/handler"
	"kafka-management-platform/internal/middleware"
	"kafka-management-platform/internal/repository"
	"kafka-management-platform/internal/service/acl"
	"kafka-management-platform/internal/service/audit"
	"kafka-management-platform/internal/service/auth"
	"kafka-management-platform/internal/service/cluster"
	"kafka-management-platform/internal/service/dashboard"
	"kafka-management-platform/internal/service/hostmapping"
	"kafka-management-platform/internal/service/message"
	"kafka-management-platform/internal/service/monitor"
	"kafka-management-platform/internal/service/scram"
	"kafka-management-platform/internal/service/ticket"
	"kafka-management-platform/internal/service/topic"
	"kafka-management-platform/internal/service/user"
	"kafka-management-platform/pkg/encryption"
	"kafka-management-platform/pkg/jwt"
	"kafka-management-platform/pkg/kerberos"
	"kafka-management-platform/pkg/victoriametrics"

	"github.com/gin-contrib/gzip"
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
	r.Use(middleware.ErrorHandlerMiddleware())
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.SecurityHeadersMiddleware())
	r.Use(middleware.HSTSMiddleware())
	r.Use(middleware.RefererPolicyMiddleware())
	r.Use(middleware.PermissionsPolicyMiddleware())
	r.Use(middleware.RequestBodySizeLimitMiddleware(10 * 1024 * 1024)) // 10MB
	r.Use(middleware.RequestIDMiddleware())
	r.Use(middleware.RateLimitMiddleware())
	r.Use(middleware.ResponseLoggerMiddleware())
	r.Use(gzip.Gzip(gzip.DefaultCompression))

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
	topicRepo := repository.NewTopicRepository(db)
	aclRepo := repository.NewACLRepository(db)
	auditLogRepo := repository.NewAuditLogRepository(db)
	clusterUserRepo := repository.NewClusterUserRepository(db)
	scramUserRepo := repository.NewScramUserRepository(db)
	topicPermRepo := repository.NewTopicPermissionRepository(db)
	ticketRepo := repository.NewTicketRepository(db)

	// 初始化 VictoriaMetrics 客户端
	vmClient := victoriametrics.NewClient(
		cfg.VictoriaMetrics.WriteURL,
		cfg.VictoriaMetrics.QueryURL,
		cfg.VictoriaMetrics.Enabled,
	)

	// 初始化 Kerberos Manager
	kerberosMgr := kerberos.NewManager("./kerberos")
	kerberosBaseDir := "./kerberos"

	// 初始化 Service
	authSvc := auth.NewService(userRepo, jwtSvc)
	permissionSvc := auth.NewPermissionService(userRepo, clusterUserRepo, topicPermRepo)
	clusterSvc := cluster.NewService(clusterRepo, clusterUserRepo, topicRepo, encryptionSvc, kerberosMgr)
	topicSvc := topic.NewService(topicRepo, clusterRepo, encryptionSvc, kerberosBaseDir)
	messageSvc := message.NewService(clusterRepo, encryptionSvc, permissionSvc, kerberosBaseDir)
	aclSvc := acl.NewService(aclRepo, clusterRepo, encryptionSvc, kerberosBaseDir)
	auditSvc := audit.NewService(auditLogRepo)
	monitorSvc := monitor.NewService(clusterRepo, encryptionSvc, vmClient, kerberosBaseDir)
	userSvc := user.NewService(userRepo)
	scramSvc := scram.NewService(scramUserRepo, clusterRepo, encryptionSvc, kerberosBaseDir)
	dashboardSvc := dashboard.NewService(clusterRepo, topicRepo, userRepo, monitorSvc)
	ticketSvc := ticket.NewService(ticketRepo, clusterRepo, scramUserRepo, clusterUserRepo, userRepo, permissionSvc, auditSvc, encryptionSvc, topicSvc, scramSvc, aclSvc)

	// 初始化 Token 黑名单缓存（内存 + 数据库双写）
	memoryCache := cache.NewMemoryCache(24 * time.Hour)
	tokenBlacklistCache := cache.NewTokenBlacklistCache(memoryCache)
	tokenBlacklistRepo := repository.NewTokenBlacklistRepository(db)
	tokenBlacklistCache.SetRepository(tokenBlacklistRepo)

	// 从数据库加载活跃黑名单到内存缓存
	tokenBlacklistCache.LoadFromDB(context.Background())

	// 初始化用户状态缓存（30 秒 TTL，减少认证中间件数据库查询）
	userStatusCache := cache.NewMemoryCache(30 * time.Second)

	// 初始化 Handler
	authHandler := handler.NewAuthHandler(authSvc, tokenBlacklistCache, auditSvc, &cfg.Cookie)
	clusterHandler := handler.NewClusterHandler(clusterSvc, permissionSvc, monitorSvc)
	topicHandler := handler.NewTopicHandler(topicSvc, permissionSvc)
	messageHandler := handler.NewMessageHandler(messageSvc)
	aclHandler := handler.NewACLHandler(aclSvc)
	userHandler := handler.NewUserHandler(userSvc)
	auditLogHandler := audit.NewHandler(auditSvc)
	monitorHandler := monitor.NewHandler(monitorSvc)
	scramUserHandler := handler.NewScramUserHandler(scramSvc)
	dashboardHandler := handler.NewDashboardHandler(dashboardSvc)
	ticketHandler := handler.NewTicketHandler(ticketSvc)

	// 初始化中间件
	permissionMiddleware := middleware.NewPermissionMiddleware(permissionSvc)
	clusterPermissionMiddleware := middleware.NewClusterPermissionMiddleware(permissionSvc)
	auditMiddleware := middleware.NewAuditMiddleware(auditSvc) // 审计中间件

	// webBase 是统一运维平台纳管要求的应用路径前缀。
	// 页面、API、静态资源全部挂在该前缀下，保证 http://{IP}:{PORT}/kmanager 可访问。
	const webBase = "/kmanager"

	// 根路径重定向到带前缀的入口，避免裸访问 404
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, webBase)
	})

	// 健康检查
	r.GET(webBase+"/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "Kafka Management Platform is running",
		})
	})

	// 系统配置（公开接口，无需认证）
	r.GET(webBase+"/api/v1/system/config", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"idle_timeout": cfg.Session.IdleTimeout, // 无操作自动登出时间（分钟）
		})
	})

	// API v1 路由组
	v1 := r.Group(webBase + "/api/v1")
	{
		// 认证路由（无需认证，但需要限流）
		authGroup := v1.Group("/auth")
		authGroup.Use(middleware.IPRateLimitMiddleware(20)) // 登录接口每 IP 每分钟 20 次
		{
			authGroup.POST("/login", authHandler.Login)
			authGroup.POST("/refresh", authHandler.RefreshToken)
		}

		// SSO 路由预留（501 Not Implemented）
		v1.GET("/auth/sso/:provider", func(c *gin.Context) {
			c.JSON(501, gin.H{"error": "SSO not implemented"})
		})
		v1.GET("/auth/sso/:provider/callback", func(c *gin.Context) {
			c.JSON(501, gin.H{"error": "SSO not implemented"})
		})

		// 需要认证的路由
		authenticated := v1.Group("")
		authenticated.Use(middleware.AuthMiddleware(jwtSvc, tokenBlacklistCache, userRepo, userStatusCache))
		authenticated.Use(auditMiddleware.Audit()) // 启用审计中间件
		{
			// 当前用户信息
			authenticated.GET("/auth/me", authHandler.GetCurrentUser)
			// 退出登录
			authenticated.POST("/auth/logout", authHandler.Logout)

			// Dashboard 路由
			authenticated.GET("/dashboard/overview", dashboardHandler.GetOverview)

			// 用户路由 - 需要超级管理员权限
			users := authenticated.Group("/users")
			users.Use(permissionMiddleware.RequireSuperAdmin())
			{
				users.GET("", userHandler.ListUsers)
				users.POST("", userHandler.CreateUser)
				users.GET("/stats", userHandler.GetStats)
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
				// 集群基础信息（所有登录用户可见，工单申请选择集群用），需在 /:id 之前注册
				clusters.GET("/options", clusterHandler.ListClusterOptions)
				clusters.GET("", clusterHandler.ListClusters)
				clusters.POST("", permissionMiddleware.RequireSuperAdmin(), clusterHandler.CreateCluster)
				clusters.GET("/:id", clusterPermissionMiddleware.RequireClusterAccess(), clusterHandler.GetCluster)
				clusters.PUT("/:id", permissionMiddleware.RequireSuperAdmin(), clusterHandler.UpdateCluster)
				clusters.DELETE("/:id", permissionMiddleware.RequireSuperAdmin(), clusterHandler.DeleteCluster)
				clusters.POST("/:id/test", clusterPermissionMiddleware.RequireClusterAccess(), clusterHandler.TestConnection)
				// 创建前测试连接（无需集群 ID）
				clusters.POST("/test-connection", permissionMiddleware.RequireSuperAdmin(), clusterHandler.TestConnectionForCreate)
				// Keytab 文件上传
				clusters.POST("/upload-keytab", permissionMiddleware.RequireSuperAdmin(), clusterHandler.UploadKeytab)
				clusters.DELETE("/upload-keytab", permissionMiddleware.RequireSuperAdmin(), clusterHandler.DeleteTempKeytab)
				clusters.POST("/:id/grant", middleware.RequireSuperAdminOrClusterAdmin(), clusterHandler.GrantAccess)
				clusters.POST("/:id/revoke", middleware.RequireSuperAdminOrClusterAdmin(), clusterHandler.RevokeAccess)
				clusters.GET("/:id/users", middleware.RequireSuperAdminOrClusterAdmin(), clusterHandler.ListClusterUsers)
				clusters.GET("/user/:userId", middleware.RequireSuperAdminOrClusterAdmin(), clusterHandler.ListUserClusters)
			}

			// Topic 路由
			topics := authenticated.Group("/topics")
			{
				topics.GET("", clusterPermissionMiddleware.RequireClusterAccess(), topicHandler.ListTopics)
				topics.POST("", clusterPermissionMiddleware.RequireClusterWriteAccess(), topicHandler.CreateTopic)
				topics.GET("/:name", clusterPermissionMiddleware.RequireClusterAccess(), topicHandler.GetTopic)
				topics.GET("/:name/config", clusterPermissionMiddleware.RequireClusterAccess(), topicHandler.GetTopicConfig)
				topics.GET("/:name/consumer-groups", clusterPermissionMiddleware.RequireClusterAccess(), topicHandler.GetTopicConsumerGroups)
				// 消息查询（只读排障：按 offset/时间戳/key 查消息并预览）
				topics.POST("/:name/messages", clusterPermissionMiddleware.RequireClusterAccess(), messageHandler.QueryMessages)
				topics.PUT("/:name/description", clusterPermissionMiddleware.RequireClusterWriteAccess(), topicHandler.UpdateTopicDescription)
				topics.DELETE("/:name", clusterPermissionMiddleware.RequireClusterWriteAccess(), topicHandler.DeleteTopic)
				topics.PUT("/:name/config", clusterPermissionMiddleware.RequireClusterWriteAccess(), topicHandler.UpdateTopicConfig)
				topics.POST("/sync/:id", clusterPermissionMiddleware.RequireClusterWriteAccess(), topicHandler.SyncTopics)
				// 批量名称预检：工单批量导入使用，提交工单仅要求登录，故不做集群授权校验
				topics.POST("/check-names", topicHandler.CheckTopicNames)
			}

			// ACL 路由
			acls := authenticated.Group("/acls")
			{
				acls.GET("", clusterPermissionMiddleware.RequireClusterAccess(), aclHandler.ListACLs)
				acls.GET("/user", clusterPermissionMiddleware.RequireClusterAccess(), aclHandler.ListUserACLsFromKafka)
				acls.POST("", clusterPermissionMiddleware.RequireClusterWriteAccess(), aclHandler.CreateACL)
				acls.DELETE("/:id", clusterPermissionMiddleware.RequireClusterWriteAccess(), aclHandler.DeleteACL)
				acls.DELETE("/kafka", clusterPermissionMiddleware.RequireClusterWriteAccess(), aclHandler.DeleteACLFromKafka)
				acls.POST("/batch-delete", clusterPermissionMiddleware.RequireClusterWriteAccess(), aclHandler.BatchDeleteACL)
				acls.POST("/sync/:id", clusterPermissionMiddleware.RequireClusterWriteAccess(), aclHandler.SyncACLs)
			}

			// SCRAM 用户路由
			scramUsers := authenticated.Group("/scram-users")
			{
				scramUsers.GET("", clusterPermissionMiddleware.RequireClusterAccess(), scramUserHandler.ListUsers)
				scramUsers.POST("", clusterPermissionMiddleware.RequireClusterWriteAccess(), scramUserHandler.CreateUser)
				scramUsers.DELETE("/:username", clusterPermissionMiddleware.RequireClusterWriteAccess(), scramUserHandler.DeleteUser)
				scramUsers.POST("/sync/:id", clusterPermissionMiddleware.RequireClusterWriteAccess(), scramUserHandler.SyncUsers)
			}

			// 工单路由（审批人权限校验在 service 内按工单所属集群完成）
			tickets := authenticated.Group("/tickets")
			{
				tickets.GET("", ticketHandler.List)
				tickets.GET("/stats", ticketHandler.Stats)
				tickets.POST("", ticketHandler.Submit)
				tickets.GET("/:id", ticketHandler.Get)
				tickets.POST("/:id/approve", ticketHandler.Approve)
				tickets.POST("/:id/reject", ticketHandler.Reject)
				tickets.POST("/:id/cancel", ticketHandler.Cancel)
				tickets.POST("/:id/retry", ticketHandler.Retry)
				tickets.POST("/:id/credentials", ticketHandler.TakeCredential)
			}

			// 监控路由
			metrics := authenticated.Group("/metrics")
			{
				// 集群级别指标（整合 JMX + Kafka Exporter）
				metrics.GET("/cluster/:id", clusterPermissionMiddleware.RequireClusterAccess(), monitorHandler.GetClusterMetrics)
				// Broker 级别指标（来自 JMX Exporter）
				metrics.GET("/broker/:id", clusterPermissionMiddleware.RequireClusterAccess(), monitorHandler.GetBrokerMetrics)
				// Broker 总览数据（来自 VictoriaMetrics）
				metrics.GET("/broker-overview/:id", clusterPermissionMiddleware.RequireClusterAccess(), monitorHandler.GetBrokerOverview)
				// 消费者组 Lag（来自内置 Kafka Exporter）
				metrics.GET("/consumer-groups/:id", clusterPermissionMiddleware.RequireClusterAccess(), monitorHandler.GetConsumerGroupLags)
				// 单个消费者组详情
				metrics.GET("/consumer-group/:id", clusterPermissionMiddleware.RequireClusterAccess(), monitorHandler.GetConsumerGroupInfo)
				// 历史指标（代理 VictoriaMetrics 查询，需集群权限）
				metrics.GET("/history", clusterPermissionMiddleware.RequireClusterAccess(), monitorHandler.GetMetricsHistory)
				// 批量查询指标（去重 + 缓存，减少 VM 请求）
				metrics.POST("/batch-query", clusterPermissionMiddleware.RequireClusterAccess(), monitorHandler.BatchQueryMetrics)
			}

			// 审计日志路由（仅管理员可见）
			auditLogs := authenticated.Group("/audit-logs")
			auditLogs.Use(middleware.RequireSuperAdminOrClusterAdmin())
			{
				auditLogs.GET("", auditLogHandler.ListAuditLogs)
				auditLogs.GET("/export", auditLogHandler.ExportLogs)
				auditLogs.GET("/:id", auditLogHandler.GetAuditLogDetail)
				auditLogs.DELETE("/clean", permissionMiddleware.RequireSuperAdmin(), auditLogHandler.CleanLogs)
			}

			// Topic 权限管理路由
			topicPermHandler := handler.NewTopicPermissionHandler(auth.NewTopicPermissionService(topicPermRepo, clusterUserRepo, userRepo, clusterRepo))
			topicPerms := authenticated.Group("/topic-permissions")
			topicPerms.Use(middleware.RequireSuperAdminOrClusterAdmin())
			{
				topicPerms.POST("", topicPermHandler.AssignTopicPermission)
				topicPerms.POST("/batch", topicPermHandler.BatchAssignTopicPermission)
				topicPerms.DELETE("", topicPermHandler.RevokeTopicPermission)
				topicPerms.GET("/user/:userId", topicPermHandler.GetUserTopicPermissions)
				topicPerms.GET("/user/:userId/cluster/:clusterId", topicPermHandler.GetUserClusterTopicPermissions)
			}

			// 主机映射管理路由
			hostMappingRepo := repository.NewHostMappingRepository(db)
			hostMappingSvc := hostmapping.NewService(hostMappingRepo)
			hostMappingHandler := handler.NewHostMappingHandler(hostMappingSvc)
			hostMappings := authenticated.Group("/host-mappings")
			{
				hostMappings.GET("", hostMappingHandler.List)
				hostMappings.GET("/:id", hostMappingHandler.GetByID)
				hostMappings.POST("", permissionMiddleware.RequireSuperAdmin(), hostMappingHandler.Create)
				hostMappings.PUT("/:id", permissionMiddleware.RequireSuperAdmin(), hostMappingHandler.Update)
				hostMappings.DELETE("/:id", permissionMiddleware.RequireSuperAdmin(), hostMappingHandler.Delete)
			}
		}
	}

	// 静态资源服务 - 服务前端构建产物
	// 配置前端构建产物的目录路径
	frontendDistPath := "./frontend/dist"

	// 静态资源目录（JS、CSS 等）—— 同样挂在前缀下，与前端 vite base 对齐
	r.Static(webBase+"/assets", frontendDistPath+"/assets")

	// 其他静态资源文件
	r.StaticFile(webBase+"/vite.svg", frontendDistPath+"/vite.svg")

	// 应用入口，直接返回 index.html
	r.GET(webBase, func(c *gin.Context) {
		c.File(frontendDistPath + "/index.html")
	})

	// SPA fallback - 带前缀的非 API 路由返回 index.html
	// 这支持前端路由（如 /clusters, /topics 等）在刷新时能正常工作
	r.NoRoute(func(c *gin.Context) {
		reqPath := c.Request.URL.Path
		// API 请求（带前缀或根 /api）未命中路由 → 404 JSON
		if strings.HasPrefix(reqPath, webBase+"/api") || strings.HasPrefix(reqPath, "/api") {
			c.JSON(404, gin.H{
				"code":    404,
				"message": "API endpoint not found",
			})
			return
		}
		// 带前缀的请求：有文件扩展名视为静态资源 → 404；否则返回 index.html（SPA 路由）
		if strings.HasPrefix(reqPath, webBase+"/") || reqPath == webBase {
			if path.Ext(reqPath) != "" {
				c.JSON(404, gin.H{
					"code":    404,
					"message": "resource not found",
				})
				return
			}
			c.File(frontendDistPath + "/index.html")
			return
		}
		// 其它根路径请求重定向到入口
		c.Redirect(http.StatusFound, webBase)
	})

	return r
}
