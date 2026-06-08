package hostmapping

import (
	"context"
	"sync"
	"time"

	"kafka-management-platform/internal/logger"
	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/repository"
)

// Service 主机映射服务（带内存缓存）
type Service struct {
	repo  *repository.HostMappingRepository
	cache sync.Map // map[string]string hostname -> ip
	mu    sync.RWMutex
}

// NewService 创建主机映射服务实例
func NewService(repo *repository.HostMappingRepository) *Service {
	svc := &Service{
		repo: repo,
	}
	// 启动时加载缓存
	svc.refreshCache()
	// 后台定时刷新（30 秒）
	go svc.backgroundRefresh()
	return svc
}

// Resolve 解析主机名，返回 IP 地址
// 如果有映射返回映射的 IP，否则返回原始主机名（走系统 DNS）
func (s *Service) Resolve(hostname string) string {
	if ip, ok := s.cache.Load(hostname); ok {
		return ip.(string)
	}
	return hostname
}

// Create 创建主机映射
func (s *Service) Create(ctx context.Context, mapping *models.HostMapping) error {
	if err := s.repo.Create(ctx, mapping); err != nil {
		return err
	}
	s.cache.Store(mapping.Hostname, mapping.IPAddress)
	return nil
}

// Update 更新主机映射
func (s *Service) Update(ctx context.Context, mapping *models.HostMapping) error {
	// 获取旧映射，如果主机名变了需要删除旧缓存
	old, _ := s.repo.GetByID(ctx, mapping.ID)
	if err := s.repo.Update(ctx, mapping); err != nil {
		return err
	}
	if old != nil && old.Hostname != mapping.Hostname {
		s.cache.Delete(old.Hostname)
	}
	s.cache.Store(mapping.Hostname, mapping.IPAddress)
	return nil
}

// Delete 删除主机映射
func (s *Service) Delete(ctx context.Context, id int64) error {
	mapping, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	if mapping != nil {
		s.cache.Delete(mapping.Hostname)
	}
	return nil
}

// GetByID 根据 ID 获取主机映射
func (s *Service) GetByID(ctx context.Context, id int64) (*models.HostMapping, error) {
	return s.repo.GetByID(ctx, id)
}

// List 获取所有主机映射
func (s *Service) List(ctx context.Context) ([]models.HostMapping, error) {
	return s.repo.List(ctx)
}

// ListWithPagination 分页获取主机映射
func (s *Service) ListWithPagination(ctx context.Context, page, pageSize int, keyword string) ([]models.HostMapping, int64, error) {
	return s.repo.ListWithPagination(ctx, page, pageSize, keyword)
}

// refreshCache 从数据库刷新缓存
func (s *Service) refreshCache() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mappings, err := s.repo.List(ctx)
	if err != nil {
		logger.Warn("Failed to refresh host mapping cache", "error", err)
		return
	}

	// 构建新缓存
	newCache := make(map[string]string, len(mappings))
	for _, m := range mappings {
		newCache[m.Hostname] = m.IPAddress
	}

	// 更新 sync.Map：先存新的，再删旧的
	s.mu.Lock()
	// 收集当前所有 key
	var oldKeys []string
	s.cache.Range(func(key, value interface{}) bool {
		oldKeys = append(oldKeys, key.(string))
		return true
	})
	// 存入新数据
	for k, v := range newCache {
		s.cache.Store(k, v)
	}
	// 删除已不存在的 key
	for _, k := range oldKeys {
		if _, exists := newCache[k]; !exists {
			s.cache.Delete(k)
		}
	}
	s.mu.Unlock()

	logger.Info("Host mapping cache refreshed", "count", len(mappings))
}

// backgroundRefresh 后台定时刷新缓存
func (s *Service) backgroundRefresh() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		s.refreshCache()
	}
}
