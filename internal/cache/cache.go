package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"kafka-management-platform/internal/logger"
	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/repository"
)

// CacheItem 缓存项
type CacheItem struct {
	Value      interface{}
	Expiration time.Time
}

// MemoryCache 内存缓存实现
type MemoryCache struct {
	items      map[string]*CacheItem
	mu         sync.RWMutex
	defaultTTL time.Duration
	maxItems   int // 最大缓存条目数，0 表示不限制
	stopCh     chan struct{}
}

// NewMemoryCache 创建内存缓存
func NewMemoryCache(defaultTTL time.Duration) *MemoryCache {
	return NewMemoryCacheWithCap(defaultTTL, 10000)
}

// NewMemoryCacheWithCap 创建带容量限制的内存缓存
func NewMemoryCacheWithCap(defaultTTL time.Duration, maxItems int) *MemoryCache {
	cache := &MemoryCache{
		items:      make(map[string]*CacheItem),
		defaultTTL: defaultTTL,
		maxItems:   maxItems,
		stopCh:     make(chan struct{}),
	}

	// 启动过期清理 goroutine
	go cache.cleanupExpired()

	return cache
}

// Stop 停止清理 goroutine
func (c *MemoryCache) Stop() {
	close(c.stopCh)
}

// Get 获取缓存值
func (c *MemoryCache) Get(ctx context.Context, key string) (interface{}, error) {
	c.mu.RLock()
	item, exists := c.items[key]
	if !exists {
		c.mu.RUnlock()
		return nil, nil
	}

	// 检查是否过期
	if time.Now().After(item.Expiration) {
		c.mu.RUnlock()
		// 过期条目需要写锁删除
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		return nil, nil
	}

	c.mu.RUnlock()
	return item.Value, nil
}

// Set 设置缓存值
func (c *MemoryCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	expiration := time.Now().Add(ttl)
	if ttl == 0 {
		expiration = time.Now().Add(c.defaultTTL)
	}

	c.items[key] = &CacheItem{
		Value:      value,
		Expiration: expiration,
	}

	// 容量限制：超过上限时清理过期条目，仍超则删除最旧的
	if c.maxItems > 0 && len(c.items) > c.maxItems {
		c.evictExpiredLocked()
		if len(c.items) > c.maxItems {
			c.evictOldestLocked()
		}
	}

	return nil
}

// Delete 删除缓存
func (c *MemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
	return nil
}

// Clear 清空缓存
func (c *MemoryCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*CacheItem)
	return nil
}

// evictExpiredLocked 清理过期缓存（调用前需持有锁）
func (c *MemoryCache) evictExpiredLocked() {
	now := time.Now()
	for key, item := range c.items {
		if now.After(item.Expiration) {
			delete(c.items, key)
		}
	}
}

// evictOldestLocked 淘汰最早过期的缓存条目（调用前需持有锁）
func (c *MemoryCache) evictOldestLocked() {
	if len(c.items) == 0 {
		return
	}
	// 找到最早过期的 key
	var oldestKey string
	var oldestExp time.Time
	for key, item := range c.items {
		if oldestExp.IsZero() || item.Expiration.Before(oldestExp) {
			oldestKey = key
			oldestExp = item.Expiration
		}
	}
	if oldestKey != "" {
		delete(c.items, oldestKey)
	}
}

// cleanupExpired 清理过期缓存
func (c *MemoryCache) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			now := time.Now()
			for key, item := range c.items {
				if now.After(item.Expiration) {
					delete(c.items, key)
				}
			}
			c.mu.Unlock()
		case <-c.stopCh:
			return
		}
	}
}

// Cache 缓存接口
type Cache interface {
	Get(ctx context.Context, key string) (interface{}, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
}

// UserCache 用户信息缓存
type UserCache struct {
	cache Cache
	ttl   time.Duration
}

// NewUserCache 创建用户缓存
func NewUserCache(cache Cache) *UserCache {
	return &UserCache{
		cache: cache,
		ttl:   15 * time.Minute,
	}
}

// GetUser 获取用户缓存
func (uc *UserCache) GetUser(ctx context.Context, userID int64) (interface{}, error) {
	key := fmt.Sprintf("user:%d", userID)
	return uc.cache.Get(ctx, key)
}

// SetUser 设置用户缓存
func (uc *UserCache) SetUser(ctx context.Context, userID int64, value interface{}) error {
	key := fmt.Sprintf("user:%d", userID)
	return uc.cache.Set(ctx, key, value, uc.ttl)
}

// InvalidateUser 失效用户缓存
func (uc *UserCache) InvalidateUser(ctx context.Context, userID int64) error {
	key := fmt.Sprintf("user:%d", userID)
	return uc.cache.Delete(ctx, key)
}

// ClusterCache 集群配置缓存
type ClusterCache struct {
	cache Cache
	ttl   time.Duration
}

// NewClusterCache 创建集群缓存
func NewClusterCache(cache Cache) *ClusterCache {
	return &ClusterCache{
		cache: cache,
		ttl:   5 * time.Minute,
	}
}

// GetCluster 获取集群缓存
func (cc *ClusterCache) GetCluster(ctx context.Context, clusterID int64) (interface{}, error) {
	key := fmt.Sprintf("cluster:%d", clusterID)
	return cc.cache.Get(ctx, key)
}

// SetCluster 设置集群缓存
func (cc *ClusterCache) SetCluster(ctx context.Context, clusterID int64, value interface{}) error {
	key := fmt.Sprintf("cluster:%d", clusterID)
	return cc.cache.Set(ctx, key, value, cc.ttl)
}

// InvalidateCluster 失效集群缓存
func (cc *ClusterCache) InvalidateCluster(ctx context.Context, clusterID int64) error {
	key := fmt.Sprintf("cluster:%d", clusterID)
	return cc.cache.Delete(ctx, key)
}

// TokenBlacklistCache Token 黑名单缓存（内存 + 数据库双写）
type TokenBlacklistCache struct {
	cache Cache
	repo  repository.TokenBlacklistRepository
	ttl   time.Duration
}

// NewTokenBlacklistCache 创建 Token 黑名单缓存
func NewTokenBlacklistCache(cache Cache) *TokenBlacklistCache {
	return &TokenBlacklistCache{
		cache: cache,
		ttl:   24 * time.Hour, // Token 有效期通常为 24 小时
	}
}

// SetRepository 设置数据库仓库（用于持久化）
func (tbc *TokenBlacklistCache) SetRepository(repo repository.TokenBlacklistRepository) {
	tbc.repo = repo
}

// hashToken 计算 Token 的 SHA256 哈希（避免存储原始 Token）
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// IsBlacklisted 检查 Token 是否在黑名单中（先查内存，再查数据库）
func (tbc *TokenBlacklistCache) IsBlacklisted(ctx context.Context, token string) (bool, error) {
	key := fmt.Sprintf("blacklist:%s", token)
	val, err := tbc.cache.Get(ctx, key)
	if err == nil && val != nil {
		return true, nil
	}

	// 内存未命中，查数据库
	if tbc.repo != nil {
		tokenHash := hashToken(token)
		dbResult, dbErr := tbc.repo.IsBlacklisted(ctx, tokenHash)
		if dbErr == nil && dbResult {
			// 数据库命中，回填内存缓存
			_ = tbc.cache.Set(ctx, key, true, tbc.ttl)
			return true, nil
		}
	}

	return false, nil
}

// AddToBlacklist 将 Token 加入黑名单（内存 + 数据库双写）
func (tbc *TokenBlacklistCache) AddToBlacklist(ctx context.Context, token string) error {
	key := fmt.Sprintf("blacklist:%s", token)
	// 写入内存缓存
	if err := tbc.cache.Set(ctx, key, true, tbc.ttl); err != nil {
		return err
	}

	// 写入数据库
	if tbc.repo != nil {
		tokenHash := hashToken(token)
		entry := &models.TokenBlacklist{
			TokenHash: tokenHash,
			ExpiresAt: time.Now().Add(tbc.ttl),
		}
		if err := tbc.repo.Create(ctx, entry); err != nil {
			logger.Warn("Failed to persist token to blacklist DB", "error", err)
			// 数据库写入失败不影响业务（内存已写入）
		}
	}

	return nil
}

// LoadFromDB 从数据库加载活跃黑名单到内存缓存（启动时调用）
func (tbc *TokenBlacklistCache) LoadFromDB(ctx context.Context) {
	if tbc.repo == nil {
		return
	}

	entries, err := tbc.repo.LoadActive(ctx)
	if err != nil {
		logger.Warn("Failed to load token blacklist from DB", "error", err)
		return
	}

	loaded := 0
	for _, entry := range entries {
		key := fmt.Sprintf("blacklist:db:%s", entry.TokenHash)
		remaining := time.Until(entry.ExpiresAt)
		if remaining <= 0 {
			continue
		}
		_ = tbc.cache.Set(ctx, key, true, remaining)
		loaded++
	}

	logger.Info("Loaded token blacklist from DB", "count", loaded, "total", len(entries))
}

// CleanupExpired 清理过期的黑名单记录
func (tbc *TokenBlacklistCache) CleanupExpired(ctx context.Context) {
	if tbc.repo == nil {
		return
	}

	deleted, err := tbc.repo.DeleteExpired(ctx)
	if err != nil {
		logger.Warn("Failed to cleanup expired token blacklist", "error", err)
		return
	}

	if deleted > 0 {
		logger.Info("Cleaned up expired token blacklist entries", "count", deleted)
	}
}

// JSONCache 支持 JSON 序列化的缓存
type JSONCache struct {
	cache Cache
}

// NewJSONCache 创建 JSON 缓存
func NewJSONCache(cache Cache) *JSONCache {
	return &JSONCache{cache: cache}
}

// GetJSON 获取 JSON 缓存
func (jc *JSONCache) GetJSON(ctx context.Context, key string, dest interface{}) error {
	val, err := jc.cache.Get(ctx, key)
	if err != nil {
		return err
	}
	if val == nil {
		return nil
	}

	data, ok := val.(string)
	if !ok {
		return fmt.Errorf("invalid cache value type")
	}

	return json.Unmarshal([]byte(data), dest)
}

// SetJSON 设置 JSON 缓存
func (jc *JSONCache) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return jc.cache.Set(ctx, key, string(data), ttl)
}

// GlobalCache 全局缓存实例
var globalCache Cache
var once sync.Once

// InitGlobalCache 初始化全局缓存
func InitGlobalCache(ttl time.Duration) {
	once.Do(func() {
		globalCache = NewMemoryCache(ttl)
		logger.Info("Global cache initialized", "ttl", ttl)
	})
}

// GetGlobalCache 获取全局缓存
func GetGlobalCache() Cache {
	if globalCache == nil {
		InitGlobalCache(5 * time.Minute)
	}
	return globalCache
}
