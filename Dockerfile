# ============ 构建阶段 ============
FROM golang:1.26-alpine AS builder

WORKDIR /app

# 安装构建依赖
RUN apk add --no-cache gcc musl-dev make

# 设置 Go 代理（国内加速）
ENV GOPROXY=https://goproxy.cn,direct

# 复制依赖文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 编译后端 + 采集器
RUN CGO_ENABLED=1 GOOS=linux go build -o kafka-management-platform ./cmd/server/main.go
RUN CGO_ENABLED=1 GOOS=linux go build -o kafka-collector ./cmd/collector/main.go

# ============ 运行阶段 ============
FROM alpine:latest

WORKDIR /app

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata

# 复制构建产物
COPY --from=builder /app/kafka-management-platform .
COPY --from=builder /app/kafka-collector .
COPY --from=builder /app/configs/config.yaml.example ./config.yaml.example
COPY --from=builder /app/scripts/init_db.sql ./scripts/init_db.sql
COPY --from=builder /app/scripts/init_db_postgres.sql ./scripts/init_db_postgres.sql
COPY --from=builder /app/deploy/entrypoint.sh ./entrypoint.sh
RUN chmod +x ./entrypoint.sh

# 创建非 root 用户
RUN adduser -D -u 1000 appuser
USER appuser

# 暴露端口
EXPOSE 8080

# 启动命令
ENTRYPOINT ["./entrypoint.sh", "./kafka-management-platform"]
