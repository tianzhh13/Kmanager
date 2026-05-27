package kafka

import (
	"strconv"
	"sync"
	"time"

	"kafka-management-platform/internal/logger"
	"kafka-management-platform/internal/models"

	"golang.org/x/sync/singleflight"
)

// AdminClientPool Kafka Admin 客户端连接池
type AdminClientPool struct {
	mu       sync.RWMutex
	clients  map[int64]*PooledAdminClient
	cleanup  time.Duration
	maxConns int
	stopCh   chan struct{}
	sf       singleflight.Group // 防止同一集群并发重复创建连接
}

// PooledAdminClient 封装了 Kafka Admin 客户端（用于连接池）
type PooledAdminClient struct {
	Client   *AdminClient
	Cluster  *models.Cluster
	LastUsed time.Time
	mu       sync.Mutex
}

// NewAdminClientPool 创建新的连接池
func NewAdminClientPool(maxConns int, cleanup time.Duration) *AdminClientPool {
	pool := &AdminClientPool{
		clients:  make(map[int64]*PooledAdminClient),
		cleanup:  cleanup,
		maxConns: maxConns,
		stopCh:   make(chan struct{}),
	}

	// 启动清理goroutine
	go pool.cleanupLoop()

	return pool
}

// Get 获取或创建 AdminClient（使用 singleflight 防止并发重复创建）
func (p *AdminClientPool) Get(cluster *models.Cluster) (*PooledAdminClient, error) {
	p.mu.RLock()
	client, exists := p.clients[cluster.ClusterID]
	p.mu.RUnlock()

	if exists {
		client.mu.Lock()
		client.LastUsed = time.Now()
		client.mu.Unlock()
		return client, nil
	}

	// 使用 singleflight 确保同一 clusterID 只有一个创建操作
	key := strconv.FormatInt(cluster.ClusterID, 10)
	v, err, _ := p.sf.Do(key, func() (interface{}, error) {
		return p.createClient(cluster)
	})
	if err != nil {
		return nil, err
	}
	return v.(*PooledAdminClient), nil
}

// createClient 创建新的 AdminClient（内部方法，由 singleflight 调用）
func (p *AdminClientPool) createClient(cluster *models.Cluster) (*PooledAdminClient, error) {
	// 双重检查
	p.mu.RLock()
	if client, ok := p.clients[cluster.ClusterID]; ok {
		p.mu.RUnlock()
		client.mu.Lock()
		client.LastUsed = time.Now()
		client.mu.Unlock()
		return client, nil
	}
	p.mu.RUnlock()

	// 创建新客户端
	adminClient, err := NewAdminClient(cluster, cluster.AuthConfig)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// 第三次检查（singleflight 内）
	if existing, ok := p.clients[cluster.ClusterID]; ok {
		adminClient.Close()
		existing.mu.Lock()
		existing.LastUsed = time.Now()
		existing.mu.Unlock()
		return existing, nil
	}

	// 检查连接数限制
	if len(p.clients) >= p.maxConns {
		p.cleanupOldestLocked()
		return nil, ErrTooManyConnections
	}

	client := &PooledAdminClient{
		Client:   adminClient,
		Cluster:  cluster,
		LastUsed: time.Now(),
	}
	p.clients[cluster.ClusterID] = client

	logger.Info("Created new Kafka admin client",
		"cluster_id", cluster.ClusterID,
		"cluster_name", cluster.ClusterName,
	)

	return client, nil
}

// Put 归还连接
func (p *AdminClientPool) Put(clusterID int64) {
	p.mu.RLock()
	client, exists := p.clients[clusterID]
	p.mu.RUnlock()

	if exists {
		client.mu.Lock()
		client.LastUsed = time.Now()
		client.mu.Unlock()
	}
}

// Close 关闭指定集群的连接
func (p *AdminClientPool) Close(clusterID int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if client, exists := p.clients[clusterID]; exists {
		client.Client.Close()
		delete(p.clients, clusterID)
		logger.Info("Closed Kafka admin client", "cluster_id", clusterID)
	}
}

// CloseAll 关闭所有连接并停止清理 goroutine
func (p *AdminClientPool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for id, client := range p.clients {
		client.Client.Close()
		delete(p.clients, id)
	}
	logger.Info("Closed all Kafka admin clients")

	// 停止清理 goroutine
	select {
	case <-p.stopCh:
		// 已关闭
	default:
		close(p.stopCh)
	}
}

// cleanupLoop 定期清理过期连接
func (p *AdminClientPool) cleanupLoop() {
	ticker := time.NewTicker(p.cleanup)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.cleanupExpired()
		case <-p.stopCh:
			return
		}
	}
}

// cleanupExpired 清理过期连接
func (p *AdminClientPool) cleanupExpired() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for id, client := range p.clients {
		client.mu.Lock()
		if now.Sub(client.LastUsed) > p.cleanup {
			client.Client.Close()
			delete(p.clients, id)
			logger.Info("Cleaned up expired Kafka admin client", "cluster_id", id)
		}
		client.mu.Unlock()
	}
}

// cleanupOldest 清理最旧的连接（内部已加锁版本）
func (p *AdminClientPool) cleanupOldestLocked() {
	var oldestID int64
	var oldestTime time.Time

	for id, client := range p.clients {
		client.mu.Lock()
		if oldestTime.IsZero() || client.LastUsed.Before(oldestTime) {
			oldestID = id
			oldestTime = client.LastUsed
		}
		client.mu.Unlock()
	}

	if oldestID != 0 {
		client := p.clients[oldestID]
		client.Client.Close()
		delete(p.clients, oldestID)
		logger.Info("Cleaned up oldest Kafka admin client", "cluster_id", oldestID)
	}
}

// cleanupOldest 清理最旧的连接（公开版本，自行加锁）
func (p *AdminClientPool) cleanupOldest() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cleanupOldestLocked()
}

// ErrTooManyConnections 连接数过多错误
var ErrTooManyConnections = &PoolError{Message: "too many connections"}

// PoolError 连接池错误
type PoolError struct {
	Message string
}

func (e *PoolError) Error() string {
	return e.Message
}
