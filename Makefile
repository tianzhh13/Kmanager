.PHONY: help build run test clean install lint

# 默认目标
help:
	@echo "Kafka Management Platform - Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make build      - 编译应用"
	@echo "  make run        - 运行应用"
	@echo "  make test       - 运行测试"
	@echo "  make test-cover - 运行测试并生成覆盖率报告"
	@echo "  make clean      - 清理构建文件"
	@echo "  make install    - 安装依赖"
	@echo "  make lint       - 运行代码检查"
	@echo "  make fmt        - 格式化代码"

# 编译应用
build:
	@echo "Building application..."
	go build -o kafka-management-platform cmd/server/main.go

# 运行应用
run:
	@echo "Running application..."
	go run cmd/server/main.go

# 运行测试
test:
	@echo "Running tests..."
	go test -v ./...

# 运行测试并生成覆盖率报告
test-cover:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.txt -covermode=atomic ./...
	go tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report generated: coverage.html"

# 清理构建文件
clean:
	@echo "Cleaning..."
	rm -f kafka-management-platform
	rm -f coverage.txt coverage.html
	go clean

# 安装依赖
install:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# 代码检查
lint:
	@echo "Running linter..."
	golangci-lint run ./...

# 格式化代码
fmt:
	@echo "Formatting code..."
	go fmt ./...
	gofmt -s -w .

# 生成加密密钥
gen-key:
	@echo "Generating encryption key..."
	@openssl rand -base64 32
