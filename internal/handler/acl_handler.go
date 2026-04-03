package handler

import (
	"net/http"
	"strconv"

	"kafka-management-platform/internal/service/acl"

	"github.com/gin-gonic/gin"
)

// ACLHandler ACL 处理器
type ACLHandler struct {
	aclSvc *acl.Service
}

// NewACLHandler 创建 ACL 处理器实例
func NewACLHandler(aclSvc *acl.Service) *ACLHandler {
	return &ACLHandler{
		aclSvc: aclSvc,
	}
}

// CreateACL 创建 ACL
func (h *ACLHandler) CreateACL(c *gin.Context) {
	var req acl.CreateACLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.aclSvc.CreateACL(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "acl created successfully"})
}

// DeleteACL 删除 ACL
func (h *ACLHandler) DeleteACL(c *gin.Context) {
	aclIDStr := c.Param("id")
	aclID, err := strconv.ParseInt(aclIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid acl_id"})
		return
	}

	if err := h.aclSvc.DeleteACL(c.Request.Context(), aclID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "acl deleted successfully"})
}

// BatchDeleteACL 批量删除 ACL
func (h *ACLHandler) BatchDeleteACL(c *gin.Context) {
	var req struct {
		ACLIDs []int64 `json:"acl_ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.aclSvc.BatchDeleteACL(c.Request.Context(), req.ACLIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "acls deleted successfully"})
}

// ListACLs 列出 ACL
func (h *ACLHandler) ListACLs(c *gin.Context) {
	var req acl.ListACLsRequest

	if clusterIDStr := c.Query("cluster_id"); clusterIDStr != "" {
		clusterID, err := strconv.ParseInt(clusterIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cluster_id"})
			return
		}
		req.ClusterID = clusterID
	}

	req.ResourceType = c.Query("resource_type")
	req.ResourceName = c.Query("resource_name")
	req.Principal = c.Query("principal")

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

	resp, err := h.aclSvc.ListACLs(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// SyncACLs 同步 ACL
func (h *ACLHandler) SyncACLs(c *gin.Context) {
	clusterIDStr := c.Param("id")
	clusterID, err := strconv.ParseInt(clusterIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cluster_id"})
		return
	}

	if err := h.aclSvc.SyncACLs(c.Request.Context(), clusterID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "acls synced successfully"})
}
