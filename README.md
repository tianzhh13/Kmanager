# Kafka 管理平台

企业级 Kafka 集群管理系统，提供统一的多集群管理、权限控制、监控和审计功能。

## 功能特性

- 🔐 **用户认证授权**：基于 JWT 的认证，支持超级管理员、集群管理员、普通用户三种角色
- 🌐 **多集群管理**：统一管理多个 Kafka 集群，支持 PLAINTEXT、SCRAM、Kerberos 认证，支持 Kerberos keytab 上传
- 📊 **Topic 管理**：可视化创建、删除、配置 Topic，支持主题描述更新、消费组查看，自动同步集群数据
- 🔒 **ACL 管理**：管理 Kafka 访问控制规则，支持批量操作、从 Kafka 直删 ACL、查看用户 ACL
- 📈 **监控展示**：集成 VictoriaMetrics，展示集群、Broker、Topic、消费组级别监控指标，支持拖拽仪表盘
- 📝 **操作审计**：完整记录所有关键操作，支持查询和导出
- 🔐 **敏感信息加密**：使用 AES-256-CFB 加密存储集群认证信息
- ⏰ **数据同步**：后台 Worker 每 5 分钟自动同步 Topic 和 ACL 数据
- 🔑 **SCRAM 用户管理**：管理 Kafka SCRAM 认证用户，支持创建、删除、同步
- 🏷️ **主机映射管理**：管理主机名到 IP 的映射关系，支持缓存和集群感知解析
- 📋 **Topic 权限管理**：管理用户对特定 Topic 的细粒度权限
- 📊 **指标采集器**：独立的 `kafka-collector` 进程，高并发采集集群指标数据
- 🧹 **Token 黑名单**：持久化 Token 黑名单，支持安全退出登录

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
- react-grid-layout（拖拽仪表盘组件）

**数据库**：
- MySQL 8.0+ / PostgreSQL 14+

**外部依赖**：
- Kafka 2.8+
- VictoriaMetrics（可选，用于监控功能）

## 快速开始

### 前置要求

- Go 1.26+
- MySQL 8.0+ 或 PostgreSQL 14+
- Node.js 18+（前端开发）
- Kafka 集群（用于测试）
- VictoriaMetrics（可选，用于监控功能）

### 安装步骤

1. **克隆项目**

```bash
git clone <repository-url>
cd kafka-management-platform
```

2. **配置文件**

```bash
cp configs/config.yaml.example configs/config.yaml
vim configs/config.yaml
```

生成加密密钥：

```bash
openssl rand -base64 32
```

3. **安装依赖**

```bash
go mod tidy
cd frontend && npm install
```

4. **初始化数据库**

```bash
# MySQL
mysql -u root -p < scripts/init_db.sql

# PostgreSQL
psql -U postgres -f scripts/init_db_postgres.sql
```

5. **运行应用**

```bash
# 启动后端
go run cmd/server/main.go

# 启动前端（新终端）
cd frontend
npm run dev
```

6. **访问 Web UI**

打开浏览器访问 `http://localhost:3000`

**默认管理员账户**：
- 用户名：`admin`
- 密码：`admin123`

⚠️ **重要**：首次登录后请立即修改默认密码！

## 项目结构

```
kafka-management-platform/
├── cmd/
│   ├── server/main.go          # 主服务入口
│   └── collector/main.go       # 指标采集器入口
├── internal/                   # 内部代码
│   ├── config/                # 配置管理
│   ├── database/              # 数据库连接
│   ├── logger/                # 日志模块
│   ├── models/                # 数据模型
│   ├── repository/            # 数据访问层
│   ├── service/               # 业务逻辑层
│   ├── handler/               # HTTP 处理器
│   ├── middleware/            # 中间件
│   ├── router/                # 路由配置
│   ├── worker/                # 后台同步任务
│   ├── collector/             # 指标采集器逻辑
│   └── cache/                 # 内存缓存（Token黑名单等）
├── pkg/                        # 公共包
│   ├── encryption/            # 加密工具
│   ├── jwt/                   # JWT 工具
│   ├── password/              # 密码工具
│   ├── kafka/                 # Kafka 客户端
│   ├── kerberos/              # Kerberos 文件管理
│   ├── victoriametrics/       # VictoriaMetrics 客户端
│   └── validator/             # 验证器
├── frontend/                   # 前端项目
├── configs/                    # 配置文件
├── deploy/                     # 部署脚本（entrypoint.sh）
├── docs/                       # 文档
└── scripts/                    # 脚本
```

## 文档

- [部署指南](docs/deployment.md) - 详细的部署说明和配置
- [API 文档](docs/api.md) - 完整的 API 端点和使用示例
- [开发指南](docs/development.md) - 开发环境搭建和代码规范
- [故障排查](docs/troubleshooting.md) - 常见问题诊断和解决

## 配置说明

### 数据库配置

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

### VictoriaMetrics 配置（监控）

```yaml
victoriametrics:
  enabled: true
  write_url: "http://victoriametrics:8428"
  query_url: "http://victoriametrics:8428"
```

### 会话与 Cookie 配置

```yaml
session:
  idle_timeout: 30  # 空闲超时（分钟），0 表示不限制

cookie:
  domain: ""
  secure: false
  same_site: "lax"
  path: "/"
```

### 日志配置

```yaml
log:
  level: info
  format: console  # json 或 console
  output_path: /var/log/kafka-management-platform.log
  max_size: 100    # 单文件最大 MB
  max_backups: 10
  max_age: 30      # 保留天数
  compress: true
```

### CORS 配置

```yaml
cors:
  allowed_origins: ["*"]
```

## 快速测试

```bash
# 健康检查
curl http://localhost:8080/health

# 获取系统配置
curl http://localhost:8080/api/v1/system/config

# 登录
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'

# 获取仪表盘概览（需要 Token）
curl http://localhost:8080/api/v1/dashboard/overview \
  -H "Authorization: Bearer YOUR_TOKEN"

# 获取集群列表（需要 Token）
curl http://localhost:8080/api/v1/clusters \
  -H "Authorization: Bearer YOUR_TOKEN"

# 退出登录
curl -X POST http://localhost:8080/api/v1/auth/logout \
  -H "Authorization: Bearer YOUR_TOKEN"
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
- ✅ 用户认证授权（JWT + bcrypt + RBAC + Token黑名单）
- ✅ 多集群管理（支持 PLAINTEXT、SCRAM、Kerberos 认证 + keytab 上传）
- ✅ Topic 管理（CRUD + 描述更新 + 消费组查看 + 同步）
- ✅ ACL 管理（CRUD + 批量操作 + 从Kafka直删 + 用户ACL查看 + 同步）
- ✅ SCRAM 用户管理（CRUD + 同步）
- ✅ 监控服务（VictoriaMetrics 集成 + 拖拽仪表盘）
- ✅ 指标采集器（独立进程 kafka-collector，高并发采集）
- ✅ 审计日志（记录 + 查询 + 导出）
- ✅ 数据同步 Worker（定时同步 Topic 和 ACL）
- ✅ 用户管理（CRUD + 禁用/启用 + 统计）
- ✅ 主机映射管理（集群感知解析 + 缓存）
- ✅ Topic 权限管理（细粒度权限控制）
- ✅ 前端 Web UI（React + Ant Design + react-grid-layout）
- ✅ 前端生产构建（Vite + 代码分割 + SPA 路由）
- ✅ 静态资源服务（后端集成前端构建产物）
- ✅ 敏感信息加密（AES-256-CFB）
- ✅ API 限流和安全中间件（CSP/HSTS/Referrer-Policy/ResponseSizeLimit/Gzip）
- ✅ Docker 部署支持
- ✅ systemd 服务配置
- ✅ PostgreSQL 14+ 完整支持

**待完善功能**：
- ⏳ 性能测试和优化
- ⏳ API 文档（Swagger）

### v0.1.0 (初始版本)

- ✅ 初始项目结构
- ✅ 基础配置和日志模块
- ✅ 数据库连接池
- ✅ 路由框架