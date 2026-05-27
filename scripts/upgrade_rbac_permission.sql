-- RBAC 权限管理升级脚本
-- 适用于 MySQL 8.0+

USE kafka_management;

-- 1. 创建用户 Topic 权限表
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

-- 2. 迁移用户角色：read_only -> normal_user
UPDATE `user` SET `role` = 'normal_user' WHERE `role` = 'read_only';

-- 3. 验证迁移结果
SELECT 'Migration completed!' AS message;
SELECT `role`, COUNT(*) as count FROM `user` GROUP BY `role`;
