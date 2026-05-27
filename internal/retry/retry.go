package retry

import (
	"context"
	"fmt"
	"time"

	"kafka-management-platform/internal/logger"
)

// Config 重试配置
type Config struct {
	MaxAttempts int           // 最大重试次数
	InitialDelay time.Duration // 初始延迟
	MaxDelay     time.Duration // 最大延迟
	Multiplier   float64       // 延迟倍数
}

// DefaultConfig 默认重试配置
var DefaultConfig = Config{
	MaxAttempts:  3,
	InitialDelay: 100 * time.Millisecond,
	MaxDelay:     5 * time.Second,
	Multiplier:   2.0,
}

// Option 重试选项
type Option func(*Config)

// WithMaxAttempts 设置最大重试次数
func WithMaxAttempts(n int) Option {
	return func(c *Config) {
		c.MaxAttempts = n
	}
}

// WithInitialDelay 设置初始延迟
func WithInitialDelay(d time.Duration) Option {
	return func(c *Config) {
		c.InitialDelay = d
	}
}

// WithMaxDelay 设置最大延迟
func WithMaxDelay(d time.Duration) Option {
	return func(c *Config) {
		c.MaxDelay = d
	}
}

// WithMultiplier 设置延迟倍数
func WithMultiplier(m float64) Option {
	return func(c *Config) {
		c.Multiplier = m
	}
}

// RetryFunc 重试函数类型
type RetryFunc func(ctx context.Context) error

// Do 执行重试
func Do(ctx context.Context, fn RetryFunc, opts ...Option) error {
	cfg := DefaultConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	var lastErr error
	delay := cfg.InitialDelay

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 执行函数
		if err := fn(ctx); err == nil {
			return nil
		} else {
			lastErr = err
		}

		// 如果不是最后一次尝试，等待后重试
		if attempt < cfg.MaxAttempts {
			logger.Warn("Retry attempt",
				"attempt", attempt,
				"max_attempts", cfg.MaxAttempts,
				"error", lastErr,
				"delay", delay,
			)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}

			// 指数退避
			delay = time.Duration(float64(delay) * cfg.Multiplier)
			if delay > cfg.MaxDelay {
				delay = cfg.MaxDelay
			}
		}
	}

	return fmt.Errorf("retry failed after %d attempts: %w", cfg.MaxAttempts, lastErr)
}

// DoWithResult 执行重试并返回结果
func DoWithResult(ctx context.Context, fn func(ctx context.Context) (interface{}, error), opts ...Option) (interface{}, error) {
	cfg := DefaultConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	var lastErr error
	delay := cfg.InitialDelay

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// 执行函数
		result, err := fn(ctx)
		if err == nil {
			return result, nil
		} else {
			lastErr = err
		}

		// 如果不是最后一次尝试，等待后重试
		if attempt < cfg.MaxAttempts {
			logger.Warn("Retry attempt with result",
				"attempt", attempt,
				"max_attempts", cfg.MaxAttempts,
				"error", lastErr,
				"delay", delay,
			)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}

			// 指数退避
			delay = time.Duration(float64(delay) * cfg.Multiplier)
			if delay > cfg.MaxDelay {
				delay = cfg.MaxDelay
			}
		}
	}

	return nil, fmt.Errorf("retry failed after %d attempts: %w", cfg.MaxAttempts, lastErr)
}

// IsRetryable 判断错误是否可重试
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	// 可以添加更多可重试错误的判断
	return true
}