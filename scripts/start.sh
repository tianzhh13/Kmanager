#!/bin/bash

# Kafka 管理平台启动脚本

# 设置变量
APP_NAME="kafka-management-platform"
APP_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BINARY_DIR="$APP_DIR/bin"
CONFIG_DIR="$APP_DIR/configs"
LOG_DIR="$APP_DIR/logs"

# 创建必要的目录
mkdir -p "$LOG_DIR"

# 检查配置文件
if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
    echo "Error: config.yaml not found in $CONFIG_DIR"
    echo "Please copy config.yaml.example to config.yaml and modify it"
    exit 1
fi

# 检查二进制文件
if [ ! -f "$BINARY_DIR/$APP_NAME" ]; then
    echo "Building application..."
    cd "$APP_DIR"
    make build
fi

# 设置环境变量
export CONFIG_PATH="$CONFIG_DIR/config.yaml"

# 启动应用
echo "Starting $APP_NAME..."
cd "$BINARY_DIR"
./$APP_NAME > "$LOG_DIR/app.log" 2>&1 &

# 等待应用启动
sleep 3

# 检查应用是否启动成功
if ps -p $! > /dev/null; then
    echo "$APP_NAME started successfully (PID: $!)"
    echo "Log file: $LOG_DIR/app.log"
else
    echo "Failed to start $APP_NAME"
    echo "Check log file: $LOG_DIR/app.log"
    exit 1
fi