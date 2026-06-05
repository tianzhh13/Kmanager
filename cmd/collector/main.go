package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"kafka-management-platform/internal/collector"
	"kafka-management-platform/internal/config"
	"kafka-management-platform/internal/database"
	"kafka-management-platform/internal/logger"
	"kafka-management-platform/internal/repository"

	"gorm.io/gorm"
)

func main() {
	// 加载配置（复用平台配置文件）
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 环境变量覆盖日志输出路径
	logOutputPath := cfg.Log.OutputPath
	if envPath := os.Getenv("LOG_OUTPUT_PATH"); envPath != "" {
		logOutputPath = envPath
	}

	// 初始化日志
	if err := logger.InitWithConfig(
		cfg.Log.Level,
		cfg.Log.Format,
		logOutputPath,
		cfg.Log.MaxSize,
		cfg.Log.MaxBackups,
		cfg.Log.MaxAge,
		cfg.Log.Compress,
	); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	log := logger.GetLogger()
	log.Info("Starting Kafka Collector...")

	// 初始化数据库（只读集群列表）
	db, err := database.Init(cfg)
	if err != nil {
		log.Fatal("Failed to initialize database", "error", err)
	}
	defer closeDB(db)

	// 初始化 Repository
	clusterRepo := repository.NewClusterRepository(db)

	// 创建并启动 Collector
	c := collector.NewCollector(cfg, clusterRepo)
	if err := c.Start(); err != nil {
		log.Fatal("Failed to start collector", "error", err)
	}
	defer c.Stop()

	log.Info("Kafka Collector is running")

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down collector...")
	log.Info("Collector exited")
}

// closeDB 关闭 GORM 数据库连接
func closeDB(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.Close()
	}
}
