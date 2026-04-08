package errors

import (
	"context"
	"fmt"
	"time"
)

// RetryConfig 重试配置
type RetryConfig struct {
	MaxAttempts int           // 最大重试次数
	InitialDelay time.Duration // 初始延迟
	MaxDelay     time.Duration // 最大延迟
	Multiplier   float64       // 退避倍数
}

// DefaultRetryConfig 默认重试配置
var DefaultRetryConfig = RetryConfig{
	MaxAttempts:  3,
	InitialDelay: 100 * time.Millisecond,
	MaxDelay:     5 * time.Second,
	Multiplier:   2.0,
}

// RetryFunc 重试函数类型
type RetryFunc func() error

// RetryWithBackoff 带指数退避的重试
func RetryWithBackoff(ctx context.Context, config RetryConfig, fn RetryFunc) error {
	var err error
	delay := config.InitialDelay

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		default:
		}

		err = fn()
		if err == nil {
			return nil
		}

		// 检查是否可重试
		if !IsRetryable(err) {
			return err
		}

		// 如果不是最后一次尝试，等待后重试
		if attempt < config.MaxAttempts {
			select {
			case <-ctx.Done():
				return fmt.Errorf("retry cancelled: %w", ctx.Err())
			case <-time.After(delay):
			}

			// 指数退避
			delay = time.Duration(float64(delay) * config.Multiplier)
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}
		}
	}

	return fmt.Errorf("max retry attempts (%d) exceeded: %w", config.MaxAttempts, err)
}

// RetryWithFixedDelay 固定延迟重试
func RetryWithFixedDelay(ctx context.Context, maxAttempts int, delay time.Duration, fn RetryFunc) error {
	config := RetryConfig{
		MaxAttempts:  maxAttempts,
		InitialDelay: delay,
		MaxDelay:     delay,
		Multiplier:   1.0,
	}
	return RetryWithBackoff(ctx, config, fn)
}

// RetryUntilSuccess 直到成功（无最大次数限制）
func RetryUntilSuccess(ctx context.Context, initialDelay, maxDelay time.Duration, multiplier float64, fn RetryFunc) error {
	delay := initialDelay

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		default:
		}

		err := fn()
		if err == nil {
			return nil
		}

		// 检查是否可重试
		if !IsRetryable(err) {
			return err
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		case <-time.After(delay):
		}

		// 指数退避
		delay = time.Duration(float64(delay) * multiplier)
		if delay > maxDelay {
			delay = maxDelay
		}
	}
}