package handler

import (
	"net/http"

	"kafka-management-platform/internal/cache"
	"kafka-management-platform/internal/middleware"
	"kafka-management-platform/internal/service/audit"
	"kafka-management-platform/internal/service/auth"

	"github.com/gin-gonic/gin"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	authSvc        *auth.Service
	blacklistCache *cache.TokenBlacklistCache
	auditSvc       *audit.Service
}

// NewAuthHandler 创建认证处理器实例
func NewAuthHandler(authSvc *auth.Service, blacklistCache *cache.TokenBlacklistCache, auditSvc *audit.Service) *AuthHandler {
	return &AuthHandler{
		authSvc:        authSvc,
		blacklistCache: blacklistCache,
		auditSvc:       auditSvc,
	}
}

// Login 用户登录
// @Summary 用户登录
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body auth.LoginRequest true "登录请求"
// @Success 200 {object} auth.LoginResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req auth.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{
			"error": "invalid request: " + err.Error(),
		})
		return
	}

	resp, err := h.authSvc.Login(c.Request.Context(), &req)
	if err != nil {
		// 记录登录失败审计日志
		if h.auditSvc != nil {
			_ = h.auditSvc.Log(c.Request.Context(), &audit.LogRequest{
				Username:     req.Username,
				Action:       "login",
				ResourceType: "auth",
				IPAddress:    c.ClientIP(),
				UserAgent:    c.Request.UserAgent(),
				Status:       "failed",
				Details:      map[string]interface{}{"error": err.Error()},
			})
		}
		c.JSON(401, gin.H{
			"error": err.Error(),
		})
		return
	}

	// 记录登录成功审计日志
	if h.auditSvc != nil {
		_ = h.auditSvc.Log(c.Request.Context(), &audit.LogRequest{
			UserID:       resp.UserInfo.UserID,
			Username:     resp.UserInfo.Username,
			Action:       "login",
			ResourceType: "auth",
			IPAddress:    c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			Status:       "success",
		})
	}

	c.JSON(200, resp)
}

// RefreshToken 刷新访问 Token
// @Summary 刷新 Token
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body object{refresh_token=string} true "刷新 Token 请求"
// @Success 200 {object} auth.LoginResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/v1/auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{
			"error": "invalid request: " + err.Error(),
		})
		return
	}

	resp, err := h.authSvc.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		c.JSON(401, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(200, resp)
}

// GetCurrentUser 获取当前用户信息
// @Summary 获取当前用户信息
// @Tags 认证
// @Produce json
// @Security Bearer
// @Success 200 {object} auth.UserInfo
// @Failure 401 {object} map[string]string
// @Router /api/v1/auth/me [get]
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	// 从 Context 获取用户信息（由认证中间件设置）
	userID, _ := c.Get("user_id")
	username, _ := c.Get("username")
	role, _ := c.Get("role")

	c.JSON(200, gin.H{
		"user_id":  userID,
		"username": username,
		"role":     role,
	})
}

// Logout 退出登录
// @Summary 退出登录
// @Tags 认证
// @Produce json
// @Security Bearer
// @Success 200 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/v1/auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	token := middleware.GetRawToken(c)
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no token found"})
		return
	}

	// 将 Token 加入黑名单
	if h.blacklistCache != nil {
		if err := h.blacklistCache.AddToBlacklist(c.Request.Context(), token); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke token"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "logged out successfully"})
}
