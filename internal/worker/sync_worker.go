package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"kafka-management-platform/internal/config"
	"kafka-management-platform/internal/logger"
	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/repository"
	"kafka-management-platform/pkg/encryption"
	"kafka-management-platform/pkg/kafka"

	"github.com/IBM/sarama"
)

const (
	// 最大重试次数
	maxRetries = 3
	// 重试间隔
	retryInterval = 10 * time.Second
)

// SyncWorker 数据同步 Worker
// 负责 Topic/ACL 元数据同步和审计日志清理，指标采集已迁移至独立 Collector 进程
type SyncWorker struct {
	cfg             *config.Config
	clusterRepo     repository.ClusterRepository
	topicRepo       repository.TopicRepository
	aclRepo         repository.ACLRepository
	auditLogRepo    repository.AuditLogRepository
	encryptSvc      *encryption.Service
	adminPool       sync.Map // map[int64]*kafka.AdminClient
	kerberosBaseDir string
	syncInterval    time.Duration // 同步间隔
	stopCh          chan struct{}
	wg              sync.WaitGroup
}

// NewSyncWorker 创建数据同步 Worker
func NewSyncWorker(
	cfg *config.Config,
	clusterRepo repository.ClusterRepository,
	topicRepo repository.TopicRepository,
	aclRepo repository.ACLRepository,
	auditLogRepo repository.AuditLogRepository,
) *SyncWorker {
	var encryptSvc *encryption.Service
	if cfg.Encryption.Key != "" {
		var err error
		encryptSvc, err = encryption.NewService(cfg.Encryption.Key)
		if err != nil {
			logger.Warn("Failed to create encryption service, auth config will not be decrypted", "error", err)
			encryptSvc = nil
		}
	}

	kerberosBaseDir := "./kerberos"

	// 从配置读取同步间隔，默认 30 秒
	syncInterval := 30 * time.Second
	if cfg.SyncWorker.Interval > 0 {
		syncInterval = time.Duration(cfg.SyncWorker.Interval) * time.Second
	}
	logger.Info("Sync worker interval", "interval", syncInterval)

	return &SyncWorker{
		cfg:             cfg,
		clusterRepo:     clusterRepo,
		topicRepo:       topicRepo,
		aclRepo:         aclRepo,
		auditLogRepo:    auditLogRepo,
		encryptSvc:      encryptSvc,
		kerberosBaseDir: kerberosBaseDir,
		syncInterval:    syncInterval,
		stopCh:          make(chan struct{}),
	}
}

// Start 启动 Worker
func (w *SyncWorker) Start() error {
	logger.Info("Starting sync worker...")

	// 启动定时同步任务
	w.wg.Add(1)
	go w.runScheduledSync()

	// 启动日志清理任务
	w.wg.Add(1)
	go w.runLogCleanup()

	logger.Info("Sync worker started")
	return nil
}

// Stop 停止 Worker
func (w *SyncWorker) Stop() error {
	logger.Info("Stopping sync worker...")

	// 关闭所有 Admin Client
	w.adminPool.Range(func(key, value interface{}) bool {
		if client, ok := value.(*kafka.AdminClient); ok {
			client.Close()
		}
		w.adminPool.Delete(key)
		return true
	})

	close(w.stopCh)
	w.wg.Wait()
	logger.Info("Sync worker stopped")
	return nil
}

// runScheduledSync 运行定时同步
func (w *SyncWorker) runScheduledSync() {
	defer w.wg.Done()

	// 使用配置的同步间隔
	ticker := time.NewTicker(w.syncInterval)
	defer ticker.Stop()

	// 立即执行一次同步
	w.syncAllClustersWithRetry()

	for {
		select {
		case <-ticker.C:
			w.syncAllClustersWithRetry()
		case <-w.stopCh:
			return
		}
	}
}

// syncAllClustersWithRetry 带重试的同步所有集群
func (w *SyncWorker) syncAllClustersWithRetry() {
	ctx := context.Background()

	clusters, _, err := w.clusterRepo.List(ctx, 0, 1000)
	if err != nil {
		logger.Error("Failed to list clusters", "error", err)
		return
	}

	logger.Info("Found clusters to sync", "count", len(clusters))

	for _, cluster := range clusters {
		w.syncClusterWithRetry(ctx, cluster.ClusterID)
	}
}

// syncClusterWithRetry 带重试的同步单个集群
func (w *SyncWorker) syncClusterWithRetry(ctx context.Context, clusterID int64) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if err := w.SyncCluster(ctx, clusterID); err != nil {
			lastErr = err
			logger.Warn("Failed to sync cluster, retrying", "attempt", i+1, "max_retries", maxRetries, "cluster_id", clusterID, "error", err)
			time.Sleep(retryInterval)
			continue
		}
		return
	}
	logger.Error("Failed to sync cluster after retries", "cluster_id", clusterID, "max_retries", maxRetries, "error", lastErr)
}

// runLogCleanup 运行日志清理
func (w *SyncWorker) runLogCleanup() {
	defer w.wg.Done()

	// 每天凌晨 3 点执行一次
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// 等待到下一个凌晨 3 点
	now := time.Now()
	nextRun := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, now.Location())
	if nextRun.Before(now) {
		nextRun = nextRun.Add(24 * time.Hour)
	}
	duration := nextRun.Sub(now)

	select {
	case <-time.After(duration):
		// 执行一次清理
	case <-w.stopCh:
		return
	}

	// 持续定时清理
	ticker.Stop()
	ticker = time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.cleanupLogs()
		case <-w.stopCh:
			return
		}
	}
}

// SyncCluster 同步单个集群
func (w *SyncWorker) SyncCluster(ctx context.Context, clusterID int64) error {
	logger.Info("Syncing cluster", "cluster_id", clusterID)

	// 获取集群配置
	cluster, err := w.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}

	// 获取或创建 Admin Client
	adminClient, err := w.getAdminClient(cluster)
	if err != nil {
		return fmt.Errorf("failed to get admin client: %w", err)
	}

	// 同步 Topics
	if err := w.syncTopics(ctx, adminClient, clusterID); err != nil {
		logger.Error("Failed to sync topics", "cluster_id", clusterID, "error", err)
	}

	// 同步 ACLs
	if err := w.syncACLs(ctx, adminClient, clusterID); err != nil {
		logger.Error("Failed to sync ACLs", "cluster_id", clusterID, "error", err)
	}

	logger.Info("Cluster synced successfully", "cluster_id", clusterID)
	return nil
}

// syncTopics 同步 Topics
func (w *SyncWorker) syncTopics(ctx context.Context, adminClient *kafka.AdminClient, clusterID int64) error {
	// 从 Kafka 获取 Topics
	kafkaTopics, err := adminClient.ListTopics()
	if err != nil {
		return fmt.Errorf("failed to list topics from kafka: %w", err)
	}

	// 从数据库获取 Topics
	dbTopics, err := w.topicRepo.ListByCluster(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("failed to list topics from database: %w", err)
	}

	// 构建数据库中 Topic 名称集合
	dbTopicMap := make(map[string]*models.Topic)
	for _, topic := range dbTopics {
		dbTopicMap[topic.TopicName] = topic
	}

	now := time.Now()

	// 处理 Kafka 中的 Topics
	for topicName, kafkaTopic := range kafkaTopics {
		// 检查是否已存在
		if existing, exists := dbTopicMap[topicName]; exists {
			// 更新现有 Topic（如果配置有变更）
			partitions := kafkaTopic.NumPartitions
			replicationFactor := kafkaTopic.ReplicationFactor

			// 只有当配置变更时才更新
			if existing.Partitions != partitions || existing.ReplicationFactor != replicationFactor {
				existing.Partitions = partitions
				existing.ReplicationFactor = replicationFactor
				existing.SyncStatus = models.SyncStatusSynced
				existing.LastSyncAt = &now
				if err := w.topicRepo.Update(ctx, existing); err != nil {
					logger.Error("Failed to update topic", "topic", topicName, "error", err)
				}
			} else {
				if existing.SyncStatus != models.SyncStatusSynced {
					existing.SyncStatus = models.SyncStatusSynced
					existing.LastSyncAt = &now
					if err := w.topicRepo.Update(ctx, existing); err != nil {
						logger.Error("Failed to update topic sync status", "topic", topicName, "error", err)
					}
				}
			}
			delete(dbTopicMap, topicName)
		} else {
			// 创建新 Topic
			topic := &models.Topic{
				ClusterID:         clusterID,
				TopicName:         topicName,
				Partitions:        kafkaTopic.NumPartitions,
				ReplicationFactor: kafkaTopic.ReplicationFactor,
				SyncStatus:        models.SyncStatusSynced,
				LastSyncAt:        &now,
			}
			if err := w.topicRepo.Create(ctx, topic); err != nil {
				logger.Error("Failed to create topic", "topic", topicName, "error", err)
			}
		}
	}

	// 清理数据库中已删除的 Topics
	for _, topic := range dbTopicMap {
		if err := w.topicRepo.Delete(ctx, topic.TopicID); err != nil {
			logger.Error("Failed to delete topic", "topic", topic.TopicName, "error", err)
		}
	}

	return nil
}

// syncACLs 同步 ACLs
func (w *SyncWorker) syncACLs(ctx context.Context, adminClient *kafka.AdminClient, clusterID int64) error {
	// 从 Kafka 获取 ACLs（使用 Any 过滤器获取所有 ACL）
	// 注意：不能使用空的 AclFilter{}，因为零值是 Unknown 类型，某些 Kafka 版本不支持
	kafkaACLs, err := adminClient.ListACLs(sarama.AclFilter{
		ResourceType:              sarama.AclResourceAny,
		ResourcePatternTypeFilter: sarama.AclPatternAny,
		Operation:                 sarama.AclOperationAny,
		PermissionType:            sarama.AclPermissionAny,
	})
	if err != nil {
		return fmt.Errorf("failed to list acls from kafka: %w", err)
	}

	// 从数据库获取 ACLs
	dbACLs, err := w.aclRepo.ListByCluster(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("failed to list acls from database: %w", err)
	}

	// 构建数据库中 ACL 映射
	dbACLMap := make(map[string]*models.ACL)
	for _, acl := range dbACLs {
		key := buildACLKey(acl)
		dbACLMap[key] = acl
	}

	now := time.Now()

	// 处理 Kafka 中的 ACLs
	for _, resourceACL := range kafkaACLs {
		for _, kafkaACL := range resourceACL.Acls {
			key := buildKafkaACLKeyFromResourceAndAcl(&resourceACL.Resource, kafkaACL)

			if existing, exists := dbACLMap[key]; exists {
				// 更新同步状态
				existing.SyncStatus = models.SyncStatusSynced
				existing.LastSyncAt = &now
				if err := w.aclRepo.Update(ctx, existing); err != nil {
					logger.Error("Failed to update ACL", "error", err)
				}
				delete(dbACLMap, key)
			} else {
				// 创建新 ACL
				acl := &models.ACL{
					ClusterID:       clusterID,
					ResourceType:    models.ResourceType(resourceACL.ResourceType),
					ResourceName:    resourceACL.ResourceName,
					ResourcePattern: models.PatternType(resourceACL.ResourcePatternType),
					Principal:       kafkaACL.Principal,
					Host:            kafkaACL.Host,
					Operation:       models.OperationType(kafkaACL.Operation),
					PermissionType:  models.PermissionType(kafkaACL.PermissionType),
					SyncStatus:      models.SyncStatusSynced,
					LastSyncAt:      &now,
				}
				if err := w.aclRepo.Create(ctx, acl); err != nil {
					logger.Error("Failed to create ACL", "error", err)
				}
			}
		}
	}

	// 清理数据库中已删除的 ACLs
	for _, acl := range dbACLMap {
		if err := w.aclRepo.Delete(ctx, acl.ACLID); err != nil {
			logger.Error("Failed to delete ACL", "acl_id", acl.ACLID, "error", err)
		}
	}

	return nil
}

// buildACLKey 构建 ACL 唯一键（6 字段完整匹配）
func buildACLKey(acl *models.ACL) string {
	return fmt.Sprintf("%s:%s:%s:%s:%s:%s", acl.ResourceType, acl.ResourceName, acl.Principal, acl.Host, acl.Operation, acl.PermissionType)
}

// buildKafkaACLKeyFromResourceAndAcl 构建 Kafka ACL 唯一键（6 字段完整匹配）
func buildKafkaACLKeyFromResourceAndAcl(resource *sarama.Resource, acl *sarama.Acl) string {
	return fmt.Sprintf("%s:%s:%s:%s:%s:%s", resource.ResourceType, resource.ResourceName, acl.Principal, acl.Host, acl.Operation, acl.PermissionType)
}

// cleanupLogs 清理日志
func (w *SyncWorker) cleanupLogs() {
	logger.Info("Running log cleanup...")

	// 默认清理 180 天前的日志
	retentionDays := 180
	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)

	deleted, err := w.auditLogRepo.DeleteBefore(context.Background(), cutoffTime)
	if err != nil {
		logger.Error("Failed to cleanup logs", "error", err)
		return
	}

	logger.Info("Cleaned up audit logs", "deleted", deleted, "retention_days", retentionDays)
}

// StartLogCleanup 手动触发日志清理
func (w *SyncWorker) StartLogCleanup(retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		retentionDays = 180
	}

	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)
	return w.auditLogRepo.DeleteBefore(context.Background(), cutoffTime)
}

// getAdminClient 获取或创建 Admin Client
func (w *SyncWorker) getAdminClient(cluster *models.Cluster) (*kafka.AdminClient, error) {
	// 解密认证配置
	authConfigJSON := cluster.AuthConfig
	if w.encryptSvc != nil && authConfigJSON != "" {
		decrypted, err := w.encryptSvc.Decrypt(authConfigJSON)
		if err != nil {
			logger.Warn("Failed to decrypt auth config", "cluster_id", cluster.ClusterID, "error", err)
		} else {
			authConfigJSON = string(decrypted)
		}
	}

	// 先从池中获取
	if client, exists := w.adminPool.Load(cluster.ClusterID); exists {
		adminClient := client.(*kafka.AdminClient)
		// 测试连接是否有效
		if err := adminClient.TestConnection(); err == nil {
			return adminClient, nil
		}
		// 连接失效，关闭旧连接并从池中移除
		logger.Info("Admin client stale, recreating", "cluster_id", cluster.ClusterID)
		adminClient.Close()
		w.adminPool.Delete(cluster.ClusterID)
	}

	// 创建新的 Admin Client（支持 Kerberos）
	client, err := kafka.NewAdminClientWithKerberos(cluster, authConfigJSON, w.kerberosBaseDir)
	if err != nil {
		return nil, err
	}

	// 存入池中
	w.adminPool.Store(cluster.ClusterID, client)
	return client, nil
}

// RemoveAdminClient 从连接池中移除 Admin Client
func (w *SyncWorker) RemoveAdminClient(clusterID int64) {
	if client, exists := w.adminPool.LoadAndDelete(clusterID); exists {
		if c, ok := client.(*kafka.AdminClient); ok {
			c.Close()
		}
	}
}

// ManualSync 手动触发同步
func (w *SyncWorker) ManualSync(clusterID int64) error {
	ctx := context.Background()
	return w.SyncCluster(ctx, clusterID)
}

// TriggerSync 触发指定集群的同步（带重试）
func (w *SyncWorker) TriggerSync(ctx context.Context, clusterID int64) {
	w.syncClusterWithRetry(ctx, clusterID)
}
