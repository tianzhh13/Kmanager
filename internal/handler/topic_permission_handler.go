package handler

import (
	"net/http"
	"strconv"

	"kafka-management-platform/internal/middleware"
	"kafka-management-platform/internal/service/auth"

	"github.com/gin-gonic/gin"
)

// TopicPermissionHandler Topic 权限处理器
type TopicPermissionHandler struct {
	topicPermSvc *auth.TopicPermissionService
}

// NewTopicPermissionHandler 创建 Topic 权限处理器实例
func NewTopicPermissionHandler(topicPermSvc *auth.TopicPermissionService) *TopicPermissionHandler {
	return &TopicPermissionHandler{
		topicPermSvc: topicPermSvc,
	}
}

// AssignTopicPermission 分配 Topic 权限
func (h *TopicPermissionHandler) AssignTopicPermission(c *gin.Context) {
	var req auth.AssignTopicPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	operatorID := middleware.GetUserID(c)

	if err := h.topicPermSvc.AssignTopicPermission(c.Request.Context(), &req, operatorID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "permission assigned successfully"})
}

// BatchAssignTopicPermission 批量分配 Topic 权限
func (h *TopicPermissionHandler) BatchAssignTopicPermission(c *gin.Context) {
	var req auth.BatchAssignTopicPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	operatorID := middleware.GetUserID(c)

	if err := h.topicPermSvc.BatchAssignTopicPermission(c.Request.Context(), &req, operatorID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "permissions assigned successfully",
		"count":   len(req.TopicNames),
	})
}

// RevokeTopicPermission 撤销 Topic 权限
func (h *TopicPermissionHandler) RevokeTopicPermission(c *gin.Context) {
	var req auth.RevokeTopicPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if err := h.topicPermSvc.RevokeTopicPermission(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "permission revoked successfully"})
}

// GetUserTopicPermissions 获取用户的 Topic 权限列表
func (h *TopicPermissionHandler) GetUserTopicPermissions(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("userId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	permissions, err := h.topicPermSvc.GetUserTopicPermissions(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": permissions})
}

// GetUserClusterTopicPermissions 获取用户在指定集群的 Topic 权限列表
func (h *TopicPermissionHandler) GetUserClusterTopicPermissions(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("userId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	clusterID, err := strconv.ParseInt(c.Param("clusterId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cluster id"})
		return
	}

	topics, err := h.topicPermSvc.GetUserClusterTopicPermissions(c.Request.Context(), userID, clusterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": topics})
}
