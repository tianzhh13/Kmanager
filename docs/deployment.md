# 部署指南

本文档详细说明 Kafka 管理平台的部署方式和配置。

## 环境要求

### 操作系统
- Linux（推荐）
- Windows
- macOS

### 软件依赖
- **Go**: 1.21+ （编译后端）
- **Node.js**: 18+ （构建前端）
- **数据库**: MySQL 8.0+ 或 PostgreSQL 14+
- **Kafka**: 2.8+ （需要启用 Admin API）
- **Prometheus**: 2.40+ （可选，用于监控功能）

## 部署方式

### 方式一：二进制部署

#### 1. 编译后端

```bash
# 编译 Linux 版本
GOOS=linux GOARCH=amd64 go build -o kafka-management-platform cmd/server/main.go

# 编译 Windows 版本
GOOS=windows GOARCH=amd64 go build -o kafka-management-platform.exe cmd/server/main.go

# 或使用 Makefile
make build
```

#### 2. 前端资源

**推荐方式：使用预编译的前端**

项目已包含预编译的前端资源（`frontend/dist/`），可直接使用，无需额外构建。

```bash
# 确认前端资源存在
ls frontend/dist/
# 应该看到: index.html, assets/, vite.svg
```

**可选方式：自行构建前端**

如果需要修改前端代码或重新构建：

```bash
cd frontend
npm install
npm run build
```

构建产物将输出到 `frontend/dist` 目录。

> **说明**：`frontend/dist/` 是编译后的静态文件，不依赖 `node_modules`，可以独立部署。`node_modules` 仅在开发和构建时需要。

#### 3. 配置应用

```bash
# 复制配置文件
cp configs/config.yaml.example configs/config.yaml

# 编辑配置文件
vim configs/config.yaml
```

**关键配置项**：

```yaml
server:
  port: 8080
  mode: release  # 生产环境使用 release

database:
  type: mysql
  host: localhost
  port: 3306
  username: kafka_admin
  password: your_password
  database: kafka_management

jwt:
  secret: your-jwt-secret-key-at-least-32-characters
  access_token_expire: 3600
  refresh_token_expire: 604800

encryption:
  key: your-base64-encoded-32-byte-key
```

生成加密密钥：

```bash
openssl rand -base64 32
```

#### 4. 初始化数据库

**MySQL**:

```bash
mysql -u root -p < scripts/init_db.sql
```

**PostgreSQL**:

```bash
psql -U postgres -f scripts/init_db_postgres.sql
```

#### 5. 运行应用

```bash
# 直接运行
./kafka-management-platform

# 或使用后台运行
nohup ./kafka-management-platform > app.log 2>&1 &
```

#### 6. 使用 Nginx 托管前端（可选）

如果前后端分离部署，可以使用 Nginx 托管前端静态文件：

```nginx
server {
    listen 80;
    server_name your-domain.com;
    
    # 前端静态文件
    location / {
        root /path/to/frontend/dist;
        try_files $uri $uri/ /index.html;
    }
    
    # API 代理
    location /api {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### 方式二：Docker 部署

#### 1. 构建镜像

```bash
docker build -t kafka-management-platform:latest .
```

#### 2. 运行容器

```bash
docker run -d \
  --name kafka-management-platform \
  -p 8080:8080 \
  -v $(pwd)/configs:/app/configs \
  -e DATABASE_HOST=your-db-host \
  -e DATABASE_PASSWORD=your-db-password \
  kafka-management-platform:latest
```

#### 3. 使用 Docker Compose

```bash
docker-compose up -d
```

`docker-compose.yaml` 示例：

```yaml
version: '3.8'

services:
  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      - DATABASE_HOST=mysql
      - DATABASE_PORT=3306
      - DATABASE_USERNAME=root
      - DATABASE_PASSWORD=password
      - DATABASE_NAME=kafka_management
    depends_on:
      - mysql
    volumes:
      - ./configs:/app/configs

  mysql:
    image: mysql:8.0
    environment:
      - MYSQL_ROOT_PASSWORD=password
      - MYSQL_DATABASE=kafka_management
    volumes:
      - mysql-data:/var/lib/mysql
      - ./scripts/init_db.sql:/docker-entrypoint-initdb.d/init.sql

volumes:
  mysql-data:
```

### 方式三：systemd 服务

#### 1. 创建服务文件

创建 `/etc/systemd/system/kafka-management-platform.service`：

```ini
[Unit]
Description=Kafka Management Platform
After=network.target mysql.service
Wants=mysql.service

[Service]
Type=simple
User=kafka
Group=kafka
WorkingDirectory=/opt/kafka-management-platform
ExecStart=/opt/kafka-management-platform/kafka-management-platform
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=5s

# 环境变量
Environment="GIN_MODE=release"

# 安全配置
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

#### 2. 启动服务

```bash
# 重载 systemd 配置
sudo systemctl daemon-reload

# 启用开机自启
sudo systemctl enable kafka-management-platform

# 启动服务
sudo systemctl start kafka-management-platform

# 查看状态
sudo systemctl status kafka-management-platform
```

#### 3. 管理服务

```bash
# 停止服务
sudo systemctl stop kafka-management-platform

# 重启服务
sudo systemctl restart kafka-management-platform

# 查看日志
sudo journalctl -u kafka-management-platform -f
```

## 生产环境配置建议

### 1. 安全配置

#### 应用配置
```yaml
server:
  mode: release  # 生产模式
  read_timeout: 30
  write_timeout: 30

jwt:
  secret: "use-strong-secret-at-least-32-characters-long"
  access_token_expire: 3600
  refresh_token_expire: 604800

encryption:
  key: "use-strong-encryption-key-32-bytes-base64"
```

#### 网络安全
- 启用 HTTPS（使用 Let's Encrypt 或自签名证书）
- 配置防火墙规则，只开放必要端口
- 使用 VPN 或内网访问

#### 密码安全
- 修改默认管理员密码
- 使用强密码策略
- 定期轮换密钥

### 2. 性能配置

#### 数据库优化
```yaml
database:
  max_open_conns: 50
  max_idle_conns: 10
  conn_max_lifetime: 3600
```

#### 缓存配置（可选）
```yaml
redis:
  host: localhost
  port: 6379
  password: ""
  db: 0
  pool_size: 10
```

#### 连接池
- Kafka 连接池：每个集群维护一个长连接
- 数据库连接池：最大 50 个连接

### 3. 监控配置

#### Prometheus 集成
```yaml
monitoring:
  prometheus_url: "http://prometheus:9090"
  scrape_interval: 15s
```

#### 日志配置
```yaml
log:
  level: info
  format: json
  output_path: /var/log/kafka-management-platform.log
```

#### 告警规则
- 应用健康检查失败
- 数据库连接数过高
- Kafka 连接异常
- API 响应时间过长

### 4. 备份策略

#### 数据库备份
```bash
# MySQL 备份
mysqldump -u root -p kafka_management > backup_$(date +%Y%m%d).sql

# PostgreSQL 备份
pg_dump -U postgres kafka_management > backup_$(date +%Y%m%d).sql
```

#### 配置备份
- 定期备份 `configs/` 目录
- 备份加密密钥和 JWT Secret

## Web UI 使用指南

### 访问前端

#### 开发环境

```bash
# 启动后端
go run cmd/server/main.go

# 启动前端开发服务器
cd frontend
npm install
npm run dev
```

访问 `http://localhost:3000`

#### 生产环境

后端已集成静态资源服务，直接访问 `http://localhost:8080` 即可。

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
| 用户管理 | `/users` | 用户列表、创建、编辑、禁用（需要超��管理员权限） |

### 默认管理员账户

- **用户名**: `admin`
- **密码**: `admin123`
- **角色**: 超级管理员

⚠️ **重要**: 首次登录后请立即修改默认密码！

### 前端技术栈

- **框架**: React 18 + TypeScript
- **UI 组件**: Ant Design 5
- **状态管理**: Redux Toolkit
- **路由**: React Router 6
- **HTTP 客户端**: Axios
- **图表**: ECharts
- **构建工具**: Vite

### 前端构建

#### 开发环境

```bash
cd frontend
npm install
npm run dev
```

前端开发服务器将在 `http://localhost:3000` 启动，并自动代理 API 请求到后端。

#### 生产构建

```bash
cd frontend
npm run build
```

构建产物将输出到 `frontend/dist` 目录。

#### 集成部署

后端已配置静态资源服务，可以直接服务前端构建产物：

```bash
# 1. 构建前端
cd frontend
npm run build

# 2. 启动后端（自动服务前端静态文件）
cd ..
go run cmd/server/main.go
```

访问 `http://localhost:8080` 即可使用完整应用。

**特性**：
- ✅ SPA 路由支持（刷新页面正常工作）
- ✅ 静态资源优化（代码分割、压缩）
- ✅ API 路由隔离（/api 路径返回 JSON 错误）

## 升级指南

### 版本升级

1. **备份数据库**
   ```bash
   mysqldump -u root -p kafka_management > backup_before_upgrade.sql
   ```

2. **停止服务**
   ```bash
   sudo systemctl stop kafka-management-platform
   ```

3. **更新代码**
   ```bash
   git pull origin main
   ```

4. **更新依赖**
   ```bash
   go mod tidy
   cd frontend && npm install
   ```

5. **重新构建**
   ```bash
   make build
   cd frontend && npm run build
   ```

6. **运行迁移**
   - 应用启动时会自动运行数据库迁移

7. **启动服务**
   ```bash
   sudo systemctl start kafka-management-platform
   ```

### 配置迁移

如果配置文件格式有变化，参考 `configs/config.yaml.example` 更新配置。

## 常见问题

### 1. 端口被占用

**问题**: 启动时报错 `bind: address already in use`

**解决**:
```bash
# 查看端口占用
lsof -i :8080

# 修改配置文件中的端口
vim configs/config.yaml
```

### 2. 数据库连接失败

**问题**: 启动时报错 `failed to connect to database`

**解决**:
- 检查数据库服务是否运行
- 验证配置文件中的数据库连接信息
- 检查数据库用户权限
- 确认数据库已创建

### 3. 前端无法访问

**问题**: 访问前端显示空白或 404

**解决**:
- 确认前端已构建：`ls frontend/dist`
- 检查后端静态资源服务配置
- 查看浏览器控制台错误信息

### 4. Kafka 连接失败

**问题**: 测试集群连接失败

**解决**:
- 检查 Kafka 集群是否运行
- 验证 Bootstrap Servers 地址和端口
- 检查认证配置是否正确
- 查看应用日志获取详细错误信息

更多问题请参考 [故障排查文档](troubleshooting.md)。