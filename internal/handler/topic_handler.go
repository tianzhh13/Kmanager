package handler

import (
	"net/http"
	"strconv"

	"kafka-management-platform/internal/middleware"
	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/service/auth"
	"kafka-management-platform/internal/service/topic"

	"github.com/gin-gonic/gin"
)

// TopicHandler Topic 处理器
type TopicHandler struct {
	topicSvc      *topic.Service
	permissionSvc *auth.PermissionService
}

// NewTopicHandler 创建 Topic 处理器实例
func NewTopicHandler(topicSvc *topic.Service, permissionSvc *auth.PermissionService) *TopicHandler {
	return &TopicHandler{
		topicSvc:      topicSvc,
		permissionSvc: permissionSvc,
	}
}

// CreateTopic 创建 Topic
func (h *TopicHandler) CreateTopic(c *gin.Context) {
	var req topic.CreateTopicRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request parameters"})
		return
	}

	if err := h.topicSvc.CreateTopic(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "topic created successfully"})
}

// DeleteTopic 删除 Topic
func (h *TopicHandler) DeleteTopic(c *gin.Context) {
	clusterIDStr := c.Query("cluster_id")
	topicName := c.Param("name")

	clusterID, err := strconv.ParseInt(clusterIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cluster_id"})
		return
	}

	if err := h.topicSvc.DeleteTopic(c.Request.Context(), clusterID, topicName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "topic deleted successfully"})
}

// UpdateTopicConfig 更新 Topic 配置
func (h *TopicHandler) UpdateTopicConfig(c *gin.Context) {
	var req topic.UpdateTopicConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request parameters"})
		return
	}

	if err := h.topicSvc.UpdateTopicConfig(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "topic config updated successfully"})
}

// GetTopicConfig 获取 Topic 配置
func (h *TopicHandler) GetTopicConfig(c *gin.Context) {
	clusterIDStr := c.Query("cluster_id")
	topicName := c.Param("name")

	clusterID, err := strconv.ParseInt(clusterIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cluster_id"})
		return
	}

	config, err := h.topicSvc.GetTopicConfig(c.Request.Context(), clusterID, topicName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": config})
}

// GetTopicConsumerGroups 获取 Topic 的消费组列表
func (h *TopicHandler) GetTopicConsumerGroups(c *gin.Context) {
	clusterIDStr := c.Query("cluster_id")
	topicName := c.Param("name")

	clusterID, err := strconv.ParseInt(clusterIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cluster_id"})
		return
	}

	groups, err := h.topicSvc.GetTopicConsumerGroups(c.Request.Context(), clusterID, topicName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": groups})
}

// GetTopic 获取 Topic 详情
func (h *TopicHandler) GetTopic(c *gin.Context) {
	clusterIDStr := c.Query("cluster_id")
	topicName := c.Param("name")

	clusterID, err := strconv.ParseInt(clusterIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cluster_id"})
		return
	}

	topic, err := h.topicSvc.GetTopic(c.Request.Context(), clusterID, topicName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, topic)
}

// ListTopics 列出 Topic
func (h *TopicHandler) ListTopics(c *gin.Context) {
	var req topic.ListTopicsRequest

	if clusterIDStr := c.Query("cluster_id"); clusterIDStr != "" {
		clusterID, err := strconv.ParseInt(clusterIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cluster_id"})
			return
		}
		req.ClusterID = clusterID
	}

	req.Search = c.Query("search")

	// 分页参数：前端传 page/page_size，转为 offset/limit
	page := 1
	pageSize := 20

	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if psStr := c.Query("page_size"); psStr != "" {
		if ps, err := strconv.Atoi(psStr); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	req.Offset = (page - 1) * pageSize
	req.Limit = pageSize

	// 获取当前用户信息
	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	// 普通用户需要过滤 Topic
	if userRole == string(models.RoleNormalUser) && req.ClusterID > 0 {
		allowedTopics, err := h.permissionSvc.GetAllowedTopics(c.Request.Context(), userID, req.ClusterID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		// 如果用户没有任何 Topic 权限，返回空列表
		if len(allowedTopics) == 0 {
			c.JSON(http.StatusOK, gin.H{"data": []interface{}{}, "total": 0})
			return
		}
		req.AllowedTopics = allowedTopics
	}

	resp, err := h.topicSvc.ListTopics(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// SyncTopics 同步 Topic
func (h *TopicHandler) SyncTopics(c *gin.Context) {
	clusterIDStr := c.Param("id")
	clusterID, err := strconv.ParseInt(clusterIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cluster_id"})
		return
	}

	if err := h.topicSvc.SyncTopics(c.Request.Context(), clusterID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "topics synced successfully"})
}
