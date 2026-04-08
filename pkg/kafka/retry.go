package kafka

import (
	"context"
	"fmt"
	"strings"
	"time"

	"kafka-management-platform/internal/logger"
	"kafka-management-platform/internal/models"
)

// RetryConfig Kafka 重试配置
type RetryConfig struct {
	MaxAttempts  int           // 最大重试次数
	InitialDelay time.Duration // 初始延迟
	MaxDelay     time.Duration // 最大延迟
	Multiplier   float64       // 退避倍数
}

// DefaultRetryConfig 默认 Kafka 重试配置
var DefaultRetryConfig = RetryConfig{
	MaxAttempts:  3,
	InitialDelay: 1 * time.Second,
	MaxDelay:     30 * time.Second,
	Multiplier:   2.0,
}

// RetryConnect 带指数退避重试创建 Kafka Admin 客户端
func RetryConnect(cluster *models.Cluster, authConfigJSON string, config RetryConfig) (*AdminClient, error) {
	var client *AdminClient
	var err error
	delay := config.InitialDelay

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		client, err = NewAdminClient(cluster, authConfigJSON)
		if err == nil {
			logger.Info("Kafka connection established",
				"cluster_id", cluster.ID,
				"cluster_name", cluster.Name,
				"attempts", attempt,
			)
			return client, nil
		}

		logger.Warn("Kafka connection failed, will retry",
			"cluster_id", cluster.ID,
			"cluster_name", cluster.Name,
			"attempt", attempt,
			"max_attempts", config.MaxAttempts,
			"error", err.Error(),
			"next_retry_delay", delay,
		)

		// 如果不是最后一次尝试，等待后重试
		if attempt < config.MaxAttempts {
			time.Sleep(delay)
			// 指数退避
			delay = time.Duration(float64(delay) * config.Multiplier)
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}
		}
	}

	return nil, fmt.Errorf("failed to connect to Kafka after %d attempts: %w", config.MaxAttempts, err)
}

// RetryConnectContext 带上下文和指数退避重试创建 Kafka Admin 客户端
func RetryConnectContext(ctx context.Context, cluster *models.Cluster, authConfigJSON string, config RetryConfig) (*AdminClient, error) {
	var client *AdminClient
	var err error
	delay := config.InitialDelay

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("Kafka connection retry cancelled: %w", ctx.Err())
		default:
		}

		client, err = NewAdminClient(cluster, authConfigJSON)
		if err == nil {
			logger.Info("Kafka connection established",
				"cluster_id", cluster.ID,
				"cluster_name", cluster.Name,
				"attempts", attempt,
			)
			return client, nil
		}

		logger.Warn("Kafka connection failed, will retry",
			"cluster_id", cluster.ID,
			"cluster_name", cluster.Name,
			"attempt", attempt,
			"max_attempts", config.MaxAttempts,
			"error", err.Error(),
			"next_retry_delay", delay,
		)

		// 如果不是最后一次尝试，等待后重试
		if attempt < config.MaxAttempts {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("Kafka connection retry cancelled: %w", ctx.Err())
			case <-time.After(delay):
			}

			// 指数退避
			delay = time.Duration(float64(delay) * config.Multiplier)
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}
		}
	}

	return nil, fmt.Errorf("failed to connect to Kafka after %d attempts: %w", config.MaxAttempts, err)
}

// RetryOperation 带重试的 Kafka 操作
func RetryOperation(ctx context.Context, config RetryConfig, operation func() error) error {
	var err error
	delay := config.InitialDelay

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("Kafka operation retry cancelled: %w", ctx.Err())
		default:
		}

		err = operation()
		if err == nil {
			return nil
		}

		// 检查是否为可重试的错误
		if !isRetryableKafkaError(err) {
			return err
		}

		logger.Warn("Kafka operation failed, will retry",
			"attempt", attempt,
			"max_attempts", config.MaxAttempts,
			"error", err.Error(),
			"next_retry_delay", delay,
		)

		// 如果不是最后一次尝试，等待后重试
		if attempt < config.MaxAttempts {
			select {
			case <-ctx.Done():
				return fmt.Errorf("Kafka operation retry cancelled: %w", ctx.Err())
			case <-time.After(delay):
			}

			// 指数退避
			delay = time.Duration(float64(delay) * config.Multiplier)
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}
		}
	}

	return fmt.Errorf("Kafka operation failed after %d attempts: %w", config.MaxAttempts, err)
}

// isRetryableKafkaError 判断 Kafka 错误是否可重试
func isRetryableKafkaError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())

	// 网络相关错误可重试
	retryablePatterns := []string{
		"connection refused",
		"connection reset",
		"connection timeout",
		"no available broker",
		"broker not available",
		"network timeout",
		"request timeout",
		"not enough replicas",
		"leader not available",
		"offline partitions",
		"controller not available",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}

	return false
}

// TestConnectionWithRetry 带重试的连接测试
func TestConnectionWithRetry(cluster *models.Cluster, authConfigJSON string, config RetryConfig) error {
	client, err := RetryConnect(cluster, authConfigJSON, config)
	if err != nil {
		return err
	}
	defer client.Close()

	return client.TestConnection()
}