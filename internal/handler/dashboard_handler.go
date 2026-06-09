package handler

import (
	"net/http"

	"kafka-management-platform/internal/service/dashboard"

	"github.com/gin-gonic/gin"
)

// DashboardHandler Dashboard 处理器
type DashboardHandler struct {
	dashboardSvc *dashboard.Service
}

// NewDashboardHandler 创建 Dashboard 处理器实例
func NewDashboardHandler(dashboardSvc *dashboard.Service) *DashboardHandler {
	return &DashboardHandler{
		dashboardSvc: dashboardSvc,
	}
}

// GetOverview 获取 Dashboard 概览数据
// GET /api/v1/dashboard/overview
// 权限：Any authenticated
func (h *DashboardHandler) GetOverview(c *gin.Context) {
	result, err := h.dashboardSvc.GetOverview(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
