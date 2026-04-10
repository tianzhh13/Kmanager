# 开发指南

本文档面向开发者，说明如何参与 Kafka 管理平台的开发和扩展。

## 开发环境搭建

### 前置要求

- **Go**: 1.21+
- **Node.js**: 18+
- **MySQL**: 8.0+ 或 PostgreSQL 14+
- **Git**: 版本控制
- **IDE**: VS Code（推荐）或 GoLand

### 安装开发工具

```bash
# 安装 Go
# macOS
brew install go

# Linux
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz

# 安装 Node.js
# macOS
brew install node

# Linux
curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
sudo apt-get install -y nodejs

# 安装 golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# 安装 air（热重载）
go install github.com/cosmtrek/air@latest
```

### 克隆项目

```bash
git clone <repository-url>
cd kafka-management-platform
```

### 安装依赖

```bash
# Go 依赖
go mod tidy
go mod vendor

# 前端依赖
cd frontend
npm install
```

### 配置开发环境

```bash
# 复制配置文件
cp configs/config.yaml.example configs/config.yaml

# 编辑配置文件
vim configs/config.yaml
```

### 启动开发服务器

#### 方式一：直接运行

```bash
# 启动后端
go run cmd/server/main.go

# 启动前端（新终端）
cd frontend
npm run dev
```

#### 方式二：使用热重载

```bash
# 安装 air
go install github.com/cosmtrek/air@latest

# 运行
air
```

创建 `.air.toml` 配置文件：

```toml
root = "."
tmp_dir = "tmp"

[build]
  bin = "./tmp/main"
  cmd = "go build -o ./tmp/main ./cmd/server"
  delay = 1000
  exclude_dir = ["assets", "tmp", "vendor", "frontend/node_modules"]
  exclude_file = []
  exclude_regex = ["_test.go"]
  exclude_unchanged = false
  follow_symlink = false
  include_dir = []
  include_file = []
  kill_delay = "0s"
  log = "build-errors.log"
  send_interrupt = false
  stop_on_error = true

[color]
  build = "yellow"
  main = "magenta"
  runner = "green"
  watcher = "cyan"

[log]
  time = false

[misc]
  clean_on_exit = true
```

## 项目架构

### 整体架构

```
┌─────────────────────────────────────────────────────────┐
│                      前端 (React)                        │
│                  http://localhost:3000                   │
└─────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────┐
│                    后端 API (Go + Gin)                   │
│                  http://localhost:8080                   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────┐ │
│  │ Handler  │  │Middleware│  │ Service  │  │Repository│ │
│  └──────────┘  └──────────┘  └──────────┘  └─────────┘ │
└─────────────────────────────────────────────────────────┘
        │                 │                 │
        ▼                 ▼                 ▼
┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│   MySQL/PG   │  │Kafka 集群(s) │  │  Prometheus  │
└──────────────┘  └──────────────┘  └──────────────┘
```

### 目录结构

```
kafka-management-platform/
├── cmd/                        # 应用入口
│   └── server/
│       └── main.go            # 主程序
├── internal/                   # 内部代码（不对外暴露）
│   ├── config/                # 配置管理
│   ├── database/              # 数据库连接
│   ├── logger/                # 日志模块
│   ├── models/                # 数据模型
│   ├── repository/            # 数据访问层
│   ├── service/               # 业务逻辑层
│   │   ├── auth/             # 认证服务
│   │   ├── cluster/          # 集群服务
│   │   ├── topic/            # Topic 服务
│   │   ├── acl/              # ACL 服务
│   │   ├── monitor/          # 监控服务
│   │   ├── audit/            # 审计服务
│   │   └── user/             # 用户服务
│   ├── handler/               # HTTP 处理器
│   ├── middleware/            # 中间件
│   ├── router/                # 路由配置
│   └── worker/                # 后台任务
├── pkg/                        # 公共包（可对外暴露）
│   ├── encryption/            # 加密工具
│   ├── jwt/                   # JWT 工具
│   ├── password/              # 密码工具
│   ├── kafka/                 # Kafka 客户端
│   ├── prometheus/            # Prometheus 客户端
│   └── validator/             # 验证器
├── configs/                    # 配置文件
├── frontend/                   # 前端项目
│   ├── src/
│   │   ├── pages/             # 页面组件
│   │   ├── components/        # 通用组件
│   │   ├── services/          # API 服务
│   │   └── store/             # 状态管理
│   └── package.json
├── scripts/                    # 脚本
├── docs/                       # 文档
├── vendor/                     # 依赖
├── go.mod
└── README.md
```

### 分层架构

#### 1. Handler 层（HTTP 处理器）

负责处理 HTTP 请求和响应，参数验证，调用 Service 层。

```go
// internal/handler/cluster_handler.go
package handler

type ClusterHandler struct {
    clusterSvc *cluster.Service
}

func NewClusterHandler(clusterSvc *cluster.Service) *ClusterHandler {
    return &ClusterHandler{clusterSvc: clusterSvc}
}

func (h *ClusterHandler) CreateCluster(c *gin.Context) {
    var req cluster.CreateClusterRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    cluster, err := h.clusterSvc.CreateCluster(c.Request.Context(), &req)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(201, cluster)
}
```

#### 2. Service 层（业务逻辑）

负责业务逻辑处理，事务管理，调用 Repository 层。

```go
// internal/service/cluster/cluster_service.go
package cluster

type Service struct {
    clusterRepo repository.ClusterRepository
    clusterUserRepo repository.ClusterUserRepository
    encryptionSvc *encryption.Service
}

func NewService(
    clusterRepo repository.ClusterRepository,
    clusterUserRepo repository.ClusterUserRepository,
    encryptionSvc *encryption.Service,
) *Service {
    return &Service{
        clusterRepo: clusterRepo,
        clusterUserRepo: clusterUserRepo,
        encryptionSvc: encryptionSvc,
    }
}

func (s *Service) CreateCluster(ctx context.Context, req *CreateClusterRequest) (*models.Cluster, error) {
    // 业务逻辑
    // 1. 验证参数
    // 2. 加密敏感信息
    // 3. 保存到数据库
    // 4. 返回结果
}
```

#### 3. Repository 层（数据访问）

负责数据库操作，CRUD 操作。

```go
// internal/repository/cluster_repository.go
package repository

type ClusterRepository interface {
    Create(ctx context.Context, cluster *models.Cluster) error
    Update(ctx context.Context, cluster *models.Cluster) error
    Delete(ctx context.Context, id uint) error
    GetByID(ctx context.Context, id uint) (*models.Cluster, error)
    List(ctx context.Context, page, pageSize int) ([]*models.Cluster, int64, error)
}

type clusterRepository struct {
    db *gorm.DB
}

func NewClusterRepository(db *gorm.DB) ClusterRepository {
    return &clusterRepository{db: db}
}
```

## 添加新功能

### 示例：添加"消息查询"功能

#### 1. 定义数据模型

```go
// internal/models/message.go
package models

import "time"

type Message struct {
    ID        uint      `gorm:"primaryKey" json:"id"`
    ClusterID uint      `gorm:"not null;index" json:"cluster_id"`
    Topic     string    `gorm:"size:255;not null;index" json:"topic"`
    Partition int32     `gorm:"not null" json:"partition"`
    Offset    int64     `gorm:"not null" json:"offset"`
    Key       string    `gorm:"type:text" json:"key"`
    Value     string    `gorm:"type:text" json:"value"`
    Timestamp time.Time `json:"timestamp"`
    CreatedAt time.Time `json:"created_at"`
    
    Cluster   Cluster   `gorm:"foreignKey:ClusterID" json:"cluster"`
}

func (Message) TableName() string {
    return "messages"
}
```

#### 2. 实现 Repository 层

```go
// internal/repository/message_repository.go
package repository

import (
    "context"
    "kafka-management-platform/internal/models"
    
    "gorm.io/gorm"
)

type MessageRepository interface {
    Create(ctx context.Context, message *models.Message) error
    GetByID(ctx context.Context, id uint) (*models.Message, error)
    List(ctx context.Context, clusterID uint, topic string, page, pageSize int) ([]*models.Message, int64, error)
}

type messageRepository struct {
    db *gorm.DB
}

func NewMessageRepository(db *gorm.DB) MessageRepository {
    return &messageRepository{db: db}
}

func (r *messageRepository) Create(ctx context.Context, message *models.Message) error {
    return r.db.WithContext(ctx).Create(message).Error
}

func (r *messageRepository) List(ctx context.Context, clusterID uint, topic string, page, pageSize int) ([]*models.Message, int64, error) {
    var messages []*models.Message
    var total int64
    
    query := r.db.WithContext(ctx).Model(&models.Message{}).Where("cluster_id = ?", clusterID)
    if topic != "" {
        query = query.Where("topic = ?", topic)
    }
    
    if err := query.Count(&total).Error; err != nil {
        return nil, 0, err
    }
    
    offset := (page - 1) * pageSize
    if err := query.Offset(offset).Limit(pageSize).Order("timestamp DESC").Find(&messages).Error; err != nil {
        return nil, 0, err
    }
    
    return messages, total, nil
}
```

#### 3. 实现 Service 层

```go
// internal/service/message/message_service.go
package message

import (
    "context"
    "kafka-management-platform/internal/models"
    "kafka-management-platform/internal/repository"
)

type Service struct {
    messageRepo repository.MessageRepository
    clusterRepo repository.ClusterRepository
}

func NewService(
    messageRepo repository.MessageRepository,
    clusterRepo repository.ClusterRepository,
) *Service {
    return &Service{
        messageRepo: messageRepo,
        clusterRepo: clusterRepo,
    }
}

type ListMessagesRequest struct {
    ClusterID uint   `json:"cluster_id"`
    Topic     string `json:"topic"`
    Page      int    `json:"page"`
    PageSize  int    `json:"page_size"`
}

func (s *Service) ListMessages(ctx context.Context, req *ListMessagesRequest) ([]*models.Message, int64, error) {
    // 验证权限
    // 查询消息
    return s.messageRepo.List(ctx, req.ClusterID, req.Topic, req.Page, req.PageSize)
}
```

#### 4. 实现 Handler 层

```go
// internal/handler/message_handler.go
package handler

import (
    "kafka-management-platform/internal/service/message"
    
    "github.com/gin-gonic/gin"
)

type MessageHandler struct {
    messageSvc *message.Service
}

func NewMessageHandler(messageSvc *message.Service) *MessageHandler {
    return &MessageHandler{messageSvc: messageSvc}
}

func (h *MessageHandler) ListMessages(c *gin.Context) {
    var req message.ListMessagesRequest
    if err := c.ShouldBindQuery(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    if req.Page == 0 {
        req.Page = 1
    }
    if req.PageSize == 0 {
        req.PageSize = 20
    }
    
    messages, total, err := h.messageSvc.ListMessages(c.Request.Context(), &req)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(200, gin.H{
        "messages": messages,
        "total":    total,
        "page":     req.Page,
        "page_size": req.PageSize,
    })
}
```

#### 5. 注册路由

```go
// internal/router/router.go
func Setup(cfg *config.Config, db *gorm.DB) *gin.Engine {
    // ... 现有代码 ...
    
    // 初始化 Message Repository
    messageRepo := repository.NewMessageRepository(db)
    
    // 初始化 Message Service
    messageSvc := message.NewService(messageRepo, clusterRepo)
    
    // 初始化 Message Handler
    messageHandler := handler.NewMessageHandler(messageSvc)
    
    // 注册路由
    messages := authenticated.Group("/messages")
    {
        messages.GET("", clusterPermissionMiddleware.RequireClusterAccess(), messageHandler.ListMessages)
    }
    
    // ... 现有代码 ...
}
```

#### 6. 添加数据库迁移

```go
// internal/database/migrate.go
func AutoMigrate(db *gorm.DB) error {
    return db.AutoMigrate(
        &models.User{},
        &models.Cluster{},
        &models.ClusterUserRelation{},
        &models.Topic{},
        &models.ACL{},
        &models.AuditLog{},
        &models.Message{}, // 新增
    )
}
```

## 运行测试

### 单元测试

```bash
# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./internal/service/auth/...

# 运行测试并显示覆盖率
go test -cover ./...

# 生成覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### 集成测试

```bash
# 运行集成测试
go test ./internal/tests/... -v

# 运行端到端测试
go test ./internal/tests/e2e_test.go -v
```

### 前端测试

```bash
cd frontend
npm run test
```

## 代码规范

### Go 代码规范

1. **格式化**
   ```bash
   # 格式化代码
   gofmt -w .
   
   # 或使用 goimports
   goimports -w .
   ```

2. **代码检查**
   ```bash
   # 运行 golangci-lint
   golangci-lint run
   
   # 检查特定文件
   golangci-lint run internal/service/auth/auth_service.go
   ```

3. **命名规范**
   - 包名��小写单词，不使用下划线
   - 导出函数：大写字母开头
   - 私有函数：小写字母开头
   - 接口：动词或形容词 + er（如 `Writer`, `Stringer`）

4. **注释规范**
   ```go
   // CreateCluster 创建一个新的 Kafka 集群配置。
   // 它会验证参数，加密敏感信息，并保存到数据库。
   //
   // 参数：
   //   - ctx: 上下文
   //   - req: 创建集群请求
   //
   // 返回：
   //   - *models.Cluster: 创建的集群
   //   - error: 错误信息
   func (s *Service) CreateCluster(ctx context.Context, req *CreateClusterRequest) (*models.Cluster, error) {
       // 实现
   }
   ```

### 前端代码规范

1. **格式化**
   ```bash
   cd frontend
   npm run lint
   ```

2. **命名规范**
   - 组件：PascalCase（如 `ClusterList.tsx`）
   - 函数：camelCase（如 `fetchClusters`）
   - 常量：UPPER_SNAKE_CASE（如 `API_BASE_URL`）

3. **组件结构**
   ```typescript
   // 导入
   import React, { useState, useEffect } from 'react';
   import { Table, Button } from 'antd';
   
   // 类型定义
   interface ClusterListProps {
     // props
   }
   
   // 组件
   const ClusterList: React.FC<ClusterListProps> = () => {
     // 状态
     const [clusters, setClusters] = useState([]);
     
     // 副作用
     useEffect(() => {
       // fetch data
     }, []);
     
     // 事件处理
     const handleCreate = () => {
       // handle create
     };
     
     // 渲染
     return (
       <div>
         {/* JSX */}
       </div>
     );
   };
   
   // 导出
   export default ClusterList;
   ```

## Git 提交规范

### 提交消息格式

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Type 类型

- `feat`: 新功能
- `fix`: 修复 bug
- `docs`: 文档更新
- `style`: 代码格式（不影响功能）
- `refactor`: 重构
- `test`: 测试
- `chore`: 构建/工具链

### 示例

```bash
# 新功能
git commit -m "feat(message): add message query feature"

# 修复 bug
git commit -m "fix(auth): fix JWT token validation error"

# 文档更新
git commit -m "docs(api): update API documentation"

# 重构
git commit -m "refactor(cluster): optimize cluster connection logic"
```

## 发布流程

### 1. 创建发布分支

```bash
git checkout -b release/v0.4.0
```

### 2. 更新版本号

- 更新 `README.md` 中的版本号
- 更新 `CHANGELOG.md`

### 3. 测试

```bash
# 运行所有测试
make test

# 构建前端
cd frontend && npm run build

# 构建后端
make build
```

### 4. 打标签

```bash
git tag -a v0.4.0 -m "Release v0.4.0"
git push origin v0.4.0
```

### 5. 构建 Docker 镜像

```bash
docker build -t kafka-management-platform:v0.4.0 .
docker push your-registry/kafka-management-platform:v0.4.0
```

## 常见问题

### 1. 依赖冲突

**问题**: `go mod tidy` 报错

**解决**:
```bash
# 清理缓存
go clean -modcache

# 重新下载
go mod download
go mod tidy
```

### 2. 数据库迁移失败

**问题**: AutoMigrate 失败

**解决**:
- 检查数据库连接
- 检查模型定义
- 手动执行 SQL

### 3. 前端构建失败

**问题**: `npm run build` 报错

**解决**:
```bash
# 清理 node_modules
rm -rf node_modules package-lock.json

# 重新安装
npm install
```

## 资源链接

- [Go 官方文档](https://golang.org/doc/)
- [Gin 框架文档](https://gin-gonic.com/docs/)
- [GORM 文档](https://gorm.io/docs/)
- [React 官方文档](https://react.dev/)
- [Ant Design 文档](https://ant.design/docs/react/introduce)
- [Kafka 官方文档](https://kafka.apache.org/documentation/)