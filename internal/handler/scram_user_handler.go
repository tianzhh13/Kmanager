package handler

import (
	"net/http"
	"strconv"

	"kafka-management-platform/internal/service/scram"

	"github.com/gin-gonic/gin"
)

// ScramUserHandler SCRAM 用户处理器
type ScramUserHandler struct {
	scramSvc *scram.Service
}

// NewScramUserHandler 创建 SCRAM 用户处理器实例
func NewScramUserHandler(scramSvc *scram.Service) *ScramUserHandler {
	return &ScramUserHandler{
		scramSvc: scramSvc,
	}
}

// CreateUser 创建 SCRAM 用户
func (h *ScramUserHandler) CreateUser(c *gin.Context) {
	var req scram.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request parameters"})
		return
	}

	if err := h.scramSvc.CreateUser(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "user created successfully"})
}

// DeleteUser 删除 SCRAM 用户
func (h *ScramUserHandler) DeleteUser(c *gin.Context) {
	clusterIDStr := c.Query("cluster_id")
	if clusterIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cluster_id is required"})
		return
	}

	clusterID, err := strconv.ParseInt(clusterIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cluster_id"})
		return
	}

	username := c.Param("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username is required"})
		return
	}

	if err := h.scramSvc.DeleteUser(c.Request.Context(), clusterID, username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "user deleted successfully"})
}

// ListUsers 列出 SCRAM 用户
func (h *ScramUserHandler) ListUsers(c *gin.Context) {
	var req scram.ListUsersRequest

	if clusterIDStr := c.Query("cluster_id"); clusterIDStr != "" {
		clusterID, err := strconv.ParseInt(clusterIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cluster_id"})
			return
		}
		req.ClusterID = clusterID
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			offset = 0
		}
		req.Offset = offset
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		limit, _ := strconv.Atoi(limitStr)
		req.Limit = limit
	} else {
		req.Limit = 20
	}
	if req.Limit > 100 {
		req.Limit = 100
	}
	if req.Limit < 1 {
		req.Limit = 20
	}

	resp, err := h.scramSvc.ListUsers(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// SyncUsers 同步 SCRAM 用户
func (h *ScramUserHandler) SyncUsers(c *gin.Context) {
	clusterIDStr := c.Param("id")
	clusterID, err := strconv.ParseInt(clusterIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cluster_id"})
		return
	}

	if err := h.scramSvc.SyncUsers(c.Request.Context(), clusterID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "users synced successfully"})
}
