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

// copyLabels 复制标签 map
func copyLabels(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
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

				// Follower 最小拉取速率
				case "kafka_server_replicafetchermanager_minfetchrate":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_min_fetch_rate",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// Follower 失败分区数
				case "kafka_server_replicafetchermanager_failedpartitionscount":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_failed_partitions_count",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// Follower 死线程数
				case "kafka_server_replicafetchermanager_deadthreadcount":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_dead_thread_count",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// ============ 批次2：请求延迟指标 ============

				// 请求排队耗时
				case "kafka_network_requestmetrics_requestqueuetimems":
					if quantile, ok := m.Labels["quantile"]; ok && quantile == "0.99" {
						if request, ok := m.Labels["request"]; ok {
							labels := copyLabels(brokerLabels)
							labels["request"] = request
							vmMetrics = append(vmMetrics, victoriametrics.Metric{
								Name:   "kafka_broker_request_queue_time_ms",
								Value:  m.Value,
								Labels: labels,
							})
						}
					}

				// 本地处理耗时
				case "kafka_network_requestmetrics_localtimems":
					if quantile, ok := m.Labels["quantile"]; ok && quantile == "0.99" {
						if request, ok := m.Labels["request"]; ok {
							labels := copyLabels(brokerLabels)
							labels["request"] = request
							vmMetrics = append(vmMetrics, victoriametrics.Metric{
								Name:   "kafka_broker_request_local_time_ms",
								Value:  m.Value,
								Labels: labels,
							})
						}
					}

				// 远程等待耗时
				case "kafka_network_requestmetrics_remotetimems":
					if quantile, ok := m.Labels["quantile"]; ok && quantile == "0.99" {
						if request, ok := m.Labels["request"]; ok {
							labels := copyLabels(brokerLabels)
							labels["request"] = request
							vmMetrics = append(vmMetrics, victoriametrics.Metric{
								Name:   "kafka_broker_request_remote_time_ms",
								Value:  m.Value,
								Labels: labels,
							})
						}
					}

				// 响应排队耗时
				case "kafka_network_requestmetrics_responsequeuetimems":
					if quantile, ok := m.Labels["quantile"]; ok && quantile == "0.99" {
						if request, ok := m.Labels["request"]; ok {
							labels := copyLabels(brokerLabels)
							labels["request"] = request
							vmMetrics = append(vmMetrics, victoriametrics.Metric{
								Name:   "kafka_broker_response_queue_time_ms",
								Value:  m.Value,
								Labels: labels,
							})
						}
					}

				// 响应发送耗时
				case "kafka_network_requestmetrics_responsesendtimems":
					if quantile, ok := m.Labels["quantile"]; ok && quantile == "0.99" {
						if request, ok := m.Labels["request"]; ok {
							labels := copyLabels(brokerLabels)
							labels["request"] = request
							vmMetrics = append(vmMetrics, victoriametrics.Metric{
								Name:   "kafka_broker_response_send_time_ms",
								Value:  m.Value,
								Labels: labels,
							})
						}
					}

				// 限流耗时
				case "kafka_network_requestmetrics_throttletimems":
					if quantile, ok := m.Labels["quantile"]; ok && quantile == "0.99" {
						if request, ok := m.Labels["request"]; ok {
							labels := copyLabels(brokerLabels)
							labels["request"] = request
							vmMetrics = append(vmMetrics, victoriametrics.Metric{
								Name:   "kafka_broker_throttle_time_ms",
								Value:  m.Value,
								Labels: labels,
							})
						}
					}

				// 消息转换耗时
				case "kafka_network_requestmetrics_messageconversionstimems":
					if quantile, ok := m.Labels["quantile"]; ok && quantile == "0.99" {
						if request, ok := m.Labels["request"]; ok {
							labels := copyLabels(brokerLabels)
							labels["request"] = request
							vmMetrics = append(vmMetrics, victoriametrics.Metric{
								Name:   "kafka_broker_message_conversions_time_ms",
								Value:  m.Value,
								Labels: labels,
							})
						}
					}

				// 请求字节数
				case "kafka_network_requestmetrics_requestbytes":
					if quantile, ok := m.Labels["quantile"]; ok && quantile == "0.99" {
						if request, ok := m.Labels["request"]; ok {
							labels := copyLabels(brokerLabels)
							labels["request"] = request
							vmMetrics = append(vmMetrics, victoriametrics.Metric{
								Name:   "kafka_broker_request_bytes",
								Value:  m.Value,
								Labels: labels,
							})
						}
					}

				// 请求错误总数
				case "kafka_network_requestmetrics_errors_total":
					if request, ok := m.Labels["request"]; ok {
						if errorType, ok := m.Labels["error"]; ok {
							labels := copyLabels(brokerLabels)
							labels["request"] = request
							labels["error"] = errorType
							vmMetrics = append(vmMetrics, victoriametrics.Metric{
								Name:   "kafka_broker_request_errors_total",
								Value:  m.Value,
								Labels: labels,
							})
						}
					}

				// 请求总数
				case "kafka_network_requestmetrics_requests_total":
					if request, ok := m.Labels["request"]; ok {
						labels := copyLabels(brokerLabels)
						labels["request"] = request
						vmMetrics = append(vmMetrics, victoriametrics.Metric{
							Name:   "kafka_broker_requests_total",
							Value:  m.Value,
							Labels: labels,
						})
					}

				// Controller 状态
				case "kafka_controller_kafkacontroller_activecontrollercount":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_active_controller",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// Controller 事件排队耗时
				case "kafka_controller_controllereventmanager_eventqueuetimems":
					if quantile, ok := m.Labels["quantile"]; ok && quantile == "0.99" {
						vmMetrics = append(vmMetrics, victoriametrics.Metric{
							Name:   "kafka_broker_controller_event_queue_time_ms",
							Value:  m.Value,
							Labels: brokerLabels,
						})
					}

				// Unclean Leader 选举总数
				case "kafka_controller_controllerstats_uncleanleaderelections_total":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_unclean_leader_elections_total",
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

				// ============ 批次3：额外流量指标 ============

				// 副本同步流入
				case "kafka_server_brokertopicmetrics_replicationbytesin_total":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_replication_bytes_in_total",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 副本同步流出
				case "kafka_server_brokertopicmetrics_replicationbytesout_total":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_replication_bytes_out_total",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 分区迁移流入
				case "kafka_server_brokertopicmetrics_reassignmentbytesin_total":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_reassignment_bytes_in_total",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 分区迁移流出
				case "kafka_server_brokertopicmetrics_reassignmentbytesout_total":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_reassignment_bytes_out_total",
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

				// ============ 批次1：集群概览指标 ============

				// 活跃 Broker 数量
				case "kafka_controller_kafkacontroller_activebrokercount":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_active_broker_count",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// Fenced Broker 数量
				case "kafka_controller_kafkacontroller_fencedbrokercount":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_fenced_broker_count",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 集群总分区数
				case "kafka_controller_kafkacontroller_globalpartitioncount":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_global_partition_count",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 集群总 Topic 数
				case "kafka_controller_kafkacontroller_globaltopiccount":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_global_topic_count",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// Preferred 副本不均衡数
				case "kafka_controller_kafkacontroller_preferredreplicaimbalancecount":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_preferred_replica_imbalance",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 拒绝字节总量
				case "kafka_server_brokertopicmetrics_bytesrejected_total":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_bytes_rejected_total",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// ============ 批次4：额外请求/错误指标 ============

				// 失败生产请求
				case "kafka_server_brokertopicmetrics_failedproducerequests_total":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_failed_produce_requests_total",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 失败拉取请求
				case "kafka_server_brokertopicmetrics_failedfetchrequests_total":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_failed_fetch_requests_total",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 生产消息转换
				case "kafka_server_brokertopicmetrics_producemessageconversions_total":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_produce_message_conversions_total",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 拉取消息转换
				case "kafka_server_brokertopicmetrics_fetchmessageconversions_total":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_fetch_message_conversions_total",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 无效 Magic Number
				case "kafka_server_brokertopicmetrics_invalidmagicnumberrecords_total":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_invalid_magic_number_records_total",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 无效 CRC
				case "kafka_server_brokertopicmetrics_invalidmessagecrcrecords_total":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_invalid_message_crc_records_total",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 无效 Offset/Sequence
				case "kafka_server_brokertopicmetrics_invalidoffsetorsequencerecords_total":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_invalid_offset_or_sequence_records_total",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 无 Key Compact 记录
				case "kafka_server_brokertopicmetrics_nokeycompactedtopicrecords_total":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_no_key_compacted_topic_records_total",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// ============ 批次5：副本-detail 指标 ============

				// Under MinISR 分区数
				case "kafka_server_replicamanager_underminisrpartitioncount":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_under_min_isr_partition_count",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// At MinISR 分区数
				case "kafka_server_replicamanager_atminisrpartitioncount":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_at_min_isr_partition_count",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 离线副本数
				case "kafka_server_replicamanager_offlinereplicacount":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_offline_replica_count",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// ISR 收缩总数
				case "kafka_server_replicamanager_isrshrinks_total":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_isr_shrinks_total",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// ISR 扩展总数
				case "kafka_server_replicamanager_isrexpands_total":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_isr_expands_total",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// ISR 更新失败总数
				case "kafka_server_replicamanager_failedisrupdates_total":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_isr_updates_failed_total",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 分区总数
				case "kafka_server_replicamanager_partitioncount":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_partition_count",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 正在迁移分区数
				case "kafka_server_replicamanager_reassigningpartitions":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_reassigning_partitions",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// ============ 批次5.5：延迟操作指标 ============

				// 延迟操作数
				case "kafka_server_delayedoperationpurgatory_numdelayedoperations":
					if purgatoryName, ok := m.Labels["delayedOperation"]; ok {
						labels := copyLabels(brokerLabels)
						labels["purgatory"] = purgatoryName
						vmMetrics = append(vmMetrics, victoriametrics.Metric{
							Name:   "kafka_broker_delayed_operations",
							Value:  m.Value,
							Labels: labels,
						})
					}

				// Purgatory 大小
				case "kafka_server_delayedoperationpurgatory_purgatorysize":
					if purgatoryName, ok := m.Labels["delayedOperation"]; ok {
						labels := copyLabels(brokerLabels)
						labels["purgatory"] = purgatoryName
						vmMetrics = append(vmMetrics, victoriametrics.Metric{
							Name:   "kafka_broker_purgatory_size",
							Value:  m.Value,
							Labels: labels,
						})
					}

				// Fetch 延迟过期总数
				case "kafka_server_delayedfetchmetrics_expires_total":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_delayed_fetch_expires_total",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// ============ 批次6：网络/线程指标 ============

				// 响应队列大小
				case "kafka_network_requestchannel_responsequeuesize":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_response_queue_size",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// Processor 空闲率
				case "kafka_network_processor_idlepercent":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_processor_idle_percent",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 网络 Processor 平均空闲率
				case "kafka_network_socketserver_networkprocessoravgidlepercent":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_network_processor_avg_idle_percent",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 请求处理线程空闲率
				case "kafka_server_kafkarequesthandlerpool_requesthandleravgidle_percent":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_request_handler_avg_idle_percent",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 已过期连接数
				case "kafka_network_socketserver_expiredconnectionskilledcount":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_expired_connections_killed_count",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 内存池可用量
				case "kafka_network_socketserver_memorypoolavailable":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_memory_pool_available",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 内存池已用量
				case "kafka_network_socketserver_memorypoolused":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_memory_pool_used",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// ============ 批次7：Broker 状态指标 ============

				// Broker 状态
				case "kafka_server_kafkaserver_brokerstate":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_state",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 磁盘读取速率
				case "kafka_server_kafkaserver_linux_disk_read_bytes":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_disk_read_bytes",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 磁盘写入速率
				case "kafka_server_kafkaserver_linux_disk_write_bytes":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_disk_write_bytes",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 离线日志目录数
				case "kafka_log_logmanager_offlinelogdirectorycount":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_offline_log_directory_count",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 日志目录离线状态
				case "kafka_log_logmanager_logdirectoryoffline":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_log_directory_offline",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// ============ 批次7.5：Log Flush 指标 ============

				// 日志 Flush 耗时
				case "kafka_log_logflushstats_logflushrateandtimems":
					if quantile, ok := m.Labels["quantile"]; ok && quantile == "0.99" {
						vmMetrics = append(vmMetrics, victoriametrics.Metric{
							Name:   "kafka_broker_log_flush_time_ms",
							Value:  m.Value,
							Labels: brokerLabels,
						})
					}

				// ============ 批次7.6：系统进程指标 ============

				// 进程 CPU 使用
				case "process_cpu_seconds_total":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_process_cpu_seconds_total",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 进程驻留内存
				case "process_resident_memory_bytes":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_process_resident_memory_bytes",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 进程虚拟内存
				case "process_virtual_memory_bytes":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_process_virtual_memory_bytes",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 进程启动时间
				case "process_start_time_seconds":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_process_start_time_seconds",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 最大文件描述符
				case "process_max_fds":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_process_max_fds",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 已用文件描述符
				case "process_open_fds":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_process_open_fds",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// ============ 批次7.7：Consumer Group 状态指标 ============

				// Consumer Group 总数
				case "kafka_coordinator_group_groupmetadatamanager_numgroups":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_consumer_group_count",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// Stable 状态消费组数
				case "kafka_coordinator_group_groupmetadatamanager_numgroupsstable":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_consumer_group_stable_count",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// Empty 状态消费组数
				case "kafka_coordinator_group_groupmetadatamanager_numgroupsempty":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_consumer_group_empty_count",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// Preparing Rebalance 数
				case "kafka_coordinator_group_groupmetadatamanager_numgroupspreparingrebalance":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_consumer_group_preparing_rebalance_count",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// Completing Rebalance 数
				case "kafka_coordinator_group_groupmetadatamanager_numgroupscompletingrebalance":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_consumer_group_completing_rebalance_count",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// Dead 状态消费组数
				case "kafka_coordinator_group_groupmetadatamanager_numgroupsdead":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_consumer_group_dead_count",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// Offset 总数
				case "kafka_coordinator_group_groupmetadatamanager_numoffsets":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_consumer_group_offsets_count",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// ============ 批次8：Topic 分区级指标 ============

				// Topic 日志大小
				case "kafka_log_log_size":
					if topic, ok := m.Labels["topic"]; ok {
						if partition, ok := m.Labels["partition"]; ok {
							labels := copyLabels(brokerLabels)
							labels["topic"] = topic
							labels["partition"] = partition
							vmMetrics = append(vmMetrics, victoriametrics.Metric{
								Name:   "kafka_topic_log_size",
								Value:  m.Value,
								Labels: labels,
							})
						}
					}

				// Topic LogEndOffset
				case "kafka_log_log_logendoffset":
					if topic, ok := m.Labels["topic"]; ok {
						if partition, ok := m.Labels["partition"]; ok {
							labels := copyLabels(brokerLabels)
							labels["topic"] = topic
							labels["partition"] = partition
							vmMetrics = append(vmMetrics, victoriametrics.Metric{
								Name:   "kafka_topic_log_end_offset",
								Value:  m.Value,
								Labels: labels,
							})
						}
					}

				// Topic LogStartOffset
				case "kafka_log_log_logstartoffset":
					if topic, ok := m.Labels["topic"]; ok {
						if partition, ok := m.Labels["partition"]; ok {
							labels := copyLabels(brokerLabels)
							labels["topic"] = topic
							labels["partition"] = partition
							vmMetrics = append(vmMetrics, victoriametrics.Metric{
								Name:   "kafka_topic_log_start_offset",
								Value:  m.Value,
								Labels: labels,
							})
						}
					}

				// Topic 日志段数量
				case "kafka_log_log_numlogsegments":
					if topic, ok := m.Labels["topic"]; ok {
						if partition, ok := m.Labels["partition"]; ok {
							labels := copyLabels(brokerLabels)
							labels["topic"] = topic
							labels["partition"] = partition
							vmMetrics = append(vmMetrics, victoriametrics.Metric{
								Name:   "kafka_topic_log_num_segments",
								Value:  m.Value,
								Labels: labels,
							})
						}
					}

				// Topic 分区 Under Replicated
				case "kafka_cluster_partition_underreplicated":
					if topic, ok := m.Labels["topic"]; ok {
						if partition, ok := m.Labels["partition"]; ok {
							labels := copyLabels(brokerLabels)
							labels["topic"] = topic
							labels["partition"] = partition
							vmMetrics = append(vmMetrics, victoriametrics.Metric{
								Name:   "kafka_topic_partition_under_replicated",
								Value:  m.Value,
								Labels: labels,
							})
						}
					}

				// Topic 分区 Under MinISR
				case "kafka_cluster_partition_underminisr":
					if topic, ok := m.Labels["topic"]; ok {
						if partition, ok := m.Labels["partition"]; ok {
							labels := copyLabels(brokerLabels)
							labels["topic"] = topic
							labels["partition"] = partition
							vmMetrics = append(vmMetrics, victoriametrics.Metric{
								Name:   "kafka_topic_partition_under_min_isr",
								Value:  m.Value,
								Labels: labels,
							})
						}
					}

				// Topic 分区 ISR 数
				case "kafka_cluster_partition_insyncreplicascount":
					if topic, ok := m.Labels["topic"]; ok {
						if partition, ok := m.Labels["partition"]; ok {
							labels := copyLabels(brokerLabels)
							labels["topic"] = topic
							labels["partition"] = partition
							vmMetrics = append(vmMetrics, victoriametrics.Metric{
								Name:   "kafka_topic_partition_isr_count",
								Value:  m.Value,
								Labels: labels,
							})
						}
					}

				// Topic 分区副本数
				case "kafka_cluster_partition_replicascount":
					if topic, ok := m.Labels["topic"]; ok {
						if partition, ok := m.Labels["partition"]; ok {
							labels := copyLabels(brokerLabels)
							labels["topic"] = topic
							labels["partition"] = partition
							vmMetrics = append(vmMetrics, victoriametrics.Metric{
								Name:   "kafka_topic_partition_replica_count",
								Value:  m.Value,
								Labels: labels,
							})
						}
					}

				// ============ 批次9：Log Cleaner 指标 ============

				// 最大脏比例
				case "kafka_log_logcleanermanager_max_dirty_percent":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_log_cleaner_max_dirty_percent",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 上次清理间隔
				case "kafka_log_logcleanermanager_time_since_last_run_ms":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_log_cleaner_time_since_last_run_ms",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 不可清理字节数
				case "kafka_log_logcleanermanager_uncleanable_bytes":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_log_cleaner_uncleanable_bytes",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 不可清理分区数
				case "kafka_log_logcleanermanager_uncleanable_partitions_count":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_log_cleaner_uncleanable_partitions_count",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// Cleaner 重新复制比例
				case "kafka_log_logcleaner_cleaner_recopy_percent":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_log_cleaner_recopy_percent",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// Cleaner 死线程数
				case "kafka_log_logcleaner_deadthreadcount":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_log_cleaner_dead_thread_count",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// Cleaner 最大缓冲利用率
				case "kafka_log_logcleaner_max_buffer_utilization_percent":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_log_cleaner_max_buffer_utilization_percent",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// Cleaner 最大清理时间
				case "kafka_log_logcleaner_max_clean_time_secs":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_log_cleaner_max_clean_time_secs",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// Cleaner 最大压缩延迟
				case "kafka_log_logcleaner_max_compaction_delay_secs":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_log_cleaner_max_compaction_delay_secs",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// ============ 批次10：JVM 指标 ============

				// GC 耗时
				case "jvm_gc_collection_seconds_sum":
					if gc, ok := m.Labels["gc"]; ok {
						labels := copyLabels(brokerLabels)
						labels["gc"] = gc
						vmMetrics = append(vmMetrics, victoriametrics.Metric{
							Name:   "kafka_broker_jvm_gc_seconds_sum",
							Value:  m.Value,
							Labels: labels,
						})
					}

				// GC 次数
				case "jvm_gc_collection_seconds_count":
					if gc, ok := m.Labels["gc"]; ok {
						labels := copyLabels(brokerLabels)
						labels["gc"] = gc
						vmMetrics = append(vmMetrics, victoriametrics.Metric{
							Name:   "kafka_broker_jvm_gc_count",
							Value:  m.Value,
							Labels: labels,
						})
					}

				// 内存池已用
				case "jvm_memory_pool_collection_used_bytes":
					if pool, ok := m.Labels["pool"]; ok {
						labels := copyLabels(brokerLabels)
						labels["pool"] = pool
						vmMetrics = append(vmMetrics, victoriametrics.Metric{
							Name:   "kafka_broker_jvm_memory_pool_used_bytes",
							Value:  m.Value,
							Labels: labels,
						})
					}

				// 内存池最大
				case "jvm_memory_pool_collection_max_bytes":
					if pool, ok := m.Labels["pool"]; ok {
						labels := copyLabels(brokerLabels)
						labels["pool"] = pool
						vmMetrics = append(vmMetrics, victoriametrics.Metric{
							Name:   "kafka_broker_jvm_memory_pool_max_bytes",
							Value:  m.Value,
							Labels: labels,
						})
					}

				// 当前线程数
				case "jvm_threads_current":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_jvm_threads_current",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// 死锁线程数
				case "jvm_threads_deadlocked":
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name:   "kafka_broker_jvm_threads_deadlocked",
						Value:  m.Value,
						Labels: brokerLabels,
					})

				// Buffer 池已用
				case "jvm_buffer_pool_used_bytes":
					if pool, ok := m.Labels["pool"]; ok {
						labels := copyLabels(brokerLabels)
						labels["pool"] = pool
						vmMetrics = append(vmMetrics, victoriametrics.Metric{
							Name:   "kafka_broker_jvm_buffer_pool_used_bytes",
							Value:  m.Value,
							Labels: labels,
						})
					}

				// ============ 批次10.5：分区 Last Stable Offset Lag ============

				// 分区 Last Stable Offset Lag
				case "kafka_cluster_partition_laststableoffsetlag":
					if topic, ok := m.Labels["topic"]; ok {
						if partition, ok := m.Labels["partition"]; ok {
							labels := copyLabels(brokerLabels)
							labels["topic"] = topic
							labels["partition"] = partition
							vmMetrics = append(vmMetrics, victoriametrics.Metric{
								Name:   "kafka_topic_partition_last_stable_offset_lag",
								Value:  m.Value,
								Labels: labels,
							})
						}
					}
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
