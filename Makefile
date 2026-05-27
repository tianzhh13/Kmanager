.PHONY: help build run test clean install lint build-frontend build-all

# 默认目标
help:
	@echo "Kafka Management Platform - Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make build          - 编译后端应用"
	@echo "  make build-frontend - 构建前端生产版本"
	@echo "  make build-all      - 构建前后端（完整构建）"
	@echo "  make run            - 运行应用"
	@echo "  make test           - 运行测试"
	@echo "  make test-cover     - 运行测试并生成覆盖率报告"
	@echo "  make clean          - 清理构建文件"
	@echo "  make clean-all      - 清理所有构建文件（包括前端）"
	@echo "  make install        - 安装后端依赖"
	@echo "  make install-frontend - 安装前端依赖"
	@echo "  make lint           - 运行代码检查"
	@echo "  make fmt            - 格式化代码"

# 编译后端应用
build:
	@echo "Building backend application..."
	go build -o kafka-management-platform cmd/server/main.go

# 构建前端生产版本
build-frontend:
	@echo "Building frontend for production..."
	cd frontend && npm run build
	@echo "Frontend build complete. Output: frontend/dist/"

# 完整构建（前后端）
build-all: build-frontend build
	@echo "Full build complete!"

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

# 清理后端构建文件
clean:
	@echo "Cleaning backend build files..."
	rm -f kafka-management-platform
	rm -f coverage.txt coverage.html
	go clean

# 清理所有构建文件（包括前端）
clean-all: clean
	@echo "Cleaning frontend build files..."
	rm -rf frontend/dist

# 安装后端依赖
install:
	@echo "Installing backend dependencies..."
	go mod download
	go mod tidy

# 安装前端依赖
install-frontend:
	@echo "Installing frontend dependencies..."
	cd frontend && npm install

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
