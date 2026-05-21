package handler

import (
	"net/http"

	"kafka-management-platform/internal/cache"
	"kafka-management-platform/internal/config"
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
	cookieCfg      *config.CookieConfig
}

// NewAuthHandler 创建认证处理器实例
func NewAuthHandler(authSvc *auth.Service, blacklistCache *cache.TokenBlacklistCache, auditSvc *audit.Service, cookieCfg *config.CookieConfig) *AuthHandler {
	return &AuthHandler{
		authSvc:        authSvc,
		blacklistCache: blacklistCache,
		auditSvc:       auditSvc,
		cookieCfg:      cookieCfg,
	}
}

// setTokenCookie 设置 Token 为 httpOnly Cookie
func (h *AuthHandler) setTokenCookie(c *gin.Context, name, value string, maxAge int64) {
	cookiePath := "/"
	if h.cookieCfg != nil && h.cookieCfg.Path != "" {
		cookiePath = h.cookieCfg.Path
	}
	cookieDomain := ""
	if h.cookieCfg != nil && h.cookieCfg.Domain != "" {
		cookieDomain = h.cookieCfg.Domain
	}
	sameSite := http.SameSiteLaxMode
	if h.cookieCfg != nil && h.cookieCfg.SameSite != "" {
		switch h.cookieCfg.SameSite {
		case "Strict":
			sameSite = http.SameSiteStrictMode
		case "None":
			sameSite = http.SameSiteNoneMode
		}
	}
	secure := false
	if h.cookieCfg != nil && h.cookieCfg.Secure {
		secure = h.cookieCfg.Secure
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     cookiePath,
		Domain:   cookieDomain,
		MaxAge:   int(maxAge),
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})
}

// clearTokenCookie 清除 Token Cookie
func (h *AuthHandler) clearTokenCookie(c *gin.Context, name string) {
	cookiePath := "/"
	if h.cookieCfg != nil && h.cookieCfg.Path != "" {
		cookiePath = h.cookieCfg.Path
	}
	cookieDomain := ""
	if h.cookieCfg != nil && h.cookieCfg.Domain != "" {
		cookieDomain = h.cookieCfg.Domain
	}
	secure := false
	if h.cookieCfg != nil && h.cookieCfg.Secure {
		secure = h.cookieCfg.Secure
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     cookiePath,
		Domain:   cookieDomain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
	})
}

// Login 用户登录
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

	// 通过 httpOnly Cookie 下发 Token，同时返回 JSON（兼容前端读取 user_info）
	h.setTokenCookie(c, "access_token", resp.AccessToken, resp.ExpiresIn)
	h.setTokenCookie(c, "refresh_token", resp.RefreshToken, 7*24*3600) // 7 天

	c.JSON(200, resp)
}

// RefreshToken 刷新访问 Token
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	// 优先从 Cookie 读取 refresh_token，兼容请求体传参
	refreshToken := ""
	if cookie, err := c.Cookie("refresh_token"); err == nil && cookie != "" {
		refreshToken = cookie
	}
	if refreshToken == "" {
		var req struct {
			RefreshToken string `json:"refresh_token" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "invalid request: " + err.Error()})
			return
		}
		refreshToken = req.RefreshToken
	}

	resp, err := h.authSvc.RefreshToken(c.Request.Context(), refreshToken)
	if err != nil {
		c.JSON(401, gin.H{"error": err.Error()})
		return
	}

	// 刷新 access_token Cookie
	h.setTokenCookie(c, "access_token", resp.AccessToken, resp.ExpiresIn)

	c.JSON(200, resp)
}

// GetCurrentUser 获取当前用户信息
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
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
func (h *AuthHandler) Logout(c *gin.Context) {
	token := middleware.GetRawToken(c)
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no token found"})
		return
	}

	if h.blacklistCache != nil {
		if err := h.blacklistCache.AddToBlacklist(c.Request.Context(), token); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke token"})
			return
		}
	}

	// 清除 Cookie
	h.clearTokenCookie(c, "access_token")
	h.clearTokenCookie(c, "refresh_token")

	c.JSON(http.StatusOK, gin.H{"message": "logged out successfully"})
}
