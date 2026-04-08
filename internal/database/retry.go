package database

import (
	"context"
	"fmt"
	"time"

	"kafka-management-platform/internal/config"
	"kafka-management-platform/internal/errors"
	"kafka-management-platform/internal/logger"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// RetryConfig 数据库重试配置
type RetryConfig struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

// DefaultRetryConfig 默认数据库重试配置
var DefaultRetryConfig = RetryConfig{
	MaxAttempts:  5,
	InitialDelay: 500 * time.Millisecond,
	MaxDelay:     10 * time.Second,
	Multiplier:   2.0,
}

// InitWithRetry 带重试的数据库初始化
func InitWithRetry(cfg *config.Config, retryCfg RetryConfig) (*gorm.DB, error) {
	var db *gorm.DB
	var err error

	delay := retryCfg.InitialDelay

	for attempt := 1; attempt <= retryCfg.MaxAttempts; attempt++ {
		db, err = initDatabase(cfg)
		if err == nil {
			logger.Info("Database connected successfully",
				"type", cfg.Database.Type,
				"host", cfg.Database.Host,
				"database", cfg.Database.Database,
				"attempts", attempt,
			)
			return db, nil
		}

		logger.Warn("Database connection failed, will retry",
			"attempt", attempt,
			"max_attempts", retryCfg.MaxAttempts,
			"error", err.Error(),
			"next_retry_delay", delay,
		)

		// 如果不是最后一次尝试，等待后重试
		if attempt < retryCfg.MaxAttempts {
			time.Sleep(delay)
			// 指数退避
			delay = time.Duration(float64(delay) * retryCfg.Multiplier)
			if delay > retryCfg.MaxDelay {
				delay = retryCfg.MaxDelay
			}
		}
	}

	return nil, errors.ErrDatabaseConnection.WithError(fmt.Errorf("failed to connect to database after %d attempts: %w", retryCfg.MaxAttempts, err))
}

// initDatabase 初始化数据库连接（内部函数）
func initDatabase(cfg *config.Config) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch cfg.Database.Type {
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.Database.Username,
			cfg.Database.Password,
			cfg.Database.Host,
			cfg.Database.Port,
			cfg.Database.Database,
		)
		dialector = mysql.Open(dsn)

	case "postgres":
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Shanghai",
			cfg.Database.Host,
			cfg.Database.Port,
			cfg.Database.Username,
			cfg.Database.Password,
			cfg.Database.Database,
		)
		dialector = postgres.Open(dsn)

	default:
		return nil, fmt.Errorf("unsupported database type: %s", cfg.Database.Type)
	}

	// 配置 GORM
	gormConfig := &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	}

	db, err := gorm.Open(dialector, gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// 获取底层 SQL DB
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	// 配置连接池
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.Database.ConnMaxLifetime) * time.Second)

	// 测试连接
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// Reconnect 重新连接数据库
func Reconnect(db *gorm.DB, cfg *config.Config) (*gorm.DB, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	// 检查现有连接是否可用
	if err := sqlDB.Ping(); err == nil {
		return db, nil
	}

	logger.Warn("Database connection lost, attempting to reconnect")

	// 关闭旧连接
	sqlDB.Close()

	// 重新初始化
	return InitWithRetry(cfg, DefaultRetryConfig)
}