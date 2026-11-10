package handler

import (
	"errors"
	"net/http"
	"strconv"

	"kafka-management-platform/internal/middleware"
	"kafka-management-platform/internal/service/message"

	"github.com/gin-gonic/gin"
)

// MessageHandler 消息查询处理器
type MessageHandler struct {
	messageSvc *message.Service
}

// NewMessageHandler 创建消息查询处理器实例
func NewMessageHandler(messageSvc *message.Service) *MessageHandler {
	return &MessageHandler{
		messageSvc: messageSvc,
	}
}

// QueryMessages 查询 Topic 消息（按 offset / 时间戳 / key 等，只读）
func (h *MessageHandler) QueryMessages(c *gin.Context) {
	topicName := c.Param("name")

	clusterID, err := strconv.ParseInt(c.Query("cluster_id"), 10, 64)
	if err != nil || clusterID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cluster_id"})
		return
	}

	var req message.QueryMessagesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request parameters"})
		return
	}

	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)

	result, err := h.messageSvc.QueryMessages(c.Request.Context(), userID, role, clusterID, topicName, &req)
	if err != nil {
		// 权限拒绝返回 403，不泄露 topic 是否存在
		if errors.Is(err, message.ErrPermissionDenied) {
			c.JSON(http.StatusForbidden, gin.H{"error": "permission denied"})
			return
		}
		if errors.Is(err, message.ErrInvalidRequest) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
