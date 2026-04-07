package kafka

import (
	"sync"
	"time"

	"kafka-management-platform/internal/logger"
	"kafka-management-platform/internal/models"

	"github.com/IBM/sarama"
)

// AdminClientPool Kafka Admin 客户端连接池
type AdminClientPool struct {
	mu       sync.RWMutex
	clients  map[int64]*AdminClient
	factory  *ClientFactory
	cleanup  time.Duration
	maxConns int
}

// AdminClient 封装了 Kafka Admin 客户端
type AdminClient struct {
	Client   sarama.ClusterAdmin
	Cluster  *models.Cluster
	LastUsed time.Time
	mu       sync.Mutex
}

// NewAdminClientPool 创建新的连接池
func NewAdminClientPool(factory *ClientFactory, maxConns int, cleanup time.Duration) *AdminClientPool {
	pool := &AdminClientPool{
		clients:  make(map[int64]*AdminClient),
		factory:  factory,
		cleanup:  cleanup,
		maxConns: maxConns,
	}

	// 启动清理goroutine
	go pool.cleanupLoop()

	return pool
}

// Get 获取或创建 AdminClient
func (p *AdminClientPool) Get(cluster *models.Cluster) (*AdminClient, error) {
	p.mu.RLock()
	client, exists := p.clients[cluster.ID]
	p.mu.RUnlock()

	if exists {
		client.mu.Lock()
		client.LastUsed = time.Now()
		client.mu.Unlock()
		return client, nil
	}

	// 创建新客户端
	adminClient, err := p.factory.NewAdminClient(cluster)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	// 再次检查（双重检查锁定）
	if existing, ok := p.clients[cluster.ID]; ok {
		p.mu.Unlock()
		adminClient.Close()
		return existing, nil
	}

	// 检查连接数限制
	if len(p.clients) >= p.maxConns {
		p.mu.Unlock()
		// 尝试清理最旧的连接
		p.cleanupOldest()
		return nil, ErrTooManyConnections
	}

	client = &AdminClient{
		Client:   adminClient,
		Cluster:  cluster,
		LastUsed: time.Now(),
	}
	p.clients[cluster.ID] = client
	p.mu.Unlock()

	logger.Info("Created new Kafka admin client",
		"cluster_id", cluster.ID,
		"cluster_name", cluster.Name,
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

// CloseAll 关闭所有连接
func (p *AdminClientPool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for id, client := range p.clients {
		client.Client.Close()
		delete(p.clients, id)
	}
	logger.Info("Closed all Kafka admin clients")
}

// cleanupLoop 定期清理过期连接
func (p *AdminClientPool) cleanupLoop() {
	ticker := time.NewTicker(p.cleanup)
	defer ticker.Stop()

	for range ticker.C {
		p.cleanupExpired()
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

// cleanupOldest 清理最旧的连接
func (p *AdminClientPool) cleanupOldest() {
	p.mu.Lock()
	defer p.mu.Unlock()

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

// ErrTooManyConnections 连接数过多错误
var ErrTooManyConnections = &PoolError{Message: "too many connections"}

// PoolError 连接池错误
type PoolError struct {
	Message string
}

func (e *PoolError) Error() string {
	return e.Message
}