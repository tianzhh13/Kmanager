package handler

import (
	"net/http"
	"strconv"

	"kafka-management-platform/internal/service/topic"

	"github.com/gin-gonic/gin"
)

// TopicHandler Topic 处理器
type TopicHandler struct {
	topicSvc *topic.Service
}

// NewTopicHandler 创建 Topic 处理器实例
func NewTopicHandler(topicSvc *topic.Service) *TopicHandler {
	return &TopicHandler{
		topicSvc: topicSvc,
	}
}

// CreateTopic 创建 Topic
func (h *TopicHandler) CreateTopic(c *gin.Context) {
	var req topic.CreateTopicRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.topicSvc.UpdateTopicConfig(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "topic config updated successfully"})
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

	if offsetStr := c.Query("offset"); offsetStr != "" {
		offset, _ := strconv.Atoi(offsetStr)
		req.Offset = offset
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		limit, _ := strconv.Atoi(limitStr)
		req.Limit = limit
	} else {
		req.Limit = 20 // 默认每页 20 条
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
