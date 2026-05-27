# 故障排查指南

本文档提供 Kafka 管理平台常见问题的诊断和解决方法。

## 目录

1. [启动问题](#启动问题)
2. [数据库问题](#数据库问题)
3. [Kafka 连接问题](#kafka-连接问题)
4. [认证问题](#认证问题)
5. [性能问题](#性能问题)
6. [前端问题](#前端问题)
7. [监控问题](#监控问题)
8. [日志查看](#日志查看)

## 启动问题

### 问题 1: 端口被占用

**症状**:
```
Error: listen tcp :8080: bind: address already in use
```

**诊断**:
```bash
# Linux/macOS
lsof -i :8080
netstat -tunlp | grep 8080

# Windows
netstat -ano | findstr :8080
```

**解决**:
```bash
# 方式一：停止占用端口的进程
kill -9 <PID>

# 方式二：修改应用端口
# 编辑 configs/config.yaml
server:
  port: 8081
```

### 问题 2: 配置文件不存在

**症状**:
```
Error: open configs/config.yaml: no such file or directory
```

**解决**:
```bash
# 复制配置模板
cp configs/config.yaml.example configs/config.yaml

# 编辑配置文件
vim configs/config.yaml
```

### 问题 3: 权限不足

**症状**:
```
Error: permission denied
```

**解决**:
```bash
# 给予执行权限
chmod +x kafka-management-platform

# 或使用 sudo
sudo ./kafka-management-platform
```

## 数据库问题

### 问题 1: 数据库连接失败

**症状**:
```
Error: failed to connect to database: connection refused
```

**诊断**:
```bash
# 检查数据库服务是否运行
# MySQL
systemctl status mysql
# 或
docker ps | grep mysql

# PostgreSQL
systemctl status postgresql

# 测试数据库连接
mysql -h localhost -u root -p
psql -h localhost -U postgres
```

**解决**:
1. 启动数据库服务
   ```bash
   # MySQL
   systemctl start mysql
   # 或
   docker start mysql
   
   # PostgreSQL
   systemctl start postgresql
   ```

2. 检查配置文件
   ```yaml
   database:
     type: mysql
     host: localhost
     port: 3306
     username: root
     password: your_password
     database: kafka_management
   ```

3. 检查防火墙
   ```bash
   # Linux
   sudo ufw allow 3306
   
   # 或关闭防火墙（仅测试环境）
   sudo ufw disable
   ```

### 问题 2: 数据库不存在

**症状**:
```
Error: Unknown database 'kafka_management'
```

**解决**:
```bash
# MySQL
mysql -u root -p < scripts/init_db.sql

# PostgreSQL
psql -U postgres -f scripts/init_db_postgres.sql
```

### 问题 3: 数据库用户权限不足

**症状**:
```
Error: Access denied for user 'kafka_admin'@'localhost'
```

**解决**:
```sql
-- MySQL
GRANT ALL PRIVILEGES ON kafka_management.* TO 'kafka_admin'@'localhost';
FLUSH PRIVILEGES;

-- PostgreSQL
GRANT ALL PRIVILEGES ON DATABASE kafka_management TO kafka_admin;
```

### 问题 4: 数据库连接池耗尽

**症状**:
```
Error: too many connections
```

**诊断**:
```sql
-- MySQL
SHOW PROCESSLIST;
SHOW STATUS LIKE 'Threads_connected';

-- PostgreSQL
SELECT count(*) FROM pg_stat_activity;
```

**解决**:
1. 调整连接池配置
   ```yaml
   database:
     max_open_conns: 50
     max_idle_conns: 10
     conn_max_lifetime: 3600
   ```

2. 增加数据库最大连接数
   ```sql
   -- MySQL
   SET GLOBAL max_connections = 200;
   
   -- PostgreSQL (postgresql.conf)
   max_connections = 200
   ```

## Kafka 连接问题

### 问题 1: Kafka 集群不可达

**症状**:
```
Error: kafka: client has run out of available brokers to talk to
```

**诊断**:
```bash
# 检查 Kafka 是否运行
docker ps | grep kafka

# 测试 Kafka 连接
kafka-broker-api-versions --bootstrap-server localhost:9092

# 检查网络连通性
telnet localhost 9092
ping kafka-broker
```

**解决**:
1. 启动 Kafka 集群
   ```bash
   docker-compose up -d kafka
   ```

2. 检查 Bootstrap Servers 配置
   ```yaml
   # 确保地址和端口正确
   bootstrap_servers: "kafka1:9092,kafka2:9092,kafka3:9092"
   ```

3. 检查防火墙
   ```bash
   sudo ufw allow 9092
   ```

### 问题 2: SCRAM 认证失败

**症状**:
```
Error: kafka server: Authentication failed
```

**诊断**:
```bash
# 使用命令行工具测试
kafka-topics.sh --bootstrap-server localhost:9093 \
  --command-config kafka.properties \
  --list

# kafka.properties 内容
security.protocol=SASL_PLAINTEXT
sasl.mechanism=SCRAM-SHA-256
sasl.jaas.config=org.apache.kafka.common.security.scram.ScramLoginModule required \
  username="kafka-user" \
  password="kafka-password";
```

**解决**:
1. 检查用户名和密码
2. 检查 SCRAM 机制（SHA-256 或 SHA-512）
3. 在 Kafka 中创建用户
   ```bash
   kafka-configs.sh --bootstrap-server localhost:9092 \
     --entity-type users --entity-name kafka-user \
     --alter --add-config 'SCRAM-SHA-256=[password=kafka-password]'
   ```

### 问题 3: Kerberos 认证失败

**症状**:
```
Error: GSSAPI authentication failed
```

**诊断**:
```bash
# 检查 Kerberos 配置
cat /etc/krb5.conf

# 测试 Kerberos 认证
kinit kafka-client@EXAMPLE.COM -k -t /path/to/kafka-client.keytab

# 查看票据
klist
```

**解决**:
1. 检查 Kerberos 配置文件
2. 检查 keytab 文件路径和权限
3. 检查 principal 格式
   ```yaml
   auth_config:
     principal: "kafka-client@EXAMPLE.COM"
     keytab: "/path/to/kafka-client.keytab"
     realm: "EXAMPLE.COM"
     service_name: "kafka"
   ```

### 问题 4: Topic 操作超时

**症状**:
```
Error: operation timeout
```

**诊断**:
```bash
# 检查 Kafka 集群状态
kafka-broker-api-versions --bootstrap-server localhost:9092

# 检查 Controller
kafka-broker-api-versions --bootstrap-server localhost:9092 | grep controller
```

**解决**:
1. 检查 Kafka 集群健康状态
2. 检查网络延迟
3. 增加超时时间
   ```go
   config.Net.DialTimeout = 30 * time.Second
   config.Net.ReadTimeout = 30 * time.Second
   config.Net.WriteTimeout = 30 * time.Second
   ```

## 认证问题

### 问题 1: JWT Token 无效

**症状**:
```
Error: invalid token
```

**诊断**:
```bash
# 解码 JWT Token（使用 jwt.io 或命令行）
echo "YOUR_TOKEN" | cut -d'.' -f2 | base64 -d
```

**解决**:
1. 检查 Token 是否过期
2. 检查 JWT Secret 配置
3. 使用 Refresh Token 获取新的 Access Token
   ```bash
   curl -X POST http://localhost:8080/api/v1/auth/refresh \
     -H "Content-Type: application/json" \
     -d '{"refresh_token":"your-refresh-token"}'
   ```

### 问题 2: 权限不足

**症状**:
```
Error: permission denied
```

**诊断**:
```bash
# 检查用户角色
curl http://localhost:8080/api/v1/auth/me \
  -H "Authorization: Bearer YOUR_TOKEN"

# 检查集群权限
curl http://localhost:8080/api/v1/clusters/1/users \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**解决**:
1. 联系超级管理员申请权限
2. 检查用户角色和集群授权关系

### 问题 3: 登录失败

**症状**:
```
Error: invalid username or password
```

**诊断**:
```bash
# 检查数据库中的用户
mysql -u root -p
USE kafka_management;
SELECT * FROM users WHERE username = 'admin';
```

**解决**:
1. 确认用户名和密码正确
2. 检查用户状态是否为 active
3. 重置密码
   ```sql
   -- 重置为 admin123
   UPDATE users SET password = '$2a$12$...' WHERE username = 'admin';
   ```

## 性能问题

### 问题 1: API 响应慢

**症状**: API 响应时间超过 3 秒

**诊断**:
```bash
# 检查响应时间
curl -w "@curl-format.txt" -o /dev/null -s http://localhost:8080/api/v1/clusters

# curl-format.txt 内容
time_namelookup:  %{time_namelookup}\n
time_connect:  %{time_connect}\n
time_appconnect:  %{time_appconnect}\n
time_pretransfer:  %{time_pretransfer}\n
time_starttransfer:  %{time_starttransfer}\n
----------\n
time_total:  %{time_total}\n
```

**解决**:
1. 检查数据库查询性能
   ```sql
   -- 开启慢查询日志
   SET GLOBAL slow_query_log = 'ON';
   SET GLOBAL long_query_time = 1;
   ```

2. 添加索引
   ```sql
   CREATE INDEX idx_cluster_id ON topics(cluster_id);
   CREATE INDEX idx_created_at ON audit_logs(created_at);
   ```

3. 启用缓存
   ```yaml
   redis:
     host: localhost
     port: 6379
   ```

### 问题 2: 内存占用高

**症状**: 应用内存占用持续增长

**诊断**:
```bash
# 查看进程内存
top -p <PID>

# 生成内存分析
curl http://localhost:8080/debug/pprof/heap > heap.out
go tool pprof heap.out
```

**解决**:
1. 检查是否有内存泄漏
2. 调整 GOGC 参数
   ```bash
   export GOGC=100
   ```

3. 限制内存使用
   ```bash
   # Linux cgroups
   echo 1G > /sys/fs/cgroup/memory/kafka-management-platform/memory.limit_in_bytes
   ```

### 问题 3: CPU 占用高

**症状**: CPU 使用率持续 100%

**诊断**:
```bash
# 查看 CPU 使用
top -p <PID>

# 生成 CPU 分析
curl http://localhost:8080/debug/pprof/profile?seconds=30 > cpu.out
go tool pprof cpu.out
```

**解决**:
1. 优化热点代码
2. 减少不必要的循环和计算
3. 使用缓存减少重复计算

## 前端问题

### 问题 1: 前端无法访问

**症状**: 浏览器显示空白或 404

**诊断**:
```bash
# 检查前端构建产物
ls -la frontend/dist/

# 检查后端静态资源服务
curl http://localhost:8080/
curl http://localhost:8080/index.html
```

**解决**:
1. 重新构建前端
   ```bash
   cd frontend
   npm run build
   ```

2. 检查后端路由配置
   ```go
   // 确保静态资源服务配置正确
   r.Static("/assets", "./frontend/dist/assets")
   r.NoRoute(func(c *gin.Context) {
       c.File("./frontend/dist/index.html")
   })
   ```

### 问题 2: API 请求跨域

**症状**:
```
Error: CORS policy: No 'Access-Control-Allow-Origin'
```

**解决**:
1. 检查 CORS 中间件配置
   ```go
   func CORSMiddleware() gin.HandlerFunc {
       return func(c *gin.Context) {
           c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
           c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
           c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
           
           if c.Request.Method == "OPTIONS" {
               c.AbortWithStatus(204)
               return
           }
           
           c.Next()
       }
   }
   ```

### 问题 3: 前端构建失败

**症状**: `npm run build` 报错

**诊断**:
```bash
# 检查 Node.js 版本
node --version

# 检查依赖
npm list
```

**解决**:
```bash
# 清理并重新安装
rm -rf node_modules package-lock.json
npm install

# 清理 npm 缓存
npm cache clean --force
```

## 监控问题

### 问题 1: Prometheus 查询失败

**症状**:
```
Error: failed to query prometheus
```

**诊断**:
```bash
# 测试 Prometheus 连接
curl http://localhost:9090/api/v1/query?query=up

# 检查 Prometheus 配置
curl http://localhost:9090/api/v1/status/config
```

**解决**:
1. 检查 Prometheus URL 配置
   ```yaml
   prometheus_url: "http://prometheus:9090"
   ```

2. 检查 Prometheus 是否运行
   ```bash
   docker ps | grep prometheus
   systemctl status prometheus
   ```

### 问题 2: 监控数据不显示

**症状**: 监控页面无数据

**诊断**:
```bash
# 检查 Prometheus 是否采集 Kafka 指标
curl http://localhost:9090/api/v1/query?query=kafka_server_broker_topic_metrics_one_min

# 检查 Prometheus targets
curl http://localhost:9090/api/v1/targets
```

**解决**:
1. 配置 Prometheus 采集 Kafka 指标
   ```yaml
   # prometheus.yml
   scrape_configs:
     - job_name: 'kafka'
       static_configs:
         - targets: ['kafka:7071']
   ```

2. 安装 JMX Exporter
   ```bash
   # Kafka 启动参数添加
   KAFKA_OPTS="-javaagent:/path/to/jmx_prometheus_javaagent.jar=7071:/path/to/kafka.yml"
   ```

## 日志查看

### 应用日志

```bash
# 查看应用日志
tail -f /var/log/kafka-management-platform.log

# 查看最近 100 行
tail -n 100 /var/log/kafka-management-platform.log

# 搜索错误
grep "ERROR" /var/log/kafka-management-platform.log
```

### systemd 日志

```bash
# 查看服务日志
journalctl -u kafka-management-platform -f

# 查看最近 100 行
journalctl -u kafka-management-platform -n 100

# 查看今天的日志
journalctl -u kafka-management-platform --since today
```

### Docker 日志

```bash
# 查看容器日志
docker logs kafka-management-platform -f

# 查看最近 100 行
docker logs kafka-management-platform --tail 100

# 查看指定时间段的日志
docker logs kafka-management-platform --since 1h
```

### 数据库日志

```bash
# MySQL
tail -f /var/log/mysql/error.log

# PostgreSQL
tail -f /var/log/postgresql/postgresql-*.log
```

### Kafka 日志

```bash
# Kafka broker 日志
tail -f /var/log/kafka/server.log

# Docker
docker logs kafka-broker -f
```

## 调试技巧

### 1. 启用调试日志

```yaml
# configs/config.yaml
log:
  level: debug
  format: console
```

### 2. 使用 pprof

```bash
# 查看 CPU 分析
go tool pprof http://localhost:8080/debug/pprof/profile

# 查看内存分析
go tool pprof http://localhost:8080/debug/pprof/heap

# 查看 goroutine
go tool pprof http://localhost:8080/debug/pprof/goroutine
```

### 3. 数据库调试

```sql
-- 开启查询日志
SET GLOBAL general_log = 'ON';

-- 查看慢查询
SHOW VARIABLES LIKE 'slow_query_log';
```

### 4. 网络调试

```bash
# 抓包
tcpdump -i any port 8080 -w app.pcap

# 查看网络连接
netstat -tunlp | grep 8080
```

## 获取帮助

如果以上方法都无法解决问题，请：

1. 查看应用日志获取详细错误信息
2. 在 GitHub Issues 中搜索类似问题
3. 提交新的 Issue，包含：
   - 错误信息
   - 日志片段
   - 环境信息（操作系统、Go 版本、数据库版本）
   - 复现步骤