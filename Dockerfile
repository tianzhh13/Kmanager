FROM alpine:3.20

WORKDIR /app

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata

# 复制本地编译的二进制
COPY kafka-management-platform .
COPY kafka-collector .

# 复制前端构建产物
COPY frontend/dist ./frontend/dist

# 复制配置模板和数据库脚本
COPY configs/config.yaml.example .
COPY scripts/init_db.sql ./scripts/
COPY scripts/init_db_postgres.sql ./scripts/

# 复制启动脚本
COPY deploy/entrypoint.sh .
RUN chmod +x ./entrypoint.sh

# 创建非 root 用户，/app 目录给写权限（entrypoint.sh 需要生成 config.yaml）
RUN adduser -D -u 1000 appuser && chown appuser:appuser /app

USER appuser

# 暴露端口
EXPOSE 8080

# 启动命令
ENTRYPOINT ["./entrypoint.sh", "./kafka-management-platform"]
