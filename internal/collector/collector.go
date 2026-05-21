package collector

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"kafka-management-platform/internal/config"
	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/repository"
	"kafka-management-platform/internal/service/monitor"
	"kafka-management-platform/pkg/encryption"
	"kafka-management-platform/pkg/kafka"
	"kafka-management-platform/pkg/victoriametrics"
)

// Collector 数据采集器（独立进程）
// 负责从 Kafka 集群采集指标并写入 VictoriaMetrics，支持多集群并行采集
type Collector struct {
	cfg             *config.Config
	clusterRepo     repository.ClusterRepository
	encryptSvc      *encryption.Service
	monitorSvc      *monitor.Service
	vmClient        *victoriametrics.Client
	adminPool       sync.Map // map[int64]*kafka.AdminClient
	kerberosBaseDir string
	concurrency     int
	semaphore       chan struct{}
	syncInterval    time.Duration
	stopCh          chan struct{}
	wg              sync.WaitGroup
}

// NewCollector 创建 Collector
func NewCollector(cfg *config.Config, clusterRepo repository.ClusterRepository) *Collector {
	var encryptSvc *encryption.Service
	if cfg.Encryption.Key != "" {
		var err error
		encryptSvc, err = encryption.NewService(cfg.Encryption.Key)
		if err != nil {
			log.Printf("Warning: failed to create encryption service: %v, auth config will not be decrypted", err)
			encryptSvc = nil
		}
	}

	vmClient := victoriametrics.NewClient(
		cfg.VictoriaMetrics.WriteURL,
		cfg.VictoriaMetrics.QueryURL,
		cfg.VictoriaMetrics.Enabled,
	)

	kerberosBaseDir := "./kerberos"
	monitorSvc := monitor.NewService(clusterRepo, encryptSvc, vmClient, kerberosBaseDir)

	concurrency := cfg.Collector.Concurrency
	if concurrency <= 0 {
		concurrency = 10
	}

	interval := 30
	if cfg.Collector.Interval > 0 {
		interval = cfg.Collector.Interval
	}
	syncInterval := time.Duration(interval) * time.Second

	log.Printf("Collector config: concurrency=%d, interval=%v", concurrency, syncInterval)

	return &Collector{
		cfg:             cfg,
		clusterRepo:     clusterRepo,
		encryptSvc:      encryptSvc,
		monitorSvc:      monitorSvc,
		vmClient:        vmClient,
		kerberosBaseDir: kerberosBaseDir,
		concurrency:     concurrency,
		semaphore:       make(chan struct{}, concurrency),
		syncInterval:    syncInterval,
		stopCh:          make(chan struct{}),
	}
}

// Start 启动 Collector
func (c *Collector) Start() error {
	log.Println("Starting collector...")
	c.wg.Add(1)
	go c.runLoop()
	log.Println("Collector started")
	return nil
}

// Stop 停止 Collector
func (c *Collector) Stop() error {
	log.Println("Stopping collector...")

	// 关闭所有 Admin Client
	c.adminPool.Range(func(key, value interface{}) bool {
		if client, ok := value.(*kafka.AdminClient); ok {
			client.Close()
		}
		c.adminPool.Delete(key)
		return true
	})

	close(c.stopCh)
	c.wg.Wait()
	log.Println("Collector stopped")
	return nil
}

// runLoop 运行定时采集循环
func (c *Collector) runLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.syncInterval)
	defer ticker.Stop()

	// 立即执行一次
	c.collectAll()

	for {
		select {
		case <-ticker.C:
			c.collectAll()
		case <-c.stopCh:
			return
		}
	}
}

// collectAll 并发采集所有集群
func (c *Collector) collectAll() {
	if !c.vmClient.IsEnabled() {
		return
	}

	ctx := context.Background()
	clusters, _, err := c.clusterRepo.List(ctx, 0, 1000)
	if err != nil {
		log.Printf("[Collector] Failed to list clusters: %v", err)
		return
	}

	if len(clusters) == 0 {
		return
	}

	startTime := time.Now()
	log.Printf("[Collector] Starting collection for %d clusters (concurrency=%d)", len(clusters), c.concurrency)

	var (
		mu           sync.Mutex
		success      int32
		fail         int32
		totalMetrics int
	)

	var wg sync.WaitGroup
	for _, cluster := range clusters {
		c.semaphore <- struct{}{} // 占位，满了就阻塞等待
		wg.Add(1)
		go func(cl *models.Cluster) {
			defer wg.Done()
			defer func() { <-c.semaphore }()

			metricCount, err := c.collectCluster(ctx, cl)
			if err != nil {
				log.Printf("[Collector] Failed to collect cluster %d (%s): %v", cl.ClusterID, cl.ClusterName, err)
				mu.Lock()
				fail++
				mu.Unlock()
				return
			}
			mu.Lock()
			success++
			totalMetrics += metricCount
			mu.Unlock()
		}(cluster)
	}
	wg.Wait()

	elapsed := time.Since(startTime)
	log.Printf("[Collector] Collection completed: %d/%d clusters success, %d metrics written in %v",
		success, len(clusters), totalMetrics, elapsed)
}

// collectCluster 采集单个集群的指标（AdminClient + JMX 并行，合并写入 VM）
func (c *Collector) collectCluster(ctx context.Context, cluster *models.Cluster) (int, error) {
	// 预获取共享数据：分区详情（Admin 和 JMX 都需要，只请求一次）
	partitionDetails, err := c.monitorSvc.GetTopicPartitionDetails(ctx, cluster.ClusterID)
	if err != nil {
		log.Printf("[Collector] Failed to get partition details for cluster %d: %v", cluster.ClusterID, err)
		partitionDetails = nil
	}

	// 并行采集 AdminClient 指标 + JMX 指标
	var adminMetrics, jmxMetrics []victoriametrics.Metric
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		adminMetrics = c.collectAdminMetrics(ctx, cluster, partitionDetails)
	}()
	go func() {
		defer wg.Done()
		jmxMetrics = c.collectJMXMetrics(ctx, cluster, partitionDetails)
	}()
	wg.Wait()

	// 合并写入 VM（一次 HTTP 请求）
	all := append(adminMetrics, jmxMetrics...)
	if len(all) == 0 {
		return 0, nil
	}

	if err := c.vmClient.Write(ctx, all); err != nil {
		return 0, fmt.Errorf("failed to write metrics to VM: %w", err)
	}

	return len(all), nil
}
