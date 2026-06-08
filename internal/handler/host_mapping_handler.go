package handler

import (
	"strconv"

	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/service/hostmapping"

	"github.com/gin-gonic/gin"
)

// HostMappingHandler 主机映射处理器
type HostMappingHandler struct {
	svc *hostmapping.Service
}

// NewHostMappingHandler 创建主机映射处理器实例
func NewHostMappingHandler(svc *hostmapping.Service) *HostMappingHandler {
	return &HostMappingHandler{svc: svc}
}

// List 获取主机映射列表
func (h *HostMappingHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	keyword := c.Query("keyword")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	mappings, total, err := h.svc.ListWithPagination(c.Request.Context(), page, pageSize, keyword)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to list host mappings"})
		return
	}

	c.JSON(200, gin.H{
		"data":     mappings,
		"total":    total,
		"page":     page,
		"page_size": pageSize,
	})
}

// GetByID 根据 ID 获取主机映射
func (h *HostMappingHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}

	mapping, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(404, gin.H{"error": "host mapping not found"})
		return
	}

	c.JSON(200, mapping)
}

// Create 创建主机映射
func (h *HostMappingHandler) Create(c *gin.Context) {
	var req struct {
		Hostname    string `json:"hostname" binding:"required"`
		IPAddress   string `json:"ip_address" binding:"required"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "hostname and ip_address are required"})
		return
	}

	mapping := &models.HostMapping{
		Hostname:    req.Hostname,
		IPAddress:   req.IPAddress,
		Description: req.Description,
	}

	if err := h.svc.Create(c.Request.Context(), mapping); err != nil {
		c.JSON(400, gin.H{"error": "failed to create host mapping, hostname may already exist"})
		return
	}

	c.JSON(201, mapping)
}

// Update 更新主机映射
func (h *HostMappingHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}

	existing, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(404, gin.H{"error": "host mapping not found"})
		return
	}

	var req struct {
		Hostname    *string `json:"hostname"`
		IPAddress   *string `json:"ip_address"`
		Description *string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request body"})
		return
	}

	if req.Hostname != nil {
		existing.Hostname = *req.Hostname
	}
	if req.IPAddress != nil {
		existing.IPAddress = *req.IPAddress
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}

	if err := h.svc.Update(c.Request.Context(), existing); err != nil {
		c.JSON(400, gin.H{"error": "failed to update host mapping"})
		return
	}

	c.JSON(200, existing)
}

// Delete 删除主机映射
func (h *HostMappingHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(500, gin.H{"error": "failed to delete host mapping"})
		return
	}

	c.JSON(200, gin.H{"message": "deleted"})
}
