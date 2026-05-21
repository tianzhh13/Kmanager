package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"kafka-management-platform/internal/collector"
	"kafka-management-platform/internal/config"
	"kafka-management-platform/internal/database"
	"kafka-management-platform/internal/repository"

	"gorm.io/gorm"
)

func main() {
	fmt.Println("Starting Kafka Collector...")

	// 加载配置（复用平台配置文件）
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 初始化数据库（只读集群列表）
	db, err := database.Init(cfg)
	if err != nil {
		fmt.Printf("Failed to initialize database: %v\n", err)
		os.Exit(1)
	}
	defer closeDB(db)

	// 初始化 Repository
	clusterRepo := repository.NewClusterRepository(db)

	// 创建并启动 Collector
	c := collector.NewCollector(cfg, clusterRepo)
	if err := c.Start(); err != nil {
		fmt.Printf("Failed to start collector: %v\n", err)
		os.Exit(1)
	}
	defer c.Stop()

	fmt.Println("Kafka Collector is running")

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("Shutting down collector...")
	fmt.Println("Collector exited")
}

// closeDB 关闭 GORM 数据库连接
func closeDB(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.Close()
	}
}
