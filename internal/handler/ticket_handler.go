package handler

import (
	"errors"
	"net/http"
	"strconv"

	"kafka-management-platform/internal/middleware"
	"kafka-management-platform/internal/service/ticket"

	"github.com/gin-gonic/gin"
)

// TicketHandler 工单处理器
type TicketHandler struct {
	ticketSvc *ticket.Service
}

// NewTicketHandler 创建工单处理器实例
func NewTicketHandler(ticketSvc *ticket.Service) *TicketHandler {
	return &TicketHandler{ticketSvc: ticketSvc}
}

// Submit 提交工单
func (h *TicketHandler) Submit(c *gin.Context) {
	var req ticket.SubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request parameters"})
		return
	}

	userID := middleware.GetUserID(c)
	username := middleware.GetUsername(c)
	role := middleware.GetUserRole(c)

	t, err := h.ticketSvc.Submit(c.Request.Context(), userID, username, role, &req)
	if err != nil {
		respondTicketError(c, err)
		return
	}
	c.JSON(http.StatusCreated, t)
}

// List 工单列表
func (h *TicketHandler) List(c *gin.Context) {
	var req ticket.ListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query parameters"})
		return
	}

	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)

	resp, err := h.ticketSvc.List(c.Request.Context(), userID, role, &req)
	if err != nil {
		respondTicketError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Stats 工单状态统计
func (h *TicketHandler) Stats(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)

	stats, err := h.ticketSvc.Stats(c.Request.Context(), userID, role)
	if err != nil {
		respondTicketError(c, err)
		return
	}
	c.JSON(http.StatusOK, stats)
}

// Get 工单详情
func (h *TicketHandler) Get(c *gin.Context) {
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
		return
	}

	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)

	t, err := h.ticketSvc.Get(c.Request.Context(), userID, role, ticketID)
	if err != nil {
		respondTicketError(c, err)
		return
	}
	c.JSON(http.StatusOK, t)
}

// actionRequest 审批操作请求
type actionRequest struct {
	Remark string `json:"remark"`
}

// Approve 审批通过
func (h *TicketHandler) Approve(c *gin.Context) {
	var req actionRequest
	_ = c.ShouldBindJSON(&req)

	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
		return
	}

	userID := middleware.GetUserID(c)
	username := middleware.GetUsername(c)
	role := middleware.GetUserRole(c)

	t, err := h.ticketSvc.Approve(c.Request.Context(), userID, username, role, ticketID, req.Remark)
	if err != nil {
		respondTicketError(c, err)
		return
	}
	c.JSON(http.StatusOK, t)
}

// Reject 驳回
func (h *TicketHandler) Reject(c *gin.Context) {
	var req actionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request parameters"})
		return
	}

	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
		return
	}

	userID := middleware.GetUserID(c)
	username := middleware.GetUsername(c)
	role := middleware.GetUserRole(c)

	t, err := h.ticketSvc.Reject(c.Request.Context(), userID, username, role, ticketID, req.Remark)
	if err != nil {
		respondTicketError(c, err)
		return
	}
	c.JSON(http.StatusOK, t)
}

// Cancel 撤回工单
func (h *TicketHandler) Cancel(c *gin.Context) {
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
		return
	}

	userID := middleware.GetUserID(c)
	username := middleware.GetUsername(c)

	t, err := h.ticketSvc.Cancel(c.Request.Context(), userID, username, ticketID)
	if err != nil {
		respondTicketError(c, err)
		return
	}
	c.JSON(http.StatusOK, t)
}

// Retry 重试失败的工单
func (h *TicketHandler) Retry(c *gin.Context) {
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
		return
	}

	userID := middleware.GetUserID(c)
	username := middleware.GetUsername(c)
	role := middleware.GetUserRole(c)

	t, err := h.ticketSvc.Retry(c.Request.Context(), userID, username, role, ticketID)
	if err != nil {
		respondTicketError(c, err)
		return
	}
	c.JSON(http.StatusOK, t)
}

// TakeCredential 一次性查看凭证
func (h *TicketHandler) TakeCredential(c *gin.Context) {
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
		return
	}

	userID := middleware.GetUserID(c)
	username := middleware.GetUsername(c)

	cred, err := h.ticketSvc.TakeCredential(c.Request.Context(), userID, username, ticketID)
	if err != nil {
		respondTicketError(c, err)
		return
	}
	c.JSON(http.StatusOK, cred)
}

// respondTicketError 工单错误统一响应
func respondTicketError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ticket.ErrTicketNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, ticket.ErrPermissionDenied):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, ticket.ErrInvalidStatus), errors.Is(err, ticket.ErrRemarkRequired), errors.Is(err, ticket.ErrCredentialUnavailable):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}
