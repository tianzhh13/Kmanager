#!/bin/bash

# Kafka 管理平台 API 测试脚本

BASE_URL="http://localhost:8080"
TOKEN=""

echo "=========================================="
echo "Kafka 管理平台 - API 测试"
echo "=========================================="
echo ""

# 测试健康检查
echo "1. 测试健康检查..."
curl -s $BASE_URL/health | jq .
echo ""

# 测试登录
echo "2. 测试登录..."
LOGIN_RESPONSE=$(curl -s -X POST $BASE_URL/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}')

echo $LOGIN_RESPONSE | jq .

# 提取 Token
TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.access_token')

if [ "$TOKEN" == "null" ] || [ -z "$TOKEN" ]; then
    echo "❌ 登录失败，无法获取 Token"
    exit 1
fi

echo "✅ 登录成功，Token: ${TOKEN:0:50}..."
echo ""

# 测试获取当前用户信息
echo "3. 测试获取当前用户信息..."
curl -s $BASE_URL/api/v1/auth/me \
  -H "Authorization: Bearer $TOKEN" | jq .
echo ""

# 测试获取集群列表
echo "4. 测试获取集群列表..."
curl -s $BASE_URL/api/v1/clusters \
  -H "Authorization: Bearer $TOKEN" | jq .
echo ""

# 测试创建集群
echo "5. 测试创建集群..."
CREATE_RESPONSE=$(curl -s -X POST $BASE_URL/api/v1/clusters \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "cluster_name": "测试集群",
    "bootstrap_servers": "localhost:9092",
    "auth_type": "plaintext",
    "prometheus_url": "http://localhost:9090",
    "description": "API 测试创建的集群"
  }')

echo $CREATE_RESPONSE | jq .

# 提取集群 ID
CLUSTER_ID=$(echo $CREATE_RESPONSE | jq -r '.cluster_id')

if [ "$CLUSTER_ID" != "null" ] && [ ! -z "$CLUSTER_ID" ]; then
    echo "✅ 集群创建成功，ID: $CLUSTER_ID"
    echo ""
    
    # 测试获取集群详情
    echo "6. 测试获取集群详情..."
    curl -s $BASE_URL/api/v1/clusters/$CLUSTER_ID \
      -H "Authorization: Bearer $TOKEN" | jq .
    echo ""
    
    # 测试更新集群
    echo "7. 测试更新集群..."
    curl -s -X PUT $BASE_URL/api/v1/clusters/$CLUSTER_ID \
      -H "Authorization: Bearer $TOKEN" \
      -H "Content-Type: application/json" \
      -d '{
        "description": "更新后的描述"
      }' | jq .
    echo ""
    
    # 测试集群连接
    echo "8. 测试集群连接..."
    curl -s -X POST $BASE_URL/api/v1/clusters/$CLUSTER_ID/test \
      -H "Authorization: Bearer $TOKEN" | jq .
    echo ""
    
    # 测试删除集群
    echo "9. 测试删除集群..."
    curl -s -X DELETE $BASE_URL/api/v1/clusters/$CLUSTER_ID \
      -H "Authorization: Bearer $TOKEN" | jq .
    echo ""
else
    echo "⚠️  集群创建失败或返回格式异常"
fi

echo "=========================================="
echo "API 测试完成！"
echo "=========================================="
echo ""
echo "提示："
echo "- 如果看到 401 错误，说明认证失败"
echo "- 如果看到 500 错误，请检查数据库连接和配置"
echo "- Token 有效期为 1 小时"
echo ""
