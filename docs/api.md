# API 文档

本文档详细说明 Kafka 管理平台的所有 API 端点和使用方法。

## API 基础信息

- **Base URL**: `http://localhost:8080/api/v1`
- **认证方式**: JWT Bearer Token
- **请求格式**: JSON
- **响应格式**: JSON

## 认证

所有 API 请求（除登录接口外）都需要在请求头中携带 JWT Token：

```
Authorization: Bearer YOUR_ACCESS_TOKEN
```

## 通用响应格式

### 成功响应

```json
{
  "code": 200,
  "message": "success",
  "data": { ... }
}
```

### 错误响应

```json
{
  "code": 400,
  "message": "error message",
  "error": "detailed error information"
}
```

## 认证 API

### 登录

用户登录并获取 JWT Token。

**请求**:
```
POST /api/v1/auth/login
```

**请求体**:
```json
{
  "username": "admin",
  "password": "admin123"
}
```

**响应示例**:
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

**示例**:
```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "admin123"
  }'
```

### 刷新 Token

使用 Refresh Token 获取新的 Access Token。

**请求**:
```
POST /api/v1/auth/refresh
```

**请求体**:
```json
{
  "refresh_token": "your-refresh-token"
}
```

**响应示例**:
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_in": 3600
}
```

### 获取当前用户信息

获取当前登录用户的详细信息。

**请求**:
```
GET /api/v1/auth/me
```

**响应示例**:
```json
{
  "user_id": 1,
  "username": "admin",
  "email": "admin@example.com",
  "role": "super_admin",
  "status": "active",
  "created_at": "2024-01-01T00:00:00Z"
}
```

**示例**:
```bash
curl http://localhost:8080/api/v1/auth/me \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

## 用户管理 API

> 需要"超级管理员"权限

### 获取用户列表

**请求**:
```
GET /api/v1/users?page=1&page_size=20&search=admin
```

**查询参数**:
- `page`: 页码（默认 1）
- `page_size`: 每页数量（默认 20）
- `search`: 搜索关键词（可选）

**响应示例**:
```json
{
  "users": [
    {
      "id": 1,
      "username": "admin",
      "email": "admin@example.com",
      "role": "super_admin",
      "status": "active",
      "created_at": "2024-01-01T00:00:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "page_size": 20
}
```

### 创建用户

**请求**:
```
POST /api/v1/users
```

**请求体**:
```json
{
  "username": "newuser",
  "password": "password123",
  "email": "user@example.com",
  "role": "cluster_admin"
}
```

### 获取用户详情

**请求**:
```
GET /api/v1/users/:id
```

### 更新用户

**请求**:
```
PUT /api/v1/users/:id
```

**请求体**:
```json
{
  "email": "newemail@example.com",
  "role": "read_only"
}
```

### 删除用户

**请求**:
```
DELETE /api/v1/users/:id
```

### 修改用户密码

**请求**:
```
PUT /api/v1/users/:id/password
```

**请求体**:
```json
{
  "new_password": "newpassword123"
}
```

### 禁用用户

**请求**:
```
POST /api/v1/users/:id/disable
```

### 启用用户

**请求**:
```
POST /api/v1/users/:id/enable
```

## 集群管理 API

### 获取集群列表

**请求**:
```
GET /api/v1/clusters
```

**响应示例**:
```json
{
  "clusters": [
    {
      "id": 1,
      "cluster_name": "生产集群",
      "bootstrap_servers": "kafka1:9092,kafka2:9092",
      "auth_type": "plaintext",
      "prometheus_url": "http://prometheus:9090",
      "description": "生产环境 Kafka 集群",
      "status": "active",
      "created_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

**示例**:
```bash
curl http://localhost:8080/api/v1/clusters \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

### 创建集群

**请求**:
```
POST /api/v1/clusters
```

**请求体（PLAINTEXT 认证）**:
```json
{
  "cluster_name": "测试集群",
  "bootstrap_servers": "localhost:9092",
  "auth_type": "plaintext",
  "prometheus_url": "http://localhost:9090",
  "description": "开发环境测试集群"
}
```

**请求体（SCRAM 认证）**:
```json
{
  "cluster_name": "生产集群",
  "bootstrap_servers": "kafka1:9092,kafka2:9092",
  "auth_type": "scram",
  "auth_config": {
    "username": "kafka-user",
    "password": "kafka-password",
    "mechanism": "SCRAM-SHA-256"
  },
  "prometheus_url": "http://prometheus:9090",
  "description": "生产环境 Kafka 集群"
}
```

**请求体（Kerberos 认证）**:
```json
{
  "cluster_name": "安全集群",
  "bootstrap_servers": "kafka1:9094",
  "auth_type": "kerberos",
  "auth_config": {
    "principal": "kafka-client@EXAMPLE.COM",
    "keytab": "/path/to/kafka-client.keytab",
    "realm": "EXAMPLE.COM",
    "service_name": "kafka"
  },
  "description": "Kerberos 认证集群"
}
```

**示例**:
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

### 获取集群详情

**请求**:
```
GET /api/v1/clusters/:id
```

### 更新集群

**请求**:
```
PUT /api/v1/clusters/:id
```

**请求体**:
```json
{
  "description": "更新后的描述",
  "prometheus_url": "http://new-prometheus:9090"
}
```

### 删除集群

**请求**:
```
DELETE /api/v1/clusters/:id
```

### 测试集群连接

**请求**:
```
POST /api/v1/clusters/:id/test
```

**响应示例（成功）**:
```json
{
  "message": "connection test successful"
}
```

**响应示例（失败）**:
```json
{
  "error": "connection test failed",
  "details": "kafka: client has run out of available brokers to talk to"
}
```

**示例**:
```bash
curl -X POST http://localhost:8080/api/v1/clusters/1/test \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

### 授予用户集群权限

**请求**:
```
POST /api/v1/clusters/:id/grant
```

**请求体**:
```json
{
  "user_id": 2
}
```

**示例**:
```bash
curl -X POST http://localhost:8080/api/v1/clusters/1/grant \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"user_id": 2}'
```

### 撤销用户集群权限

**请求**:
```
POST /api/v1/clusters/:id/revoke
```

**请求体**:
```json
{
  "user_id": 2
}
```

### 获取集群授权用户列表

**请求**:
```
GET /api/v1/clusters/:id/users
```

## Topic 管理 API

### 获取 Topic 列表

**请求**:
```
GET /api/v1/topics?cluster_id=1&page=1&page_size=20&search=test
```

**查询参数**:
- `cluster_id`: 集群 ID（必需）
- `page`: 页码（默认 1）
- `page_size`: 每页数量（默认 20）
- `search`: 搜索关键词（可选）

**响应示例**:
```json
{
  "topics": [
    {
      "id": 1,
      "cluster_id": 1,
      "topic_name": "test-topic",
      "partitions": 3,
      "replication_factor": 1,
      "config": {
        "retention.ms": "604800000",
        "cleanup.policy": "delete"
      },
      "created_at": "2024-01-01T00:00:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "page_size": 20
}
```

**示例**:
```bash
curl "http://localhost:8080/api/v1/topics?cluster_id=1" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

### 创建 Topic

**请求**:
```
POST /api/v1/topics
```

**请求体**:
```json
{
  "cluster_id": 1,
  "topic_name": "test-topic",
  "partitions": 3,
  "replication_factor": 1,
  "config": {
    "retention.ms": "604800000",
    "cleanup.policy": "delete"
  }
}
```

**示例**:
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

### 获取 Topic 详情

**请求**:
```
GET /api/v1/topics/:name?cluster_id=1
```

### 删除 Topic

**请求**:
```
DELETE /api/v1/topics/:name?cluster_id=1
```

### 更新 Topic 配置

**请求**:
```
PUT /api/v1/topics/:name/config
```

**请求体**:
```json
{
  "cluster_id": 1,
  "config": {
    "retention.ms": "86400000",
    "cleanup.policy": "compact"
  }
}
```

### 同步 Topic 数据

从 Kafka 集群同步 Topic 元数据到数据库。

**请求**:
```
POST /api/v1/topics/sync/:id
```

**示例**:
```bash
curl -X POST http://localhost:8080/api/v1/topics/sync/1 \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

## ACL 管理 API

### 获取 ACL 列表

**请求**:
```
GET /api/v1/acls?cluster_id=1&resource_type=topic&principal=User:test
```

**查询参数**:
- `cluster_id`: 集群 ID（必需）
- `resource_type`: 资源类型（可选）
- `resource_name`: 资源名称（可选）
- `principal`: Principal（可选）
- `page`: 页码（默认 1）
- `page_size`: 每页数量（默认 20）

**响应示例**:
```json
{
  "acls": [
    {
      "id": 1,
      "cluster_id": 1,
      "resource_type": "topic",
      "resource_name": "test-topic",
      "resource_pattern": "literal",
      "principal": "User:test-user",
      "host": "*",
      "operation": "read",
      "permission_type": "allow",
      "created_at": "2024-01-01T00:00:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "page_size": 20
}
```

**示例**:
```bash
curl "http://localhost:8080/api/v1/acls?cluster_id=1" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

### 创建 ACL 规则

**请求**:
```
POST /api/v1/acls
```

**请求体**:
```json
{
  "cluster_id": 1,
  "resource_type": "topic",
  "resource_name": "test-topic",
  "resource_pattern": "literal",
  "principal": "User:test-user",
  "host": "*",
  "operation": "read",
  "permission_type": "allow"
}
```

**示例**:
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

### 删除 ACL 规则

**请求**:
```
DELETE /api/v1/acls/:id
```

### 批量删除 ACL

**请求**:
```
POST /api/v1/acls/batch-delete
```

**请求体**:
```json
{
  "acl_ids": [1, 2, 3]
}
```

**示例**:
```bash
curl -X POST http://localhost:8080/api/v1/acls/batch-delete \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"acl_ids": [1, 2, 3]}'
```

### 同步 ACL 数据

从 Kafka 集群同步 ACL 规则到数据库。

**请求**:
```
POST /api/v1/acls/sync/:id
```

## 监控 API

### 获取集群监控指标

**请求**:
```
GET /api/v1/metrics/cluster/:id?start=2024-01-01T00:00:00Z&end=2024-01-01T01:00:00Z
```

**查询参数**:
- `start`: 开始时间（ISO 8601 格式）
- `end`: 结束时间（ISO 8601 格式）

**响应示例**:
```json
{
  "metrics": {
    "broker_count": 3,
    "topic_count": 10,
    "partition_count": 30,
    "messages_in_rate": 1500.5,
    "bytes_in_rate": 1024000,
    "bytes_out_rate": 2048000
  }
}
```

**示例**:
```bash
curl "http://localhost:8080/api/v1/metrics/cluster/1?start=2024-01-01T00:00:00Z&end=2024-01-01T01:00:00Z" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

### 获取 Broker 监控指标

**请求**:
```
GET /api/v1/metrics/broker/:id?broker_id=0
```

### 获取 Topic 监控指标

**请求**:
```
GET /api/v1/metrics/topic/:id?topic=test-topic
```

**示例**:
```bash
curl "http://localhost:8080/api/v1/metrics/topic/1?topic=test-topic" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

### 获取消费组监控指标

**请求**:
```
GET /api/v1/metrics/consumer-group/:id?group=test-group
```

### 自定义 PromQL 查询

**请求**:
```
GET /api/v1/metrics/query/:id?query=kafka_server_broker_topic_metrics_one_min
```

## 审计日志 API

### 查询审计日志

**请求**:
```
GET /api/v1/audit-logs?page=1&page_size=20&user_id=1&operation=CREATE_TOPIC
```

**查询参数**:
- `page`: 页码（默认 1）
- `page_size`: 每页数量（默认 20）
- `user_id`: 用户 ID（可选）
- `operation`: 操作类型（可选）
- `resource_type`: 资源类型（可选）
- `status`: 状态（可选）
- `start_time`: 开始时间（可选）
- `end_time`: 结束时间（可选）

**响应示例**:
```json
{
  "logs": [
    {
      "id": 1,
      "user_id": 1,
      "username": "admin",
      "operation": "CREATE_TOPIC",
      "resource_type": "topic",
      "resource_id": "test-topic",
      "status": "success",
      "ip_address": "192.168.1.100",
      "user_agent": "Mozilla/5.0...",
      "request_data": "{...}",
      "response_data": "{...}",
      "created_at": "2024-01-01T00:00:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "page_size": 20
}
```

**示例**:
```bash
curl "http://localhost:8080/api/v1/audit-logs?page=1&page_size=20" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

### 导出审计日志

**请求**:
```
GET /api/v1/audit-logs/export?format=csv&start_time=2024-01-01&end_time=2024-01-31
```

**查询参数**:
- `format`: 导出格式（csv 或 json）
- `start_time`: 开始时间
- `end_time`: 结束时间

## 错误码

| 错误码 | 说明 |
|--------|------|
| 200 | 成功 |
| 400 | 请求参数错误 |
| 401 | 未认证或 Token 无效 |
| 403 | 权限不足 |
| 404 | 资源不存在 |
| 409 | 资源冲突（如重名） |
| 500 | 服务器内部错误 |

## 限流

API 请求有限流保护：

- **全局限流**: 每分钟 100 请求
- **登录接口**: 每 IP 每分钟 20 请求

超过限流后会返回 429 错误。

## API 端点总览

### 认证
- `POST /api/v1/auth/login` - 用户登录
- `POST /api/v1/auth/refresh` - 刷新 Token
- `GET /api/v1/auth/me` - 获取当前用户信息

### 用户管理（需要超级管理员权限）
- `GET /api/v1/users` - 获取用户列表
- `POST /api/v1/users` - 创建用户
- `GET /api/v1/users/:id` - 获取用户详情
- `PUT /api/v1/users/:id` - 更新用户
- `DELETE /api/v1/users/:id` - 删除用户
- `PUT /api/v1/users/:id/password` - 修改密码
- `POST /api/v1/users/:id/disable` - 禁用用户
- `POST /api/v1/users/:id/enable` - 启用用户

### 集群管理
- `GET /api/v1/clusters` - 获取集群列表
- `POST /api/v1/clusters` - 创建集群
- `GET /api/v1/clusters/:id` - 获取集群详情
- `PUT /api/v1/clusters/:id` - 更新集群
- `DELETE /api/v1/clusters/:id` - 删除集群
- `POST /api/v1/clusters/:id/test` - 测试连接
- `POST /api/v1/clusters/:id/grant` - 授予用户权限
- `POST /api/v1/clusters/:id/revoke` - 撤销用户权限
- `GET /api/v1/clusters/:id/users` - 获取授权用户列表

### Topic 管理
- `GET /api/v1/topics` - 获取 Topic 列表
- `POST /api/v1/topics` - 创建 Topic
- `GET /api/v1/topics/:name` - 获取 Topic 详情
- `DELETE /api/v1/topics/:name` - 删除 Topic
- `PUT /api/v1/topics/:name/config` - 更新 Topic 配置
- `POST /api/v1/topics/sync/:id` - 同步 Topic 数据

### ACL 管理
- `GET /api/v1/acls` - 获取 ACL 列表
- `POST /api/v1/acls` - 创建 ACL
- `DELETE /api/v1/acls/:id` - 删除 ACL
- `POST /api/v1/acls/batch-delete` - 批量删除 ACL
- `POST /api/v1/acls/sync/:id` - 同步 ACL 数据

### 监控
- `GET /api/v1/metrics/cluster/:id` - 获取集群指标
- `GET /api/v1/metrics/broker/:id` - 获取 Broker 指标
- `GET /api/v1/metrics/topic/:id` - 获取 Topic 指标
- `GET /api/v1/metrics/consumer-group/:id` - 获取消费组指标
- `GET /api/v1/metrics/query/:id` - 自定义 PromQL 查询

### 审计日志
- `GET /api/v1/audit-logs` - 查询审计日志
- `GET /api/v1/audit-logs/export` - 导出审计日志