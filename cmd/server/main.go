package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kafka-management-platform/internal/config"
	"kafka-management-platform/internal/database"
	"kafka-management-platform/internal/logger"
	"kafka-management-platform/internal/repository"
	"kafka-management-platform/internal/router"
	"kafka-management-platform/internal/worker"
)

func main() {
	// 初始化日志
	if err := logger.Init(); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	log := logger.GetLogger()
	log.Info("Starting Kafka Management Platform...")

	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config", "error", err)
	}

	// 初始化数据库
	db, err := database.Init(cfg)
	if err != nil {
		log.Fatal("Failed to initialize database", "error", err)
	}

	// 自动迁移数据库表
	if err := database.AutoMigrate(db); err != nil {
		log.Fatal("Failed to migrate database", "error", err)
	}

	// 初始化 Repository
	clusterRepo := repository.NewClusterRepository(db)
	topicRepo := repository.NewTopicRepository(db)
	aclRepo := repository.NewACLRepository(db)
	auditLogRepo := repository.NewAuditLogRepository(db)

	// 初始化并启动 Sync Worker
	syncWorker := worker.NewSyncWorker(cfg, clusterRepo, topicRepo, aclRepo, auditLogRepo)
	if err := syncWorker.Start(); err != nil {
		log.Error("Failed to start sync worker", "error", err)
	}
	defer syncWorker.Stop()

	// 初始化路由
	r := router.Setup(cfg, db)

	// 创建 HTTP 服务器
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
	}

	// 启动服务器
	go func() {
		log.Info("Server starting", "port", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server", "error", err)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", "error", err)
	}

	log.Info("Server exited")
}
