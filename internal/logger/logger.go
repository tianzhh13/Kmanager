package logger

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var globalLogger *zap.SugaredLogger

// InitWithConfig 使用配置初始化日志
func InitWithConfig(level, format, outputPath string, maxSize, maxBackups, maxAge int, compress bool) error {
	// 解析日志级别
	zapLevel := zapcore.InfoLevel
	if level != "" {
		if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
			return fmt.Errorf("invalid log level: %w", err)
		}
	}

	// 编码器配置
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	// 选择编码器
	var encoder zapcore.Encoder
	if format == "console" {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	// 输出目标
	var writeSyncer zapcore.WriteSyncer
	if outputPath == "" || outputPath == "stdout" {
		writeSyncer = zapcore.AddSync(os.Stdout)
	} else {
		// 确保日志目录存在
		if dir := filepath.Dir(outputPath); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create log directory: %w", err)
			}
		}
		writeSyncer = zapcore.AddSync(&lumberjack.Logger{
			Filename:   outputPath,
			MaxSize:    maxSize,    // MB
			MaxBackups: maxBackups, // 保留旧文件数
			MaxAge:     maxAge,     // 保留天数
			Compress:   compress,   // 压缩旧文件
		})
	}

	core := zapcore.NewCore(encoder, writeSyncer, zap.NewAtomicLevelAt(zapLevel))
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	globalLogger = logger.Sugar()
	return nil
}

// Init 使用默认配置初始化日志（兼容旧调用）
func Init() error {
	return InitWithConfig("info", "json", "stdout", 100, 5, 30, true)
}

// GetLogger 获取全局日志实例
func GetLogger() *zap.SugaredLogger {
	if globalLogger == nil {
		// 如果未初始化，使用默认配置
		logger, _ := zap.NewProduction()
		globalLogger = logger.Sugar()
	}
	return globalLogger
}

// Sync 刷新日志缓冲区
func Sync() {
	if globalLogger != nil {
		_ = globalLogger.Sync()
	}
}

// Debug 记录 debug 级别日志
func Debug(msg string, keysAndValues ...interface{}) {
	GetLogger().Debugw(msg, keysAndValues...)
}

// Info 记录 info 级别日志
func Info(msg string, keysAndValues ...interface{}) {
	GetLogger().Infow(msg, keysAndValues...)
}

// Warn 记录 warn 级别日志
func Warn(msg string, keysAndValues ...interface{}) {
	GetLogger().Warnw(msg, keysAndValues...)
}

// Error 记录 error 级别日志
func Error(msg string, keysAndValues ...interface{}) {
	GetLogger().Errorw(msg, keysAndValues...)
}

// Fatal 记录 fatal 级别日志并退出程序
func Fatal(msg string, keysAndValues ...interface{}) {
	GetLogger().Fatalw(msg, keysAndValues...)
}
