#!/bin/bash

# Kafka 管理平台基础服务验证脚本

echo "=========================================="
echo "Kafka 管理平台 - 基础服务验证"
echo "=========================================="
echo ""

# 检查 Go 版本
echo "1. 检查 Go 版本..."
if command -v go &> /dev/null; then
    GO_VERSION=$(go version)
    echo "✓ Go 已安装: $GO_VERSION"
else
    echo "✗ Go 未安装，请先安装 Go 1.21+"
    exit 1
fi
echo ""

# 检查依赖
echo "2. 检查 Go 依赖..."
if go mod verify &> /dev/null; then
    echo "✓ Go 依赖验证通过"
else
    echo "✗ Go 依赖验证失败，运行 'go mod tidy' 修复"
    exit 1
fi
echo ""

# 编译检查
echo "3. 编译检查..."
if go build -o /tmp/kafka-mgmt-test cmd/server/main.go &> /dev/null; then
    echo "✓ 编译成功"
    rm -f /tmp/kafka-mgmt-test
else
    echo "✗ 编译失败，请检查代码错误"
    exit 1
fi
echo ""

# 运行测试
echo "4. 运行单元测试..."
if go test ./pkg/... -v; then
    echo "✓ 单元测试通过"
else
    echo "✗ 单元测试失败"
    exit 1
fi
echo ""

# 检查配置文件
echo "5. 检查配置文件..."
if [ -f "configs/config.yaml" ]; then
    echo "✓ 配置文件存在: configs/config.yaml"
else
    echo "⚠ 配置文件不存在，请复制 configs/config.yaml.example 并修改"
fi
echo ""

# 检查数据库连接（可选）
echo "6. 数据库连接检查..."
echo "⚠ 请手动验证数据库连接配置"
echo ""

echo "=========================================="
echo "基础服务验证完成！"
echo "=========================================="
echo ""
echo "下一步："
echo "1. 复制 configs/config.yaml.example 为 configs/config.yaml"
echo "2. 修改配置文件中的数据库、JWT、加密密钥等配置"
echo "3. 运行数据库初始化脚本: mysql < scripts/init_db.sql"
echo "4. 启动应用: go run cmd/server/main.go"
echo ""
