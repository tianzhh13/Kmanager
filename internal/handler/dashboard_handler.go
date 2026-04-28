package handler

import (
	"context"

	"kafka-management-platform/internal/repository"

	"github.com/gin-gonic/gin"
)

// DashboardHandler 仪表盘处理器
type DashboardHandler struct {
	clusterRepo   repository.ClusterRepository
	topicRepo     repository.TopicRepository
	scramUserRepo repository.ScramUserRepository
}

// NewDashboardHandler 创建仪表盘处理器实例
func NewDashboardHandler(
	clusterRepo repository.ClusterRepository,
	topicRepo repository.TopicRepository,
	scramUserRepo repository.ScramUserRepository,
) *DashboardHandler {
	return &DashboardHandler{
		clusterRepo:   clusterRepo,
		topicRepo:     topicRepo,
		scramUserRepo: scramUserRepo,
	}
}

// StatsResponse 统计数据响应
type StatsResponse struct {
	ClusterCount   int64 `json:"cluster_count"`
	TopicCount     int64 `json:"topic_count"`
	ScramUserCount int64 `json:"scram_user_count"`
}

// GetStats 获取仪表盘统计数据
func (h *DashboardHandler) GetStats(c *gin.Context) {
	ctx := c.Request.Context()

	stats := StatsResponse{}

	// 并行获取统计数据
	type result struct {
		clusterCount   int64
		topicCount     int64
		scramUserCount int64
		err            error
	}

	resultChan := make(chan result, 1)

	go func() {
		var r result
		r.clusterCount, r.err = h.getClusterCount(ctx)
		if r.err != nil {
			resultChan <- r
			return
		}

		r.topicCount, r.err = h.topicRepo.Count(ctx)
		if r.err != nil {
			resultChan <- r
			return
		}

		r.scramUserCount, r.err = h.scramUserRepo.Count(ctx)
		resultChan <- r
	}()

	r := <-resultChan
	if r.err != nil {
		c.JSON(500, gin.H{"error": r.err.Error()})
		return
	}

	stats.ClusterCount = r.clusterCount
	stats.TopicCount = r.topicCount
	stats.ScramUserCount = r.scramUserCount

	c.JSON(200, stats)
}

func (h *DashboardHandler) getClusterCount(ctx context.Context) (int64, error) {
	// 获取所有集群数量（管理员视角）
	// 这里简化处理，直接查询总数
	clusters, total, err := h.clusterRepo.ListByUser(ctx, 0, "super_admin", 0, 1)
	if err != nil {
		return 0, err
	}
	_ = clusters // 忽略返回的数据，只需要 total
	return total, nil
}
