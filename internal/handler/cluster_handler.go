package handler

import (
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"kafka-management-platform/internal/middleware"
	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/service/auth"
	"kafka-management-platform/internal/service/cluster"
	"kafka-management-platform/internal/service/monitor"

	"github.com/gin-gonic/gin"
)

// ClusterHandler 集群处理器
type ClusterHandler struct {
	clusterSvc    *cluster.Service
	permissionSvc *auth.PermissionService
	monitorSvc    *monitor.Service
}

// NewClusterHandler 创建集群处理器实例
func NewClusterHandler(clusterSvc *cluster.Service, permissionSvc *auth.PermissionService, monitorSvc *monitor.Service) *ClusterHandler {
	return &ClusterHandler{
		clusterSvc:    clusterSvc,
		permissionSvc: permissionSvc,
		monitorSvc:    monitorSvc,
	}
}

// UploadKeytab 上传 Keytab 文件
// 返回临时文件 ID，用于创建/更新集群时引用
func (h *ClusterHandler) UploadKeytab(c *gin.Context) {
	file, err := c.FormFile("keytab")
	if err != nil {
		c.JSON(400, gin.H{"error": "no keytab file provided"})
		return
	}

	// 验证文件大小 (最大 1MB)
	if file.Size > 1048576 {
		c.JSON(400, gin.H{"error": "keytab file size exceeds 1MB limit"})
		return
	}

	// 验证文件扩展名 (必须是 .keytab)
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".keytab" {
		c.JSON(400, gin.H{"error": "keytab file must have .keytab extension"})
		return
	}

	// 读取文件内容
	f, err := file.Open()
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to open keytab file"})
		return
	}
	defer f.Close()

	// 读取文件数据
	data := make([]byte, file.Size)
	if _, err := f.Read(data); err != nil {
		c.JSON(500, gin.H{"error": "failed to read keytab file"})
		return
	}

	// 保存到临时目录
	tempID, err := h.clusterSvc.SaveTempKeytab(c.Request.Context(), data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "operation failed"})
		return
	}

	c.JSON(200, gin.H{
		"temp_id": tempID,
		"message": "keytab uploaded successfully",
	})
}

// DeleteTempKeytab 删除临时 Keytab 文件
func (h *ClusterHandler) DeleteTempKeytab(c *gin.Context) {
	tempID := c.Query("temp_id")
	if tempID == "" {
		c.JSON(400, gin.H{"error": "temp_id is required"})
		return
	}
	if err := h.clusterSvc.DeleteTempKeytab(c.Request.Context(), tempID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "operation failed"})
		return
	}
	c.JSON(200, gin.H{"message": "temp keytab deleted"})
}

// CreateCluster 创建集群
func (h *ClusterHandler) CreateCluster(c *gin.Context) {
	var req cluster.CreateClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request parameters"})
		return
	}

	// 设置创建者
	req.CreatedBy = middleware.GetUserID(c)

	result, err := h.clusterSvc.CreateCluster(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "operation failed"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request parameters"})
		return
	}

	if err := h.clusterSvc.UpdateCluster(c.Request.Context(), clusterID, &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "operation failed"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "operation failed"})
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

	// 附加 stats 字段（broker_count / topic_count / health_status）
	healthStatus := h.monitorSvc.GetClustersHealthStatus(c.Request.Context(), []int64{clusterID})
	brokerCounts := h.monitorSvc.GetBrokerCountByCluster(c.Request.Context(), []int64{clusterID})
	topicCounts, _ := h.clusterSvc.GetTopicCountByCluster(c.Request.Context())

	type clusterWithStats struct {
		*models.Cluster
		BrokerCount  *int   `json:"broker_count"`
		TopicCount   *int64 `json:"topic_count"`
		HealthStatus string `json:"health_status"`
	}

	resp := clusterWithStats{Cluster: result}
	if bc, ok := brokerCounts[clusterID]; ok {
		resp.BrokerCount = &bc
	}
	if tc, ok := topicCounts[clusterID]; ok {
		resp.TopicCount = &tc
	}
	if hs, ok := healthStatus[clusterID]; ok {
		resp.HealthStatus = hs
	} else {
		resp.HealthStatus = "unknown"
	}

	c.JSON(200, resp)
}

// ListClusters 获取集群列表
func (h *ClusterHandler) ListClusters(c *gin.Context) {
	// 获取分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	offset := (page - 1) * pageSize

	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)

	clusters, total, err := h.clusterSvc.ListClusters(c.Request.Context(), userID, models.UserRole(role), offset, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "operation failed"})
		return
	}

	// 扩展字段：with_stats=true 时附加 broker_count / topic_count / health_status
	withStats := c.Query("with_stats") == "true"
	if withStats && len(clusters) > 0 {
		clusterIDs := make([]int64, len(clusters))
		for i, cl := range clusters {
			clusterIDs[i] = cl.ClusterID
		}

		// 并发获取 VM 数据
		healthStatus := h.monitorSvc.GetClustersHealthStatus(c.Request.Context(), clusterIDs)
		brokerCounts := h.monitorSvc.GetBrokerCountByCluster(c.Request.Context(), clusterIDs)

		// 获取 per-cluster topic count
		topicCounts, _ := h.clusterSvc.GetTopicCountByCluster(c.Request.Context())

		// 附加到响应
		type clusterWithStats struct {
			*models.Cluster
			BrokerCount  *int   `json:"broker_count"`
			TopicCount   *int64 `json:"topic_count"`
			HealthStatus string `json:"health_status"`
		}

		result := make([]clusterWithStats, len(clusters))
		for i, cl := range clusters {
			item := clusterWithStats{Cluster: cl}
			if bc, ok := brokerCounts[cl.ClusterID]; ok {
				item.BrokerCount = &bc
			}
			if tc, ok := topicCounts[cl.ClusterID]; ok {
				item.TopicCount = &tc
			}
			if hs, ok := healthStatus[cl.ClusterID]; ok {
				item.HealthStatus = hs
			} else {
				item.HealthStatus = "unknown"
			}
			result[i] = item
		}

		c.JSON(200, gin.H{
			"data":      result,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
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

	// 集群管理员只能操作自己管理的集群
	userRole := middleware.GetUserRole(c)
	if userRole == string(models.RoleClusterAdmin) {
		userID := middleware.GetUserID(c)
		hasAccess, err := h.permissionSvc.CheckClusterPermission(c.Request.Context(), userID, clusterID, userRole)
		if err != nil || !hasAccess {
			c.JSON(http.StatusForbidden, gin.H{"error": "no permission for this cluster"})
			return
		}
	}

	var req struct {
		UserID int64 `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request parameters"})
		return
	}

	if err := h.clusterSvc.GrantClusterAccess(c.Request.Context(), clusterID, req.UserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "operation failed"})
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

	// 集群管理员只能操作自己管理的集群
	userRole := middleware.GetUserRole(c)
	if userRole == string(models.RoleClusterAdmin) {
		userID := middleware.GetUserID(c)
		hasAccess, err := h.permissionSvc.CheckClusterPermission(c.Request.Context(), userID, clusterID, userRole)
		if err != nil || !hasAccess {
			c.JSON(http.StatusForbidden, gin.H{"error": "no permission for this cluster"})
			return
		}
	}

	var req struct {
		UserID int64 `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request parameters"})
		return
	}

	if err := h.clusterSvc.RevokeClusterAccess(c.Request.Context(), clusterID, req.UserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "operation failed"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "operation failed"})
		return
	}

	c.JSON(200, gin.H{"data": users})
}

// ListUserClusters 获取用户已授权的集群列表
func (h *ClusterHandler) ListUserClusters(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("userId"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid user id"})
		return
	}

	clusters, err := h.clusterSvc.ListUserClusters(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "operation failed"})
		return
	}

	c.JSON(200, gin.H{"data": clusters})
}

// TestConnection 测试集群连接
func (h *ClusterHandler) TestConnection(c *gin.Context) {
	clusterID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid cluster id"})
		return
	}

	if err := h.clusterSvc.TestConnection(c.Request.Context(), clusterID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "connection test successful"})
}

// TestConnectionForCreate 在创建集群前测试连接配置
func (h *ClusterHandler) TestConnectionForCreate(c *gin.Context) {
	var req cluster.CreateClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request parameters"})
		return
	}

	if err := h.clusterSvc.TestConnectionForCreate(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "connection test successful"})
}
