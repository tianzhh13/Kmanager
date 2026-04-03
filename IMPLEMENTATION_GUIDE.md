# Kafka 管理平台 - 快速实现指南

## 当前状态

✅ **已完成**（任务 1-5）：
- 项目基础设施
- 数据库设计和 Repository 层
- 加密服务
- 认证授权服务
- 集群管理服务（部分）

## 快速实现路线图

### 阶段 1：完成核心后端服务（任务 6-8）

#### 任务 6：集群管理服务 ✅ 部分完成
- ✅ 6.1 集群 CRUD 操作
- ⏳ 6.2 集群连接测试（需要 Kafka 客户端）
- ⏳ 6.3 集群权限管理（已在 6.1 中实现）

**下一步**：实现 Kafka Admin 客户端工厂（任务 7.1）

#### 任务 7：Topic 管理服务
需要文件：
- `pkg/kafka/admin_client.go` - Kafka Admin 客户端封装
- `internal/service/topic/topic_service.go` - Topic 管理服务

#### 任务 8：ACL 管理服务
需要文件：
- `internal/service/acl/acl_service.go` - ACL 管理服务

### 阶段 2：API 层和中间件（任务 13）

需要文件：
- `internal/middleware/auth.go` - JWT 认证中间件
- `internal/middleware/permission.go` - 权限验证中间件
- `internal/middleware/cors.go` - CORS 中间件
- `internal/handler/auth_handler.go` - 认证 API
- `internal/handler/cluster_handler.go` - 集群 API
- `internal/handler/topic_handler.go` - Topic API
- `internal/handler/acl_handler.go` - ACL API

### 阶段 3：前端开发（任务 19-23）

#### 前端项目结构
```
frontend/
├── src/
│   ├── pages/
│   │   ├── Login.tsx
│   │   ├── ClusterList.tsx
│   │   ├── TopicList.tsx
│   │   └── ACLList.tsx
│   ├── components/
│   │   ├── Layout.tsx
│   │   └── PrivateRoute.tsx
│   ├── services/
│   │   ├── api.ts
│   │   ├── auth.ts
│   │   └── cluster.ts
│   ├── store/
│   │   └── authSlice.ts
│   └── App.tsx
├── package.json
└── vite.config.ts
```

## 最小可用产品（MVP）实现

为了快速看到效果，建议按以下顺序实现：

### 1. 完成认证 API（30 分钟）

创建 `internal/handler/auth_handler.go`：

```go
package handler

import (
	"github.com/gin-gonic/gin"
	"kafka-management-platform/internal/service/auth"
)

type AuthHandler struct {
	authSvc *auth.Service
}

func NewAuthHandler(authSvc *auth.Service) *AuthHandler {
	return &AuthHandler{authSvc: authSvc}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req auth.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.authSvc.Login(c.Request.Context(), &req)
	if err != nil {
		c.JSON(401, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, resp)
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.authSvc.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		c.JSON(401, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, resp)
}
```

### 2. 更新路由（10 分钟）

修改 `internal/router/router.go`，添加真实的 handler。

### 3. 创建简单前端（1 小时）

使用 Vite + React 创建最小前端：

```bash
cd frontend
npm create vite@latest . -- --template react-ts
npm install antd axios react-router-dom @reduxjs/toolkit react-redux
```

创建登录页面和集群列表页面。

### 4. 测试完整流程（30 分钟）

- 启动后端
- 启动前端
- 测试登录
- 测试集群列表

## 简化实现建议

### 方案 A：仅后端 API（推荐用于快速验证）

1. 完成所有 Handler 和 API 路由
2. 使用 Postman 或 curl 测试
3. 暂时跳过前端

**优点**：
- 快速验证后端逻辑
- 专注于核心功能
- 易于调试

### 方案 B：前后端分离开发

1. 后端提供完整 API
2. 前端独立开发
3. 使用 Mock 数据测试前端

**优点**：
- 并行开发
- 前后端解耦
- 灵活调整

### 方案 C：全栈快速原型

1. 使用模板引擎（如 Go template）
2. 服务端渲染简单页面
3. 快速看到效果

**优点**：
- 最快看到效果
- 无需前端构建
- 适合演示

## 关键代码模板

### Kafka Admin 客户端

```go
package kafka

import (
	"github.com/IBM/sarama"
	"kafka-management-platform/internal/models"
)

type AdminClient struct {
	client sarama.ClusterAdmin
}

func NewAdminClient(cluster *models.Cluster) (*AdminClient, error) {
	config := sarama.NewConfig()
	config.Version = sarama.V2_8_0_0
	
	// 根据认证类型配置
	switch cluster.AuthType {
	case models.AuthTypePlaintext:
		// 无需额外配置
	case models.AuthTypeSCRAM:
		// 配置 SCRAM 认证
	case models.AuthTypeKerberos:
		// 配置 Kerberos 认证
	}
	
	client, err := sarama.NewClusterAdmin(
		strings.Split(cluster.BootstrapServers, ","),
		config,
	)
	if err != nil {
		return nil, err
	}
	
	return &AdminClient{client: client}, nil
}

func (c *AdminClient) CreateTopic(name string, partitions int32, replicationFactor int16) error {
	return c.client.CreateTopic(name, &sarama.TopicDetail{
		NumPartitions:     partitions,
		ReplicationFactor: replicationFactor,
	}, false)
}

func (c *AdminClient) ListTopics() (map[string]sarama.TopicDetail, error) {
	return c.client.ListTopics()
}

func (c *AdminClient) DeleteTopic(name string) error {
	return c.client.DeleteTopic(name)
}

func (c *AdminClient) Close() error {
	return c.client.Close()
}
```

### JWT 认证中间件

```go
package middleware

import (
	"strings"
	"github.com/gin-gonic/gin"
	"kafka-management-platform/pkg/jwt"
)

func AuthMiddleware(jwtSvc *jwt.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 Header 获取 Token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(401, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		// 解析 Bearer Token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(401, gin.H{"error": "invalid authorization header"})
			c.Abort()
			return
		}

		// 验证 Token
		claims, err := jwtSvc.ValidateToken(parts[1])
		if err != nil {
			c.JSON(401, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		// 将用户信息存入 Context
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)

		c.Next()
	}
}
```

### 前端登录页面

```typescript
// src/pages/Login.tsx
import React, { useState } from 'react';
import { Form, Input, Button, message } from 'antd';
import { useNavigate } from 'react-router-dom';
import axios from 'axios';

const Login: React.FC = () => {
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();

  const onFinish = async (values: any) => {
    setLoading(true);
    try {
      const response = await axios.post('http://localhost:8080/api/v1/auth/login', values);
      localStorage.setItem('access_token', response.data.access_token);
      localStorage.setItem('refresh_token', response.data.refresh_token);
      message.success('登录成功');
      navigate('/clusters');
    } catch (error) {
      message.error('登录失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ maxWidth: 400, margin: '100px auto' }}>
      <h1>Kafka 管理平台</h1>
      <Form onFinish={onFinish}>
        <Form.Item name="username" rules={[{ required: true }]}>
          <Input placeholder="用户名" />
        </Form.Item>
        <Form.Item name="password" rules={[{ required: true }]}>
          <Input.Password placeholder="密码" />
        </Form.Item>
        <Form.Item>
          <Button type="primary" htmlType="submit" loading={loading} block>
            登录
          </Button>
        </Form.Item>
      </Form>
    </div>
  );
};

export default Login;
```

## 下一步行动

### 立即可做（1-2 小时）

1. ✅ 创建 Auth Handler
2. ✅ 创建 JWT 中间件
3. ✅ 更新路由，连接真实 API
4. ✅ 测试登录 API

### 短期目标（1 天）

1. 完成 Kafka Admin 客户端
2. 完成 Topic 和 ACL 服务
3. 完成所有 API Handler
4. 使用 Postman 测试所有 API

### 中期目标（2-3 天）

1. 创建前端项目
2. 实现登录页面
3. 实现集群列表页面
4. 实现 Topic 管理页面

## 测试命令

```bash
# 测试登录
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'

# 测试集群列表（需要 Token）
curl http://localhost:8080/api/v1/clusters \
  -H "Authorization: Bearer YOUR_TOKEN"
```

## 需要帮助？

如果您需要：
1. 完整实现某个具体功能
2. 调试特定问题
3. 优化代码结构

请告诉我具体需求，我会提供详细的实现代码！
