-- Kafka 管理平台数据库初始化脚本
-- 适用于 PostgreSQL 14+

-- 创建数据库（需要以 postgres 用户执行）
-- CREATE DATABASE kafka_management WITH ENCODING 'UTF8';

-- 连接到数据库
\c kafka_management;

-- 用户表
CREATE TABLE IF NOT EXISTS "user" (
    user_id BIGSERIAL PRIMARY KEY,
    username VARCHAR(64) NOT NULL UNIQUE,
    password_hash VARCHAR(128) NOT NULL,
    email VARCHAR(128),
    role VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_user_username ON "user"(username);
CREATE INDEX IF NOT EXISTS idx_user_role ON "user"(role);

-- 集群表
CREATE TABLE IF NOT EXISTS cluster (
    cluster_id BIGSERIAL PRIMARY KEY,
    cluster_name VARCHAR(128) NOT NULL,
    bootstrap_servers TEXT NOT NULL,
    auth_type VARCHAR(32) NOT NULL DEFAULT 'none',
    auth_config TEXT,
    jmx_host VARCHAR(128),
    jmx_port INTEGER,
    prometheus_url VARCHAR(256),
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    description TEXT,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (created_by) REFERENCES "user"(user_id)
);

CREATE INDEX IF NOT EXISTS idx_cluster_name ON cluster(cluster_name);
CREATE INDEX IF NOT EXISTS idx_cluster_status ON cluster(status);

-- 集群用户关联表
CREATE TABLE IF NOT EXISTS cluster_user_relation (
    id BIGSERIAL PRIMARY KEY,
    cluster_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (cluster_id) REFERENCES cluster(cluster_id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES "user"(user_id) ON DELETE CASCADE,
    UNIQUE (cluster_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_cluster_user_relation_user_id ON cluster_user_relation(user_id);

-- Topic 表
CREATE TABLE IF NOT EXISTS topic (
    topic_id BIGSERIAL PRIMARY KEY,
    cluster_id BIGINT NOT NULL,
    topic_name VARCHAR(256) NOT NULL,
    partitions INTEGER NOT NULL,
    replication_factor SMALLINT NOT NULL,
    config JSONB,
    sync_status VARCHAR(32) NOT NULL DEFAULT 'synced',
    last_sync_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (cluster_id) REFERENCES cluster(cluster_id) ON DELETE CASCADE,
    UNIQUE (cluster_id, topic_name)
);

CREATE INDEX IF NOT EXISTS idx_topic_name ON topic(topic_name);
CREATE INDEX IF NOT EXISTS idx_topic_sync_status ON topic(sync_status);

-- ACL 表
CREATE TABLE IF NOT EXISTS acl (
    acl_id BIGSERIAL PRIMARY KEY,
    cluster_id BIGINT NOT NULL,
    resource_type VARCHAR(32) NOT NULL,
    resource_name VARCHAR(256) NOT NULL,
    resource_pattern VARCHAR(32) NOT NULL,
    principal VARCHAR(256) NOT NULL,
    host VARCHAR(128) NOT NULL DEFAULT '*',
    operation VARCHAR(32) NOT NULL,
    permission_type VARCHAR(32) NOT NULL,
    sync_status VARCHAR(32) NOT NULL DEFAULT 'synced',
    last_sync_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (cluster_id) REFERENCES cluster(cluster_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_acl_cluster_resource ON acl(cluster_id, resource_type, resource_name);
CREATE INDEX IF NOT EXISTS idx_acl_principal ON acl(principal);
CREATE INDEX IF NOT EXISTS idx_acl_sync_status ON acl(sync_status);

-- 审计日志表
CREATE TABLE IF NOT EXISTS audit_log (
    log_id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    username VARCHAR(64) NOT NULL,
    action VARCHAR(64) NOT NULL,
    resource VARCHAR(64) NOT NULL,
    resource_id VARCHAR(256),
    cluster_id BIGINT,
    details TEXT,
    ip_address VARCHAR(64),
    user_agent VARCHAR(256),
    status VARCHAR(32) NOT NULL,
    error_msg TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_audit_log_user_id ON audit_log(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_action ON audit_log(action);
CREATE INDEX IF NOT EXISTS idx_audit_log_resource ON audit_log(resource);
CREATE INDEX IF NOT EXISTS idx_audit_log_cluster_id ON audit_log(cluster_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_created_at ON audit_log(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_log_status ON audit_log(status);

-- SCRAM 用户表
CREATE TABLE IF NOT EXISTS scram_users (
    user_id BIGSERIAL PRIMARY KEY,
    cluster_id BIGINT NOT NULL,
    username VARCHAR(256) NOT NULL,
    mechanism VARCHAR(32) NOT NULL DEFAULT 'SCRAM-SHA-256',
    sync_status VARCHAR(32) DEFAULT 'synced',
    last_sync_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (cluster_id, username)
);

CREATE INDEX IF NOT EXISTS idx_scram_users_cluster_id ON scram_users(cluster_id);

-- 创建更新时间触发器函数
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- 为需要自动更新 updated_at 的表创建触发器
CREATE TRIGGER update_user_updated_at BEFORE UPDATE ON "user"
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_cluster_updated_at BEFORE UPDATE ON cluster
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_topic_updated_at BEFORE UPDATE ON topic
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_scram_users_updated_at BEFORE UPDATE ON scram_users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 插入默认超级管理员用户
-- 密码: admin123 (使用 bcrypt 加密，cost=12)
INSERT INTO "user" (username, password_hash, email, role, status, created_at, updated_at)
VALUES ('admin', '$2a$12$gwA7cH9WHrSvaY37au5KaOuqgi5gLCo258.eqmq4tRyHQL7eT.T7q', 'admin@example.com', 'super_admin', 'active', NOW(), NOW())
ON CONFLICT (username) DO NOTHING;

-- 完成
SELECT 'Database initialization completed successfully!' AS message;
