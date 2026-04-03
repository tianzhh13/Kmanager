# 集群连接测试文档

## 概述

本文档说明如何测试 Kafka 集群连接功能，包括三种认证方式：PLAINTEXT、SCRAM 和 Kerberos。

## API 端点

```
POST /api/v1/clusters/:id/test
```

需要认证：是（JWT Token）

## 测试场景

### 1. PLAINTEXT 认证测试

创建一个使用 PLAINTEXT 认证的集群并测试连接：

```bash
# 1. 登录获取 Token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.access_token')

# 2. 创建 PLAINTEXT 集群
CLUSTER_RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/clusters \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "cluster_name": "测试集群-PLAINTEXT",
    "bootstrap_servers": "localhost:9092",
    "auth_type": "plaintext",
    "description": "PLAINTEXT 认证测试集群"
  }')

CLUSTER_ID=$(echo $CLUSTER_RESPONSE | jq -r '.cluster_id')

# 3. 测试连接
curl -s -X POST http://localhost:8080/api/v1/clusters/$CLUSTER_ID/test \
  -H "Authorization: Bearer $TOKEN" | jq .
```

预期响应（成功）：
```json
{
  "message": "connection test successful"
}
```

预期响应（失败）：
```json
{
  "error": "connection test failed",
  "details": "connection test failed: kafka: client has run out of available brokers to talk to"
}
```

### 2. SCRAM 认证测试

创建一个使用 SCRAM-SHA-256 认证的集群并测试连接：

```bash
# 创建 SCRAM 集群
CLUSTER_RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/clusters \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "cluster_name": "测试集群-SCRAM",
    "bootstrap_servers": "kafka-broker:9093",
    "auth_type": "scram",
    "auth_config": {
      "username": "kafka-user",
      "password": "kafka-password",
      "mechanism": "SCRAM-SHA-256"
    },
    "description": "SCRAM 认证测试集群"
  }')

CLUSTER_ID=$(echo $CLUSTER_RESPONSE | jq -r '.cluster_id')

# 测试连接
curl -s -X POST http://localhost:8080/api/v1/clusters/$CLUSTER_ID/test \
  -H "Authorization: Bearer $TOKEN" | jq .
```

### 3. Kerberos 认证测试

创建一个使用 Kerberos 认证的集群并测试连接：

```bash
# 创建 Kerberos 集群
CLUSTER_RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/clusters \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "cluster_name": "测试集群-Kerberos",
    "bootstrap_servers": "kafka-broker:9094",
    "auth_type": "kerberos",
    "auth_config": {
      "principal": "kafka-client@EXAMPLE.COM",
      "keytab": "/path/to/kafka-client.keytab",
      "realm": "EXAMPLE.COM",
      "service_name": "kafka"
    },
    "description": "Kerberos 认证测试集群"
  }')

CLUSTER_ID=$(echo $CLUSTER_RESPONSE | jq -r '.cluster_id')

# 测试连接
curl -s -X POST http://localhost:8080/api/v1/clusters/$CLUSTER_ID/test \
  -H "Authorization: Bearer $TOKEN" | jq .
```

## 错误处理

### 常见错误及解决方案

1. **连接超时**
   - 错误信息：`connection test failed: kafka: client has run out of available brokers to talk to`
   - 原因：Kafka 集群不可达或 Bootstrap Servers 配置错误
   - 解决：检查网络连接和 Bootstrap Servers 配置

2. **认证失败**
   - 错误信息：`connection test failed: kafka server: The request is not authorized`
   - 原因：认证配置错误（用户名、密码、机制不正确）
   - 解决：检查 auth_config 中的认证信息

3. **TLS 证书错误**
   - 错误信息：`connection test failed: x509: certificate signed by unknown authority`
   - 原因：TLS 证书验证失败
   - 解决：配置正确的 CA 证书或在测试环境中使用 InsecureSkipVerify

4. **Kerberos 配置错误**
   - 错误信息：`connection test failed: GSSAPI authentication failed`
   - 原因：Kerberos 配置不正确（principal、keytab、realm）
   - 解决：检查 Kerberos 配置文件和 keytab 文件路径

## 安全注意事项

1. **敏感信息保护**：
   - 认证配置（auth_config）在数据库中使用 AES-256 加密存储
   - API 响应中不返回明文的认证信息
   - 错误信息不暴露敏感的认证细节

2. **权限控制**：
   - 只有超级管理员和被授权的集群管理员可以测试集群连接
   - 测试连接操作会记录审计日志

## 测试清单

- [ ] PLAINTEXT 认证连接测试成功
- [ ] PLAINTEXT 认证连接测试失败（错误的 Bootstrap Servers）
- [ ] SCRAM-SHA-256 认证连接测试成功
- [ ] SCRAM-SHA-256 认证连接测试失败（错误的用户名/密码）
- [ ] SCRAM-SHA-512 认证连接测试成功
- [ ] Kerberos 认证连接测试成功
- [ ] Kerberos 认证连接测试失败（错误的 principal）
- [ ] 权限验证：非授权用户无法测试连接
- [ ] 错误信息不暴露敏感信息
