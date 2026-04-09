# Kafka 管理平台使用指南

## 目录

1. [系统架构](#系统架构)
2. [快速开始](#快速开始)
3. [Web UI 使用](#web-ui-使用)
4. [API 使用](#api-使用)
5. [常见问题](#常见问题)

## 系统架构

```
┌─────────────────────────────────────────────────────────┐
│                      前端 (React)                        │
│                  http://localhost:5173                   │
└─────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────┐
│                    后端 API (Go + Gin)                   │
│                  http://localhost:8080                   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────┐ │
│  │ 认证服务 │  │ 集群服务 │  │Topic服务 │  │ ACL服务 │ │
│  └──────────┘  └──────────┘  └──────────┘  └─────────┘ │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐              │
│  │ 监控服务 │  │ 审计服务 │  │同步Worker│              │
│  └──────────┘  └──────────┘  └──────────┘              │
└─────────────────────────────────────────────────────────┘
        │                 │                 │
        ▼                 ▼                 ▼
┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│   MySQL/PG   │  │Kafka 集群(s) │  │  Prometheus  │
│   数据库     │  │              │  │   (可选)     │
└──────────────┘  └──────────────┘  └──────────────┘
```

## 快速开始

### 1. 环境准备

确保已安装以下软件：

- Go 1.21+
- MySQL 8.0+ 或 PostgreSQL 14+
- Node.js 18+ 和 npm
- Kafka 集群（用于测试）
- Prometheus（可选，用于监控）

### 2. 配置数据库

**MySQL 示例**：

```sql
CREATE DATABASE kafka_management CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'kafka_admin'@'localhost' IDENTIFIED BY 'your_password';
GRANT ALL PRIVILEGES ON kafka_management.* TO 'kafka_admin'@'localhost';
FLUSH PRIVILEGES;
```

### 3. 配置应用

```bash
# 复制配置文件
cp configs/config.yaml.example configs/config.yaml

# 生成加密密钥
openssl rand -base64 32

# 编辑配置文件
vim configs/config.yaml
```

**关键配置项**：

```yaml
server:
  port: 8080
  mode: debug  # 生产环境改为 release

database:
  type: mysql
  host: localhost
  port: 3306
  username: kafka_admin
  password: your_password
  database: kafka_management

jwt:
  secret: your-jwt-secret-key-at-least-32-characters-long
  access_token_expire: 3600     # 1 小时
  refresh_token_expire: 604800  # 7 天

encryption:
  key: your-base64-encoded-32-byte-key  # 从 openssl 生成
```

### 4. 启动服务

**启动后端**：

```bash
# 安装依赖
go mod tidy
go mod vendor

# 运行
go run cmd/server/main.go
```

**启动前端**：

```bash
cd frontend
npm install
npm run dev
```

### 5. 访问系统

- **Web UI**: http://localhost:5173
- **API**: http://localhost:8080
- **健康检查**: http://localhost:8080/health

**默认管理员账户**：
- 用户名：`admin`
- 密码：`admin123`

⚠️ **首次登录后请立即修改密码！**

## Web UI 使用

### 登录页面

1. 访问 http://localhost:5173/login
2. 输入用户名和密码
3. 点击"登录"按钮
4. 登录成功后自动跳转到仪表盘

### 仪表盘

显示系统关键指标：
- 集群总数
- Topic 总数
- 用户数量
- 最近操作记录

### 集群管理

#### 创建集群

1. 点击左侧菜单"集群管理"
2. 点击"创建集群"按钮
3. 填写集群信息：
   - 集群名称：如"生产环境集群"
   - Bootstrap Servers：如"kafka1:9092,kafka2:9092"
   - 认证类型：PLAINTEXT / SCRAM / Kerberos
   - Prometheus URL：如"http://prometheus:9090"（可选）
   - 描述：集群用途说明

4. 根据认证类型填写认证信息：
   - **PLAINTEXT**：无需额外配置
   - **SCRAM**：填写用户名和密码
   - **Kerberos**：填写 Principal 和 Keytab

5. 点击"测试连接"验证配置
6. 点击"创建"保存集群

#### 管理集群权限

1. 在集群列表中点击"管理权限"
2. 查看已授权用户列表
3. 点击"添加用户"授予用户管理权限
4. 点击"撤销"移除用户权限

### Topic 管理

#### 创建 Topic

1. 点击左侧菜单"Topic 管理"
2. 选择目标集群
3. 点击"创建 Topic"
4. 填写 Topic 信息：
   - Topic 名称：如"user-events"
   - 分区数：如 3
   - 副本数：如 2
   - 配置参数（可选）：如 `retention.ms=86400000`

5. 点击"创建"

#### 同步 Topic 数据

1. 在 Topic 列表页点击"同步"
2. 系统将从 Kafka 集群同步最新的 Topic 列表和配置
3. 同步完成后自动刷新列表

### ACL 管理

#### 创建 ACL 规则

1. 点击左侧菜单"ACL 管理"
2. 选择目标集群
3. 点击"创建 ACL"
4. 填写 ACL 信息：
   - 资源类型：Topic / Group / Cluster
   - 资源名称：如"test-topic"
   - 匹配模式：Literal（精确匹配）/ Prefixed（前缀匹配）
   - Principal：如"User:app-consumer"
   - Host：如"*" 或具体 IP
   - 操作：Read / Write / Create / Delete / All
   - 权限类型：Allow / Deny

5. 点击"创建"

#### 批量删除 ACL

1. 在 ACL 列表中勾选要删除的规则
2. 点击"批量删除"
3. 确认删除操作

### 监控页面

#### 集群监控

1. 点击左侧菜单"监控"
2. 选择目标集群
3. 查看集群级别指标：
   - Broker 数量
   - Topic 数量
   - 消息流入速率
   - 字节流入/流出速率

#### Topic 监控

1. 在监控页面选择"Topic 监控"
2. 选择要监控的 Topic
3. 查看指标：
   - 消息流入/流出速率
   - 分区数量
   - 字节速率

#### 消费组监控

1. 选择"消费组监控"
2. 输入消费组名称
3. 查看指标：
   - 消费延迟（Lag）
   - 消费速率
   - 成员数量

### 审计日志

#### 查询审计日志

1. 点击左侧菜单"审计日志"
2. 使用过滤器筛选：
   - 用户名
   - 操作类型
   - 资源类型
   - 时间范围
   - 状态（成功/失败）

3. 点击"查询"查看结果

#### 导出审计日志

1. 设置查询条件
2. 点击"导出"按钮
3. 选择导出格式（CSV / JSON）
4. 下载文件

### 用户管理

> 需要"超级管理员"权限

#### 创建用户

1. 点击左侧菜���"用户管理"
2. 点击"创建用户"
3. 填写用户信息：
   - 用户名
   - 密码
   - 邮箱
   - 角色：超级管理员 / 集群管理员 / 只读用户

4. 点击"创建"

#### 禁用/启用用户

1. 在用户列表中找到目标用户
2. 点击"禁用"或"启用"按钮
3. 确认操作

## API 使用

### 认证流程

```bash
# 1. 登录获取 Token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.access_token')

# 2. 使用 Token 访问 API
curl http://localhost:8080/api/v1/clusters \
  -H "Authorization: Bearer $TOKEN"
```

### 常用 API 示例

#### 创建 SCRAM 认证的集群

```bash
curl -X POST http://localhost:8080/api/v1/clusters \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "cluster_name": "生产集群",
    "bootstrap_servers": "kafka1:9092,kafka2:9092,kafka3:9092",
    "auth_type": "scram",
    "auth_config": {
      "username": "admin",
      "password": "admin-secret",
      "mechanism": "SCRAM-SHA-256"
    },
    "prometheus_url": "http://prometheus:9090",
    "description": "生产环境 Kafka 集群"
  }'
```

#### 创建 Topic 并配置

```bash
curl -X POST http://localhost:8080/api/v1/topics \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "cluster_id": 1,
    "topic_name": "user-events",
    "partitions": 6,
    "replication_factor": 3,
    "config": {
      "retention.ms": "604800000",
      "cleanup.policy": "delete",
      "compression.type": "lz4"
    }
  }'
```

#### 查询监控数据

```bash
# 获取最近 1 小时的集群指标
START=$(date -u -d '1 hour ago' +%Y-%m-%dT%H:%M:%SZ)
END=$(date -u +%Y-%m-%dT%H:%M:%SZ)

curl "http://localhost:8080/api/v1/metrics/cluster/1?start=${START}&end=${END}" \
  -H "Authorization: Bearer $TOKEN"
```

## 常见问题

### 1. 数据库连接失败

**问题**：启动时报错 `failed to connect to database`

**解决方案**：
- 检查数据库服务是否运行
- 验证配置文件中的数据库连接信息
- 检查数据库用户权限
- 确认数据库已创建

```bash
# 测试数据库连接
mysql -h localhost -u kafka_admin -p kafka_management
```

### 2. JWT Token 无效

**问题**：API 返回 401 Unauthorized

**解决方案**：
- 检查 Token 是否过期（默认 1 小时）
- 使用 Refresh Token 获取新的 Access Token
- 确认 JWT Secret 配置正确

```bash
# 刷新 Token
curl -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"your-refresh-token"}'
```

### 3. Kafka 连接失败

**问题**：测试集群连接失败

**解决方案**：
- 检查 Kafka 集群是否运行
- 验证 Bootstrap Servers 地址和端口
- 检查认证配置是否正确
- 查看应用日志获取详细错误信息

```bash
# 测试 Kafka 连接
kafka-broker-api-versions --bootstrap-server kafka1:9092
```

### 4. 前端无法访问后端 API

**问题**：前端显示网络错误

**解决方案**：
- 确认后端服务已启动
- 检查 CORS 配置
- 验证 API 地址配置

### 5. 监控数据不显示

**问题**：监控页面无数据

**解决方案**：
- 确认集群已配置 Prometheus URL
- 检查 Prometheus 服务是否运行
- 验证 Prometheus 是否已采集 Kafka 指标
- 确认 PromQL 查询语法正确

```bash
# 测试 Prometheus 查询
curl "http://localhost:9090/api/v1/query?query=up"
```

### 6. 数据同步失败

**问题**：Topic 或 ACL 数据未同步

**解决方案**：
- 检查 Kafka Admin API 是否可用
- 查看应用日志中的同步错误
- 手动触发同步操作
- 验证 Kafka 用户权限

### 7. 权限不足

**问题**：操作被拒绝，返回 403 Forbidden

**解决方案**：
- 确认用户角色和权限
- 检查集群权限配置
- 联系超级管理员申请权限

## 性能优化建议

### 后端优化

1. **数据库优化**：
   - 为高频查询字段添加索引
   - 调整连接池大小
   - 定期清理审计日志

2. **缓存配置**：
   - 启用 Redis 缓存（可选）
   - 配置合理的缓存过期时间

3. **并发控制**：
   - 调整 Worker 同步间隔
   - 限制并发请求数量

### 前端优化

1. **构建优化**：
   - 启用代码分割
   - 压缩静态资源
   - 使用 CDN 加速

2. **运行时优化**：
   - 启用虚拟滚动（大数据列表）
   - 懒加载监控图表
   - 合理使用缓存

## 安全建议

1. **生产环境必须修改**：
   - 默认管理员密码
   - JWT Secret
   - 加密密钥

2. **网络安全**：
   - 启用 HTTPS
   - 配置防火墙规则
   - 限制 API 访问频率

3. **数据安全**：
   - 定期备份数据库
   - 加密敏感配置
   - 定期审计用户权限

## 技术支持

- 项目文档：`README.md`
- API 文档：访问 `/swagger/index.html`（如果已配置）
- 问题反馈：提交 GitHub Issue