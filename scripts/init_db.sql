-- Kafka 管理平台数据库初始化脚本
-- 适用于 MySQL 8.0+

-- 创建数据库
CREATE DATABASE IF NOT EXISTS kafka_management CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

USE kafka_management;

-- 用户表
CREATE TABLE IF NOT EXISTS `user` (
    `user_id` BIGINT PRIMARY KEY AUTO_INCREMENT,
    `username` VARCHAR(64) NOT NULL UNIQUE,
    `password_hash` VARCHAR(128) NOT NULL,
    `email` VARCHAR(128),
    `role` VARCHAR(32) NOT NULL COMMENT '角色：super_admin(超级管理员)/cluster_admin(集群管理员)/normal_user(普通用户)',
    `status` VARCHAR(32) NOT NULL DEFAULT 'active',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX `idx_username` (`username`),
    INDEX `idx_role` (`role`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 集群表
CREATE TABLE IF NOT EXISTS `cluster` (
    `cluster_id` BIGINT PRIMARY KEY AUTO_INCREMENT,
    `cluster_name` VARCHAR(128) NOT NULL,
    `bootstrap_servers` TEXT NOT NULL,
    `auth_type` VARCHAR(32) NOT NULL DEFAULT 'none',
    `sasl_mechanism` VARCHAR(32) COMMENT 'SASL 机制：PLAIN/SCRAM-SHA-256/SCRAM-SHA-512',
    `auth_config` TEXT,
    `jmx_exporter_url` VARCHAR(256) COMMENT 'JMX Exporter HTTP 地址（如 http://broker1:7071）',
    `status` VARCHAR(32) NOT NULL DEFAULT 'active',
    `description` TEXT,
    `created_by` BIGINT NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (`created_by`) REFERENCES `user`(`user_id`),
    INDEX `idx_cluster_name` (`cluster_name`),
    INDEX `idx_status` (`status`),
    INDEX `idx_sasl_mechanism` (`sasl_mechanism`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 集群用户关联表
CREATE TABLE IF NOT EXISTS `cluster_user_relation` (
    `id` BIGINT PRIMARY KEY AUTO_INCREMENT,
    `cluster_id` BIGINT NOT NULL,
    `user_id` BIGINT NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (`cluster_id`) REFERENCES `cluster`(`cluster_id`) ON DELETE CASCADE,
    FOREIGN KEY (`user_id`) REFERENCES `user`(`user_id`) ON DELETE CASCADE,
    UNIQUE KEY `uk_cluster_user` (`cluster_id`, `user_id`),
    INDEX `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Topic 表
CREATE TABLE IF NOT EXISTS `topic` (
    `topic_id` BIGINT PRIMARY KEY AUTO_INCREMENT,
    `cluster_id` BIGINT NOT NULL,
    `topic_name` VARCHAR(256) NOT NULL,
    `partitions` INT NOT NULL,
    `replication_factor` SMALLINT NOT NULL,
    `config` JSON,
    `sync_status` VARCHAR(32) NOT NULL DEFAULT 'synced',
    `last_sync_at` TIMESTAMP NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (`cluster_id`) REFERENCES `cluster`(`cluster_id`) ON DELETE CASCADE,
    UNIQUE KEY `uk_cluster_topic` (`cluster_id`, `topic_name`),
    INDEX `idx_topic_name` (`topic_name`),
    INDEX `idx_sync_status` (`sync_status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ACL 表
CREATE TABLE IF NOT EXISTS `acl` (
    `acl_id` BIGINT PRIMARY KEY AUTO_INCREMENT,
    `cluster_id` BIGINT NOT NULL,
    `resource_type` VARCHAR(32) NOT NULL,
    `resource_name` VARCHAR(256) NOT NULL,
    `resource_pattern` VARCHAR(32) NOT NULL,
    `principal` VARCHAR(256) NOT NULL,
    `host` VARCHAR(128) NOT NULL DEFAULT '*',
    `operation` VARCHAR(32) NOT NULL,
    `permission_type` VARCHAR(32) NOT NULL,
    `sync_status` VARCHAR(32) NOT NULL DEFAULT 'synced',
    `last_sync_at` TIMESTAMP NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (`cluster_id`) REFERENCES `cluster`(`cluster_id`) ON DELETE CASCADE,
    INDEX `idx_cluster_resource` (`cluster_id`, `resource_type`, `resource_name`),
    INDEX `idx_principal` (`principal`),
    INDEX `idx_sync_status` (`sync_status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 审计日志表
CREATE TABLE IF NOT EXISTS `audit_log` (
    `log_id` BIGINT PRIMARY KEY AUTO_INCREMENT,
    `user_id` BIGINT NOT NULL,
    `username` VARCHAR(64) NOT NULL,
    `action` VARCHAR(64) NOT NULL,
    `resource` VARCHAR(64) NOT NULL,
    `resource_id` VARCHAR(256),
    `cluster_id` BIGINT,
    `details` TEXT,
    `ip_address` VARCHAR(64),
    `user_agent` VARCHAR(256),
    `status` VARCHAR(32) NOT NULL,
    `error_msg` TEXT,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX `idx_user_id` (`user_id`),
    INDEX `idx_action` (`action`),
    INDEX `idx_resource` (`resource`),
    INDEX `idx_cluster_id` (`cluster_id`),
    INDEX `idx_created_at` (`created_at`),
    INDEX `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- SCRAM 用户表
CREATE TABLE IF NOT EXISTS `scram_users` (
    `user_id` BIGINT PRIMARY KEY AUTO_INCREMENT,
    `cluster_id` BIGINT NOT NULL,
    `username` VARCHAR(256) NOT NULL,
    `mechanism` VARCHAR(32) NOT NULL DEFAULT 'SCRAM-SHA-256',
    `sync_status` VARCHAR(32) DEFAULT 'synced',
    `last_sync_at` TIMESTAMP NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX `idx_cluster_id` (`cluster_id`),
    UNIQUE INDEX `uk_cluster_username` (`cluster_id`, `username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 用户 Topic 权限表（普通用户只能看到被分配的 Topic）
CREATE TABLE IF NOT EXISTS `user_topic_permission` (
    `id` BIGINT PRIMARY KEY AUTO_INCREMENT,
    `user_id` BIGINT NOT NULL,
    `cluster_id` BIGINT NOT NULL,
    `topic_name` VARCHAR(255) NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `created_by` BIGINT NOT NULL COMMENT '分配人ID',
    UNIQUE KEY `uk_user_cluster_topic` (`user_id`, `cluster_id`, `topic_name`),
    INDEX `idx_user_id` (`user_id`),
    INDEX `idx_cluster_id` (`cluster_id`),
    FOREIGN KEY (`user_id`) REFERENCES `user`(`user_id`) ON DELETE CASCADE,
    FOREIGN KEY (`cluster_id`) REFERENCES `cluster`(`cluster_id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 主机名映射表（hostname → IP，避免依赖 /etc/hosts）
CREATE TABLE IF NOT EXISTS `host_mappings` (
    `id` BIGINT PRIMARY KEY AUTO_INCREMENT,
    `hostname` VARCHAR(255) NOT NULL,
    `ip_address` VARCHAR(45) NOT NULL,
    `description` VARCHAR(500),
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY `uk_hostname` (`hostname`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 插入默认超级管理员用户
-- 密码: admin123 (使用 bcrypt 加密，cost=12)
INSERT INTO `user` (`username`, `password_hash`, `email`, `role`, `status`, `created_at`, `updated_at`)
VALUES ('admin', '$2a$12$gwA7cH9WHrSvaY37au5KaOuqgi5gLCo258.eqmq4tRyHQL7eT.T7q', 'admin@example.com', 'super_admin', 'active', NOW(), NOW())
ON DUPLICATE KEY UPDATE `username` = `username`;

-- 完成
SELECT 'Database initialization completed successfully!' AS message;
