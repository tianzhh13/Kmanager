package worker

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"kafka-management-platform/internal/config"
	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/repository"
	"kafka-management-platform/internal/service/monitor"
	"kafka-management-platform/pkg/encryption"
	"kafka-management-platform/pkg/kafka"
	"kafka-management-platform/pkg/victoriametrics"

	"github.com/IBM/sarama"
)

const (
	// 最大重试次数
	maxRetries = 3
	// 重试间隔
	retryInterval = 10 * time.Second
)

// SyncWorker 数据同步 Worker
type SyncWorker struct {
	cfg             *config.Config
	clusterRepo     repository.ClusterRepository
	topicRepo       repository.TopicRepository
	aclRepo         repository.ACLRepository
	auditLogRepo    repository.AuditLogRepository
	encryptSvc      *encryption.Service
	monitorSvc      *monitor.Service
	vmClient        *victoriametrics.Client
	adminClientPool sync.Map // map[int64]*kafka.AdminClient
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
			log.Printf("Warning: failed to create encryption service: %v, auth config will not be decrypted", err)
			encryptSvc = nil
		}
	}

	// 创建 VictoriaMetrics 客户端
	vmClient := victoriametrics.NewClient(
		cfg.VictoriaMetrics.WriteURL,
		cfg.VictoriaMetrics.QueryURL,
		cfg.VictoriaMetrics.Enabled,
	)

	// Kerberos 文件基础目录
	kerberosBaseDir := "./kerberos"

	// 创建监控服务
	monitorSvc := monitor.NewService(clusterRepo, encryptSvc, vmClient, kerberosBaseDir)

	// 从配置读取同步间隔，默认 30 秒
	syncInterval := 30 * time.Second
	if cfg.SyncWorker.Interval > 0 {
		syncInterval = time.Duration(cfg.SyncWorker.Interval) * time.Second
	}
	log.Printf("Sync worker interval: %v", syncInterval)

	return &SyncWorker{
		cfg:             cfg,
		clusterRepo:     clusterRepo,
		topicRepo:       topicRepo,
		aclRepo:         aclRepo,
		auditLogRepo:    auditLogRepo,
		encryptSvc:      encryptSvc,
		monitorSvc:      monitorSvc,
		vmClient:        vmClient,
		kerberosBaseDir: kerberosBaseDir,
		syncInterval:    syncInterval,
		stopCh:          make(chan struct{}),
	}
}

// Start 启动 Worker
func (w *SyncWorker) Start() error {
	log.Println("Starting sync worker...")

	// 启动定时同步任务
	w.wg.Add(1)
	go w.runScheduledSync()

	// 启动日志清理任务
	w.wg.Add(1)
	go w.runLogCleanup()

	log.Println("Sync worker started")
	return nil
}

// Stop 停止 Worker
func (w *SyncWorker) Stop() error {
	log.Println("Stopping sync worker...")

	// 关闭所有 Admin Client
	w.adminClientPool.Range(func(key, value interface{}) bool {
		if client, ok := value.(*kafka.AdminClient); ok {
			client.Close()
		}
		w.adminClientPool.Delete(key)
		return true
	})

	close(w.stopCh)
	w.wg.Wait()
	log.Println("Sync worker stopped")
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
		log.Printf("Failed to list clusters: %v", err)
		return
	}

	log.Printf("Found %d clusters to sync", len(clusters))

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
			log.Printf("Retry %d/%d: Failed to sync cluster %d: %v", i+1, maxRetries, clusterID, err)
			time.Sleep(retryInterval)
			continue
		}
		return
	}
	log.Printf("Failed to sync cluster %d after %d retries: %v", clusterID, maxRetries, lastErr)
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
	log.Printf("Syncing cluster %d...", clusterID)

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
		log.Printf("Failed to sync topics for cluster %d: %v", clusterID, err)
	}

	// 同步 ACLs
	if err := w.syncACLs(ctx, adminClient, clusterID); err != nil {
		log.Printf("Failed to sync ACLs for cluster %d: %v", clusterID, err)
	}

	// 采集指标并写入 VictoriaMetrics
	if w.vmClient != nil && w.vmClient.IsEnabled() {
		if err := w.collectAndWriteMetrics(ctx, cluster); err != nil {
			log.Printf("Failed to collect metrics for cluster %d: %v", clusterID, err)
		}
		// 采集 Per-Broker 指标
		if err := w.collectPerBrokerMetrics(ctx, cluster); err != nil {
			log.Printf("Failed to collect per-broker metrics for cluster %d: %v", clusterID, err)
		}
	}

	log.Printf("Cluster %d synced successfully", clusterID)
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
					log.Printf("Failed to update topic %s: %v", topicName, err)
				}
			} else {
				// 更新同步状态
				existing.SyncStatus = models.SyncStatusSynced
				existing.LastSyncAt = &now
				if err := w.topicRepo.Update(ctx, existing); err != nil {
					log.Printf("Failed to update topic sync status %s: %v", topicName, err)
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
				log.Printf("Failed to create topic %s: %v", topicName, err)
			}
		}
	}

	// 清理数据库中已删除的 Topics
	for _, topic := range dbTopicMap {
		if err := w.topicRepo.Delete(ctx, topic.TopicID); err != nil {
			log.Printf("Failed to delete topic %s: %v", topic.TopicName, err)
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
					log.Printf("Failed to update ACL: %v", err)
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
					log.Printf("Failed to create ACL: %v", err)
				}
			}
		}
	}

	// 清理数据库中已删除的 ACLs
	for _, acl := range dbACLMap {
		if err := w.aclRepo.Delete(ctx, acl.ACLID); err != nil {
			log.Printf("Failed to delete ACL %d: %v", acl.ACLID, err)
		}
	}

	return nil
}

// buildACLKey 构建 ACL 唯一键
func buildACLKey(acl *models.ACL) string {
	return fmt.Sprintf("%s:%s:%s:%s:%s", acl.ResourceType, acl.ResourceName, acl.Principal, acl.Operation, acl.PermissionType)
}

// buildKafkaACLKeyFromResourceAndAcl 构建 Kafka ACL 唯一键
func buildKafkaACLKeyFromResourceAndAcl(resource *sarama.Resource, acl *sarama.Acl) string {
	return fmt.Sprintf("%s:%s:%s:%s:%s", resource.ResourceType, resource.ResourceName, acl.Principal, acl.Operation, acl.PermissionType)
}

// cleanupLogs 清理日志
func (w *SyncWorker) cleanupLogs() {
	log.Println("Running log cleanup...")

	// 默认清理 180 天前的日志
	retentionDays := 180
	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)

	deleted, err := w.auditLogRepo.DeleteBefore(context.Background(), cutoffTime)
	if err != nil {
		log.Printf("Failed to cleanup logs: %v", err)
		return
	}

	log.Printf("Cleaned up %d audit logs older than %d days", deleted, retentionDays)
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
			log.Printf("Warning: failed to decrypt auth config for cluster %d: %v", cluster.ClusterID, err)
		} else {
			authConfigJSON = string(decrypted)
		}
	}

	// 先从池中获取
	if client, exists := w.adminClientPool.Load(cluster.ClusterID); exists {
		adminClient := client.(*kafka.AdminClient)
		// 测试连接是否有效
		if err := adminClient.TestConnection(); err == nil {
			return adminClient, nil
		}
		// 连接失效，关闭旧连接并从池中移除
		log.Printf("Admin client for cluster %d is stale, recreating...", cluster.ClusterID)
		adminClient.Close()
		w.adminClientPool.Delete(cluster.ClusterID)
	}

	// 创建新的 Admin Client（支持 Kerberos）
	client, err := kafka.NewAdminClientWithKerberos(cluster, authConfigJSON, w.kerberosBaseDir)
	if err != nil {
		return nil, err
	}

	// 存入池中
	w.adminClientPool.Store(cluster.ClusterID, client)
	return client, nil
}

// RemoveAdminClient 从连接池中移除 Admin Client
func (w *SyncWorker) RemoveAdminClient(clusterID int64) {
	if client, exists := w.adminClientPool.LoadAndDelete(clusterID); exists {
		if c, ok := client.(*kafka.AdminClient); ok {
			c.Close()
		}
	}
}

// collectPerBrokerMetrics 采集 Per-Broker 指标并写入 VictoriaMetrics
func (w *SyncWorker) collectPerBrokerMetrics(ctx context.Context, cluster *models.Cluster) error {
	if cluster.JMXExporterURLs == "" {
		return nil
	}

	urls := monitor.ParseJMXExporterURLs(cluster.JMXExporterURLs)
	if len(urls) == 0 {
		return nil
	}

	baseLabels := map[string]string{
		"cluster_id":   strconv.FormatInt(cluster.ClusterID, 10),
		"cluster_name": cluster.ClusterName,
	}

	multiClient := monitor.NewMultiJMXClient(urls)

	// 1. 获取原始 JMX 指标（request latency, replica lag 等）
	rawMetrics, err := multiClient.FetchAllBrokerRawMetrics(ctx)
	if err != nil {
		log.Printf("[SyncWorker] Failed to fetch raw broker metrics: %v", err)
	} else {
		var vmMetrics []victoriametrics.Metric
		for _, broker := range rawMetrics {
			brokerLabels := make(map[string]string)
			for k, v := range baseLabels {
				brokerLabels[k] = v
			}
			brokerLabels["broker_id"] = strconv.Itoa(broker.BrokerID)
			brokerLabels["broker_host"] = broker.BrokerHost

			for _, m := range broker.Metrics {
				switch m.Name {
				// 请求延迟指标（99分位）
				case "kafka_network_requestmetrics_totaltimems":
					if quantile, ok := m.Labels["quantile"]; ok && quantile == "0.99" {
						if request, ok := m.Labels["request"]; ok {
							latencyLabels := make(map[string]string)
							for k, v := range brokerLabels {
								latencyLabels[k] = v
							}
							latencyLabels["request"] = request
							vmMetrics = append(vmMetrics, victoriametrics.Metric{
								Name:   "kafka_broker_request_latency_ms",
								Value:  m.Value,
								Labels: latencyLabels,
							})
						}
					}

				// 副本同步延迟
				case "kafka_server_replicafetchermanager_maxlag":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_replica_max_lag",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// Controller 状态
				case "kafka_controller_kafkacontroller_activecontrollercount":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_active_controller",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 字节流入（累计计数器，PromQL 用 rate() 算速率）
				case "kafka_server_brokertopicmetrics_bytesin_total", "kafka_server_BrokerTopicMetrics_BytesInPersec":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_bytes_in_total",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 字节流出
				case "kafka_server_brokertopicmetrics_bytesout_total", "kafka_server_BrokerTopicMetrics_BytesOutPersec":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_bytes_out_total",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 消息流入速率
				case "kafka_server_brokertopicmetrics_messagesin_total", "kafka_server_BrokerTopicMetrics_MessagesInPersec":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_messages_in_total",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 未同步分区
				case "kafka_server_replicamanager_underreplicatedpartitions", "kafka_server_ReplicaManager_UnderReplicatedPartitions":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_under_replicated_partitions",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 离线分区
				case "kafka_controller_kafkacontroller_offlinepartitionscount", "kafka_controller_KafkaController_OfflinePartitionsCount":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_offline_partitions",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 生产请求速率
				case "kafka_server_brokertopicmetrics_totalproducerequests_total", "kafka_server_BrokerTopicMetrics_TotalProduceRequestsPersec":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_produce_requests_total",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 消费请求速率
				case "kafka_server_brokertopicmetrics_totalfetchrequests_total", "kafka_server_BrokerTopicMetrics_TotalFetchRequestsPersec":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_fetch_requests_total",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 请求队列大小
				case "kafka_network_requestchannel_requestqueuesize":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_request_queue_size",
						Value:  m.Value,
						Labels: brokerLabels,
					})
				}
			}
		}

		if len(vmMetrics) > 0 {
			if err := w.vmClient.Write(ctx, vmMetrics); err != nil {
				log.Printf("[SyncWorker] Failed to write per-broker JMX metrics: %v", err)
			}
		}
	}

	// 2. 从分区详情计算 Per-Broker Leader/Replica 数量
	partitionDetails, err := w.monitorSvc.GetTopicPartitionDetails(ctx, cluster.ClusterID)
	if err != nil {
		log.Printf("[SyncWorker] Failed to get partition details for per-broker stats: %v", err)
		return nil
	}

	// 统计每个 Broker 的 Leader 数和 Replica 数
	leaderCount := make(map[int32]int)
	replicaCount := make(map[int32]int)
	for _, pd := range partitionDetails {
		leaderCount[pd.Leader]++
		for _, replica := range pd.Replicas {
			replicaCount[replica]++
		}
	}

	// 获取集群元数据以拿到 broker 列表
	metadata, err := w.monitorSvc.GetClusterMetadata(ctx, cluster.ClusterID)
	if err != nil {
		log.Printf("[SyncWorker] Failed to get cluster metadata for per-broker stats: %v", err)
		return nil
	}

	var vmMetrics []victoriametrics.Metric
	for _, broker := range metadata.Brokers {
		brokerLabels := make(map[string]string)
		for k, v := range baseLabels {
			brokerLabels[k] = v
		}
		brokerLabels["broker_id"] = strconv.FormatInt(int64(broker.ID), 10)
		brokerLabels["broker_host"] = broker.Host

		lc := float64(leaderCount[broker.ID])
		rc := float64(replicaCount[broker.ID])

		vmMetrics = append(vmMetrics,
			victoriametrics.Metric{Name: "kafka_broker_leader_count", Value: lc, Labels: brokerLabels},
			victoriametrics.Metric{Name: "kafka_broker_replica_count", Value: rc, Labels: brokerLabels},
		)
	}

	if len(vmMetrics) > 0 {
		if err := w.vmClient.Write(ctx, vmMetrics); err != nil {
			log.Printf("[SyncWorker] Failed to write per-broker leader/replica metrics: %v", err)
		}
	}

	return nil
}

// ManualSync 手动触发同步
func (w *SyncWorker) ManualSync(clusterID int64) error {
	ctx := context.Background()
	return w.SyncCluster(ctx, clusterID)
}

// TriggerSync 触发指定集群的同步（带重试）
func (w *SyncWorker) TriggerSync(ctx context.Context, clusterID int64) error {
	w.syncClusterWithRetry(ctx, clusterID)
	return nil
}

// collectAndWriteMetrics 采集指标并写入 VictoriaMetrics
func (w *SyncWorker) collectAndWriteMetrics(ctx context.Context, cluster *models.Cluster) error {
	// 获取集群指标
	metrics, err := w.monitorSvc.GetClusterMetrics(ctx, cluster.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster metrics: %w", err)
	}

	// 构建基础标签
	baseLabels := map[string]string{
		"cluster_id":   strconv.FormatInt(cluster.ClusterID, 10),
		"cluster_name": cluster.ClusterName,
	}

	var vmMetrics []victoriametrics.Metric

	// 1. JMX Broker 指标已全部改由 collectPerBrokerMetrics 写入 per-broker 粒度
	//    集群级聚合由前端 PromQL (sum/max) 完成

	// 2. 写入集群级别元数据指标
	vmMetrics = append(vmMetrics,
		victoriametrics.Metric{Name: "kafka_broker_count", Value: float64(metrics.BrokerCount), Labels: baseLabels},
		victoriametrics.Metric{Name: "kafka_topic_count", Value: float64(metrics.TopicCount), Labels: baseLabels},
		victoriametrics.Metric{Name: "kafka_consumer_group_count", Value: float64(len(metrics.ConsumerGroups)), Labels: baseLabels},
	)

	// 2.1 写入 Broker 信息指标（用于 Broker 监控 Tab 筛选）
	for _, broker := range metrics.Brokers {
		brokerLabels := make(map[string]string)
		for k, v := range baseLabels {
			brokerLabels[k] = v
		}
		brokerLabels["broker_id"] = strconv.FormatInt(int64(broker.ID), 10)
		brokerLabels["broker_host"] = broker.Host
		brokerLabels["broker_port"] = strconv.FormatInt(int64(broker.Port), 10)
		if broker.Rack != "" {
			brokerLabels["broker_rack"] = broker.Rack
		}
		// kafka_broker_info 是一个 info 类型指标，值固定为 1
		vmMetrics = append(vmMetrics,
			victoriametrics.Metric{Name: "kafka_broker_info", Value: 1, Labels: brokerLabels},
		)
	}

	// 3. 写入按 Topic 分区的指标
	topicPartitions, err := w.monitorSvc.GetTopicPartitionCount(ctx, cluster.ClusterID)
	if err == nil {
		for topic, partitionCount := range topicPartitions {
			labels := make(map[string]string)
			for k, v := range baseLabels {
				labels[k] = v
			}
			labels["topic"] = topic
			vmMetrics = append(vmMetrics,
				victoriametrics.Metric{Name: "kafka_topic_partitions", Value: float64(partitionCount), Labels: labels},
			)
		}
	}

	// 4. 写入分区详情指标（副本、ISR、Leader 等）
	partitionDetails, err := w.monitorSvc.GetTopicPartitionDetails(ctx, cluster.ClusterID)
	if err == nil {
		// 构建 topic -> partitions 映射用于获取 offset
		topicPartitions := make(map[string][]int32)
		for _, pd := range partitionDetails {
			topicPartitions[pd.Topic] = append(topicPartitions[pd.Topic], pd.Partition)
		}

		// 获取所有分区的 LogEndOffset
		endOffsets, err := w.monitorSvc.GetTopicPartitionOffsets(ctx, cluster.ClusterID, topicPartitions)
		if err != nil {
			log.Printf("[SyncWorker] Failed to get partition offsets: %v", err)
		}

		// 获取所有分区的 LogStartOffset
		startOffsets, err := w.monitorSvc.GetTopicPartitionStartOffsets(ctx, cluster.ClusterID, topicPartitions)
		if err != nil {
			log.Printf("[SyncWorker] Failed to get partition start offsets: %v", err)
		}

		for _, pd := range partitionDetails {
			labels := make(map[string]string)
			for k, v := range baseLabels {
				labels[k] = v
			}
			labels["topic"] = pd.Topic
			labels["partition"] = strconv.FormatInt(int64(pd.Partition), 10)

			// 分区副本数
			vmMetrics = append(vmMetrics,
				victoriametrics.Metric{Name: "kafka_topic_partition_replicas", Value: float64(len(pd.Replicas)), Labels: labels},
			)

			// ISR 数量
			vmMetrics = append(vmMetrics,
				victoriametrics.Metric{Name: "kafka_topic_partition_in_sync_replica", Value: float64(len(pd.ISR)), Labels: labels},
			)

			// 是否是首选 Leader（1 或 0）
			preferredLeaderValue := float64(0)
			if pd.IsPreferredLeader {
				preferredLeaderValue = 1
			}
			vmMetrics = append(vmMetrics,
				victoriametrics.Metric{Name: "kafka_topic_partition_leader_is_preferred", Value: preferredLeaderValue, Labels: labels},
			)

			// 是否未同步（1 或 0）
			underReplicatedValue := float64(0)
			if pd.UnderReplicated {
				underReplicatedValue = 1
			}
			vmMetrics = append(vmMetrics,
				victoriametrics.Metric{Name: "kafka_topic_partition_under_replicated_partition", Value: underReplicatedValue, Labels: labels},
			)

			// 从 endOffsets 中获取 LogEndOffset
			logEndOffset := int64(0)
			if endOffsets != nil && endOffsets[pd.Topic] != nil {
				if offset, ok := endOffsets[pd.Topic][pd.Partition]; ok {
					logEndOffset = offset
				}
			}

			// 当前偏移量（LogEndOffset）
			vmMetrics = append(vmMetrics,
				victoriametrics.Metric{Name: "kafka_topic_partition_current_offset", Value: float64(logEndOffset), Labels: labels},
			)

			// 从 startOffsets 中获取 LogStartOffset
			logStartOffset := int64(0)
			if startOffsets != nil && startOffsets[pd.Topic] != nil {
				if offset, ok := startOffsets[pd.Topic][pd.Partition]; ok {
					logStartOffset = offset
				}
			}

			// 最旧偏移量（LogStartOffset）
			vmMetrics = append(vmMetrics,
				victoriametrics.Metric{Name: "kafka_topic_partition_oldest_offset", Value: float64(logStartOffset), Labels: labels},
			)
		}
	}

	// 5. 写入消费者组详细指标
	var totalLag int64
	for _, cg := range metrics.ConsumerGroups {
		// 跳过内部消费者组
		if strings.HasPrefix(cg.GroupID, "__") {
			continue
		}

		// 消费者组成员数
		cgLabels := make(map[string]string)
		for k, v := range baseLabels {
			cgLabels[k] = v
		}
		cgLabels["consumergroup"] = cg.GroupID
		vmMetrics = append(vmMetrics,
			victoriametrics.Metric{Name: "kafka_consumergroup_members", Value: float64(cg.Members), Labels: cgLabels},
		)

		// 按 Topic 汇总的 Lag
		for _, topicLag := range cg.Topics {
			// 跳过内部 Topic
			if strings.HasPrefix(topicLag.Topic, "__") {
				continue
			}

			topicLabels := make(map[string]string)
			for k, v := range baseLabels {
				topicLabels[k] = v
			}
			topicLabels["consumergroup"] = cg.GroupID
			topicLabels["topic"] = topicLag.Topic
			vmMetrics = append(vmMetrics,
				victoriametrics.Metric{Name: "kafka_consumergroup_lag_sum", Value: float64(topicLag.Lag), Labels: topicLabels},
			)

			// 分区级别的 Lag 和 current_offset
			for _, partitionLag := range topicLag.Partitions {
				partitionLabels := make(map[string]string)
				for k, v := range topicLabels {
					partitionLabels[k] = v
				}
				partitionLabels["partition"] = strconv.FormatInt(int64(partitionLag.Partition), 10)
				vmMetrics = append(vmMetrics,
					victoriametrics.Metric{Name: "kafka_consumergroup_lag", Value: float64(partitionLag.Lag), Labels: partitionLabels},
					victoriametrics.Metric{Name: "kafka_consumergroup_current_offset", Value: float64(partitionLag.CurrentOffset), Labels: partitionLabels},
				)
				// 写入 lag_seconds（仅对有值的分区）
				if partitionLag.LagSeconds >= 0 {
					vmMetrics = append(vmMetrics,
						victoriametrics.Metric{Name: "kafka_consumergroup_lag_seconds", Value: float64(partitionLag.LagSeconds), Labels: partitionLabels},
					)
				}
			}
		}

		totalLag += cg.TotalLag
	}

	// 5. 写入总延迟
	vmMetrics = append(vmMetrics,
		victoriametrics.Metric{Name: "kafka_total_lag", Value: float64(totalLag), Labels: baseLabels},
	)

	// 写入 VictoriaMetrics
	if err := w.vmClient.Write(ctx, vmMetrics); err != nil {
		return fmt.Errorf("failed to write metrics to victoriametrics: %w", err)
	}

	log.Printf("Collected and wrote %d metrics for cluster %d", len(vmMetrics), cluster.ClusterID)
	return nil
}
