package handler

import (
	"strconv"

	"kafka-management-platform/internal/middleware"
	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/service/cluster"

	"github.com/gin-gonic/gin"
)

// ClusterHandler 集群处理器
type ClusterHandler struct {
	clusterSvc *cluster.Service
}

// NewClusterHandler 创建集群处理器实例
func NewClusterHandler(clusterSvc *cluster.Service) *ClusterHandler {
	return &ClusterHandler{
		clusterSvc: clusterSvc,
	}
}

// CreateCluster 创建集群
func (h *ClusterHandler) CreateCluster(c *gin.Context) {
	var req cluster.CreateClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	// 设置创建者
	req.CreatedBy = middleware.GetUserID(c)

	result, err := h.clusterSvc.CreateCluster(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(201, result)
}

// UpdateCluster 更新集群
func (h *ClusterHandler) UpdateCluster(c *gin.Context) {
	clusterID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid cluster id"})
		return
	}

	var req cluster.UpdateClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if err := h.clusterSvc.UpdateCluster(c.Request.Context(), clusterID, &req); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "cluster updated successfully"})
}

// DeleteCluster 删除集群
func (h *ClusterHandler) DeleteCluster(c *gin.Context) {
	clusterID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid cluster id"})
		return
	}

	if err := h.clusterSvc.DeleteCluster(c.Request.Context(), clusterID); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "cluster deleted successfully"})
}

// GetCluster 获取集群详情
func (h *ClusterHandler) GetCluster(c *gin.Context) {
	clusterID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid cluster id"})
		return
	}

	result, err := h.clusterSvc.GetCluster(c.Request.Context(), clusterID)
	if err != nil {
		c.JSON(404, gin.H{"error": "cluster not found"})
		return
	}

	c.JSON(200, result)
}

// ListClusters 获取集群列表
func (h *ClusterHandler) ListClusters(c *gin.Context) {
	// 获取分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	offset := (page - 1) * pageSize

	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)

	clusters, total, err := h.clusterSvc.ListClusters(c.Request.Context(), userID, models.UserRole(role), offset, pageSize)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"data":      clusters,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GrantAccess 授予用户集群访问权限
func (h *ClusterHandler) GrantAccess(c *gin.Context) {
	clusterID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid cluster id"})
		return
	}

	var req struct {
		UserID int64 `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if err := h.clusterSvc.GrantClusterAccess(c.Request.Context(), clusterID, req.UserID); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "access granted successfully"})
}

// RevokeAccess 撤销用户集群访问权限
func (h *ClusterHandler) RevokeAccess(c *gin.Context) {
	clusterID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid cluster id"})
		return
	}

	var req struct {
		UserID int64 `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if err := h.clusterSvc.RevokeClusterAccess(c.Request.Context(), clusterID, req.UserID); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "access revoked successfully"})
}

// ListClusterUsers 获取集群的授权用户列表
func (h *ClusterHandler) ListClusterUsers(c *gin.Context) {
	clusterID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid cluster id"})
		return
	}

	users, err := h.clusterSvc.ListClusterUsers(c.Request.Context(), clusterID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"data": users})
}

// TestConnection 测试集群连接
func (h *ClusterHandler) TestConnection(c *gin.Context) {
	clusterID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid cluster id"})
		return
	}

	if err := h.clusterSvc.TestConnection(c.Request.Context(), clusterID); err != nil {
		c.JSON(500, gin.H{"error": "connection test failed", "details": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "connection test successful"})
}
