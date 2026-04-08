#!/bin/bash

# Kafka 管理平台停止脚本

APP_NAME="kafka-management-platform"

# 查找进程 PID
PID=$(ps -ef | grep "$APP_NAME" | grep -v grep | awk '{print $2}')

if [ -z "$PID" ]; then
    echo "$APP_NAME is not running"
    exit 0
fi

# 发送停止信号
echo "Stopping $APP_NAME (PID: $PID)..."
kill -TERM $PID

# 等待进程停止
for i in {1..30}; do
    if ! ps -p $PID > /dev/null; then
        echo "$APP_NAME stopped successfully"
        exit 0
    fi
    sleep 1
done

# 如果进程仍未停止，强制杀死
echo "Force killing $APP_NAME..."
kill -9 $PID
sleep 1

# 验证进程已停止
if ps -p $PID > /dev/null; then
    echo "Failed to stop $APP_NAME"
    exit 1
else
    echo "$APP_NAME stopped successfully"
    exit 0
fi