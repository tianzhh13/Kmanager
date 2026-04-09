# Kafka 管理平台

企业级 Kafka 集群管理系统，提供统一的多集群管理、权限控制、监控和审计功能。

## 功能特性

- 🔐 **用户认证授权**：基于 JWT 的认证，支持超级管理员、集群管理员、只读用户三种角色
- 🌐 **多集群管理**：统一管理多个 Kafka 集群，支持 PLAINTEXT、SCRAM、Kerberos 认证
- 📊 **Topic 管理**：可视化创建、删除、配置 Topic，自动同步集群数据
- 🔒 **ACL 管理**：管理 Kafka 访问控制规则，支持批量操作
- 📈 **监控展示**：集成 Prometheus，展示集群、Broker、Topic、消费组级别监控指标
- 📝 **操作审计**：完整记录所有关键操作，支持查询和导出
- 🔐 **敏感信息加密**：使用 AES-256-CFB 加密存储集群认证信息
- ⏰ **数据同步**：后台 Worker 每 5 分钟自动同步 Topic 和 ACL 数据

## 技术栈

**后端**：
- Go 1.26
- Gin（Web 框架）
- GORM（ORM）
- Sarama（Kafka 客户端）
- JWT（认证）
- Zap（日志）

**前端**：
- React 18
- TypeScript
- Ant Design 5
- Redux Toolkit
- ECharts

**数据库**：
- MySQL 8.0+ / PostgreSQL 14+

**外部依赖**：
- Kafka 2.8+
- Prometheus 2.40+

## 快速开始

### 前置要求

- Go 1.21+
- MySQL 8.0+ 或 PostgreSQL 14+
- Node.js 18+（前端开发）
- Kafka 集群（用于测试）
- Prometheus（可选，用于监控功能）

### 安装步骤

1. **克隆项目**

```bash
git clone <repository-url>
cd kafka-management-platform
```

2. **配置文件**

复制配置模板并修改：

```bash
cp configs/config.yaml.example configs/config.yaml
```

编辑 `configs/config.yaml`，配置数据库、JWT 密钥、加密密钥等。

生成加密密钥：

```bash
# 生成 32 字节的 AES-256 密钥（Base64 编码）
openssl rand -base64 32
```

3. **安装依赖**

```bash
go mod tidy
go mod vendor
```

4. **初始化数据库**

创建数据库：

```sql
CREATE DATABASE kafka_management CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

运行数据库迁移（应用启动时自动执行）。

5. **运行后端应用**

```bash
go run cmd/server/main.go
```

应用将在 `http://localhost:8080` 启动。

6. **运行前端应用**

```bash
cd frontend
npm install
npm run dev
```

前端将在 `http://localhost:5173` 启动，并代理 API 请求到后端 `http://localhost:8080`。

7. **访问 Web UI**

打开浏览器访问 `http://localhost:5173`，使用默认管理员账户登录。

### 默认管理员账户

- **用户名**：`admin`
- **密码**：`admin123`
- **角色**：超级管理员

⚠️ **重要**：首次登录后请立即修改默认密码！

## 项目结构

```
kafka-management-platform/
├── cmd/
│   └── server/
│       └── main.go              # 应用入口
├── internal/
│   ├── config/                  # 配置管理
│   ├── database/                # 数据库连接
│   ├── logger/                  # 日志模块
│   ├── router/                  # 路由定义
│   ├── models/                  # 数据模型
│   ├── repository/              # 数据访问层
│   ├── service/                 # 业务逻辑层
│   │   ├── auth/               # 认证服务
│   │   ├── cluster/            # 集群管理服务
│   │   ├── topic/              # Topic 管理服务
│   │   ├── acl/                # ACL 管理服务
│   │   ├── monitor/            # 监控服务
│   │   └── audit/              # 审计服务
│   ├── middleware/              # 中间件
│   ├── handler/                 # HTTP 处理器
│   └── worker/                  # 后台任务
├── pkg/
│   ├── encryption/              # 加密工具
│   ├── kafka/                   # Kafka 客户端封装
│   └── validator/               # 验证器
├── configs/
│   └── config.yaml              # 配置文件
├── frontend/                    # 前端项目
├── docs/                        # 文档
├── scripts/                     # 脚本
├── go.mod
├── go.sum
└── README.md
```

## 配置说明

### 数据库配置

支持 MySQL 和 PostgreSQL：

```yaml
database:
  type: mysql  # mysql 或 postgres
  host: localhost
  port: 3306
  username: root
  password: your_password
  database: kafka_management
```

### JWT 配置

```yaml
jwt:
  secret: your-jwt-secret-key
  access_token_expire: 3600     # 1 小时
  refresh_token_expire: 604800  # 7 天
```

### 加密配置

```yaml
encryption:
  key: your-base64-encoded-32-byte-key
```

## API 文档

### 认证 API

#### 登录
```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "admin123"
  }'
```

响应示例：
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_in": 3600,
  "user_info": {
    "user_id": 1,
    "username": "admin",
    "email": "admin@example.com",
    "role": "super_admin"
  }
}
```

#### 获取当前用户信息
```bash
curl http://localhost:8080/api/v1/auth/me \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

### 集群管理 API

#### 获取集群列表
```bash
curl http://localhost:8080/api/v1/clusters \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

#### 创建集群
```bash
curl -X POST http://localhost:8080/api/v1/clusters \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "cluster_name": "测试集群",
    "bootstrap_servers": "localhost:9092",
    "auth_type": "plaintext",
    "prometheus_url": "http://localhost:9090",
    "description": "开发环境测试集群"
  }'
```

#### 测试集群连接
```bash
curl -X POST http://localhost:8080/api/v1/clusters/1/test \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

#### 授予用户集群权限
```bash
curl -X POST http://localhost:8080/api/v1/clusters/1/grant \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"user_id": 2}'
```

### Topic 管理 API

#### 获取 Topic 列表
```bash
curl "http://localhost:8080/api/v1/topics?cluster_id=1" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

#### 创建 Topic
```bash
curl -X POST http://localhost:8080/api/v1/topics \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "cluster_id": 1,
    "topic_name": "test-topic",
    "partitions": 3,
    "replication_factor": 1
  }'
```

#### 同步 Topic 数据
```bash
curl -X POST http://localhost:8080/api/v1/topics/sync/1 \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

### ACL 管理 API

#### 获取 ACL 列表
```bash
curl "http://localhost:8080/api/v1/acls?cluster_id=1" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

#### 创建 ACL 规则
```bash
curl -X POST http://localhost:8080/api/v1/acls \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "cluster_id": 1,
    "resource_type": "topic",
    "resource_name": "test-topic",
    "resource_pattern": "literal",
    "principal": "User:test-user",
    "host": "*",
    "operation": "read",
    "permission_type": "allow"
  }'
```

#### 批量删除 ACL
```bash
curl -X POST http://localhost:8080/api/v1/acls/batch-delete \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"acl_ids": [1, 2, 3]}'
```

### 监控 API

#### 获取集群监控指标
```bash
curl "http://localhost:8080/api/v1/metrics/cluster/1?start=2024-01-01T00:00:00Z&end=2024-01-01T01:00:00Z" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

#### 获取 Topic 监控指标
```bash
curl "http://localhost:8080/api/v1/metrics/topic/1?topic=test-topic" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

### 审计日志 API

#### 查询审计日志
```bash
curl "http://localhost:8080/api/v1/audit-logs?page=1&page_size=20" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

主要 API 端点：

**认证**：
- `POST /api/v1/auth/login` - 用户登录
- `POST /api/v1/auth/refresh` - 刷新 Token
- `GET /api/v1/auth/me` - 获取当前用户信息

**用户管理**（需要超级管理员权限）：
- `GET /api/v1/users` - 获取用户列表
- `POST /api/v1/users` - 创建用户
- `GET /api/v1/users/:id` - 获取用户详情
- `PUT /api/v1/users/:id` - 更新用户
- `DELETE /api/v1/users/:id` - 删除用户
- `PUT /api/v1/users/:id/password` - 修改密码
- `POST /api/v1/users/:id/disable` - 禁用用户
- `POST /api/v1/users/:id/enable` - 启用用户

**集群管理**：
- `GET /api/v1/clusters` - 获取集群列表
- `POST /api/v1/clusters` - 创建集群
- `GET /api/v1/clusters/:id` - 获取集群详情
- `PUT /api/v1/clusters/:id` - 更新集群
- `DELETE /api/v1/clusters/:id` - 删除集群
- `POST /api/v1/clusters/:id/test` - 测试连接
- `POST /api/v1/clusters/:id/grant` - 授予用户权限
- `POST /api/v1/clusters/:id/revoke` - 撤销用户权限
- `GET /api/v1/clusters/:id/users` - 获取授权用户列表

**Topic 管理**：
- `GET /api/v1/topics` - 获取 Topic 列表
- `POST /api/v1/topics` - 创建 Topic
- `GET /api/v1/topics/:name` - 获取 Topic 详情
- `DELETE /api/v1/topics/:name` - 删除 Topic
- `PUT /api/v1/topics/:name/config` - 更新 Topic 配置
- `POST /api/v1/topics/sync/:id` - 同步 Topic 数据

**ACL 管理**：
- `GET /api/v1/acls` - 获取 ACL 列表
- `POST /api/v1/acls` - 创建 ACL
- `DELETE /api/v1/acls/:id` - 删除 ACL
- `POST /api/v1/acls/batch-delete` - 批量删除 ACL
- `POST /api/v1/acls/sync/:id` - 同步 ACL 数据

**监控**：
- `GET /api/v1/metrics/cluster/:id` - 获取集群指标
- `GET /api/v1/metrics/broker/:id` - 获取 Broker 指标
- `GET /api/v1/metrics/topic/:id` - 获取 Topic 指标
- `GET /api/v1/metrics/consumer-group/:id` - 获取消费组指标
- `GET /api/v1/metrics/query/:id` - 自定义 PromQL 查询

**审计日志**：
- `GET /api/v1/audit-logs` - 查询审计日志
- `GET /api/v1/audit-logs/export` - 导出审计日志

## Web UI 使用指南

### 访问前端

1. 启动后端服务：
```bash
go run cmd/server/main.go
```

2. 启动前端开发服务器：
```bash
cd frontend
npm install
npm run dev
```

3. 打开浏览器访问 `http://localhost:5173`

### 页面功能

| 页面 | 路径 | 功能说明 |
|------|------|----------|
| 登录 | `/login` | 用户登录认证 |
| 仪表盘 | `/dashboard` | 系统概览，显示关键指标 |
| 集群管理 | `/clusters` | 集群列表、创建、编辑、删除、连接测试 |
| Topic 管理 | `/topics` | Topic 列表、创建、删除、配置修改 |
| ACL 管理 | `/acls` | ACL 规则列表、创建、删除、批量操作 |
| 监控 | `/monitor` | 集群、Broker、Topic、消费组监控指标 |
| 审计日志 | `/audit-logs` | 操作日志查询、过滤、导出 |
| 用户管理 | `/users` | 用户列表、创建、编辑、禁用（需要超级管理员权限） |

### 前端技术栈

- **框架**：React 18 + TypeScript
- **UI 组件**：Ant Design 5
- **状态管理**：Redux Toolkit
- **路由**：React Router 6
- **HTTP 客户端**：Axios
- **图表**：ECharts
- **构建工具**：Vite

### 前端构建

```bash
cd frontend
npm run build
```

构建产物将输出到 `frontend/dist` 目录，可以部署到任何静态文件服务器。

## 开发指南

### 添加新功能

1. 在 `internal/models` 定义数据模型
2. 在 `internal/repository` 实现数据访问
3. 在 `internal/service` 实现业务逻辑
4. 在 `internal/handler` 实现 HTTP 处理器
5. 在 `internal/router` 注册路由

### 运行测试

```bash
# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./internal/service/auth/...

# 运行测试并显示覆盖率
go test -cover ./...
```

### 代码规范

- 使用 `gofmt` 格式化代码
- 使用 `golangci-lint` 进行代码检查
- 遵循 Go 官方代码规范

## 部署

### 环境要求

- 操作系统：Linux / Windows / macOS
- 数据库：MySQL 8.0+ 或 PostgreSQL 14+
- Kafka 集群：2.8+（需要启用 Admin API）
- Prometheus：2.40+（可选，用于监控功能）

### 二进制部署

```bash
# 编译后端
go build -o kafka-management-platform cmd/server/main.go

# 编译前端
cd frontend
npm run build

# 运行后端
./kafka-management-platform

# 使用 Nginx 托管前端静态文件
# nginx.conf 示例：
# server {
#   listen 80;
#   server_name your-domain.com;
#   
#   location / {
#     root /path/to/frontend/dist;
#     try_files $uri $uri/ /index.html;
#   }
#   
#   location /api {
#     proxy_pass http://localhost:8080;
#     proxy_set_header Host $host;
#     proxy_set_header X-Real-IP $remote_addr;
#   }
# }
```

### Docker 部署

```bash
# 构建镜像
docker build -t kafka-management-platform .

# 运行容器
docker run -d -p 8080:8080 \
  -v $(pwd)/configs:/app/configs \
  -e DATABASE_HOST=your-db-host \
  -e DATABASE_PASSWORD=your-db-password \
  kafka-management-platform

# 使用 docker-compose
docker-compose up -d
```

### systemd 服务

创建 `/etc/systemd/system/kafka-management-platform.service`：

```ini
[Unit]
Description=Kafka Management Platform
After=network.target mysql.service

[Service]
Type=simple
User=kafka
WorkingDirectory=/opt/kafka-management-platform
ExecStart=/opt/kafka-management-platform/kafka-management-platform
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

启动服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable kafka-management-platform
sudo systemctl start kafka-management-platform
```

### 生产环境配置建议

1. **安全配置**：
   - 修改 `server.mode` 为 `release`
   - 使用强密码和密钥
   - 启用 HTTPS
   - 配置防火墙规则

2. **性能配置**：
   - 调整数据库连接池大小
   - 配置 Redis 缓存（可选）
   - 启用 GZIP 压缩

3. **监控配置**：
   - 配置 Prometheus URL
   - 启用日志收集
   - 配置告警规则

## 故障排查

### 数据库连接失败

检查数据库配置和网络连接：

```bash
# MySQL
mysql -h localhost -u root -p

# PostgreSQL
psql -h localhost -U postgres
```

### 日志查看

```bash
# 查看应用日志
tail -f /var/log/kafka-management-platform.log

# 查看 systemd 日志
journalctl -u kafka-management-platform -f
```

## 贡献指南

欢迎贡献代码！请遵循以下步骤：

1. Fork 项目
2. 创建功能分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建 Pull Request

## 许可证

[MIT License](LICENSE)

## 联系方式

- 项目主页：<repository-url>
- 问题反馈：<repository-url>/issues

## 更新日志

### v0.3.0 (当前版本)

**已完成功能**：
- ✅ 用户认证授权（JWT + bcrypt + RBAC）
- ✅ 多集群管理（支持 PLAINTEXT、SCRAM、Kerberos 认证）
- ✅ Topic 管理（CRUD + 同步）
- ✅ ACL 管理（CRUD + 批量操作 + 同步）
- ✅ 监控服务（Prometheus 集成）
- ✅ 审计日志（记录 + 查询 + 清理）
- ✅ 数据同步 Worker（定时同步 Topic 和 ACL）
- ✅ 用户管理（CRUD + 禁用/启用）
- ✅ 前端 Web UI（React + Ant Design）
- ✅ 敏感信息加密（AES-256-CFB）
- ✅ API 限流和安全中间件

**待完善功能**：
- ⏳ 前端构建产物（需要 Node.js 环境）
- ⏳ 集成测试
- ⏳ 性能测试
- ⏳ API 文档（Swagger）

### v0.1.0 (初始版本)

- ✅ 初始项目结构
- ✅ 基础配置和日志模块
- ✅ 数据库连接池
- ✅ 路由框架
