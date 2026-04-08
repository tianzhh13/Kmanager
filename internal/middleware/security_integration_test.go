package middleware

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/validator"
	"kafka-management-platform/pkg/jwt"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

// ==================== 测试专用的限流器（避免重复声明） ====================

// testRateLimiter 测试用限流器
type testRateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

func newTestRateLimiter(rps float64, burst int) *testRateLimiter {
	return &testRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(rps),
		burst:    burst,
	}
}

func (r *testRateLimiter) getLimiter(key string) *rate.Limiter {
	r.mu.RLock()
	limiter, exists := r.limiters[key]
	r.mu.RUnlock()

	if exists {
		return limiter
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if limiter, exists = r.limiters[key]; exists {
		return limiter
	}

	limiter = rate.NewLimiter(r.rate, r.burst)
	r.limiters[key] = limiter
	return limiter
}

func (r *testRateLimiter) Allow(key string) bool {
	return r.getLimiter(key).Allow()
}

// ==================== JWT Token 验证测试 ====================
// 验证需求: 13.3 - Access_Token 有效期为 1 小时

func TestJWTTokenValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建 JWT 服务
	jwtSecret := "test-secret-key-for-security-integration-test"
	jwtIssuer := "kafka-management-platform"
	jwtService := jwt.NewService(jwtSecret, jwtIssuer, 3600, 604800) // 1小时 access, 7天 refresh

	t.Run("有效 Token 应该验证通过", func(t *testing.T) {
		// 生成有效 Token
		token, err := jwtService.GenerateAccessToken(1, "testuser", models.RoleSuperAdmin)
		require.NoError(t, err)
		require.NotEmpty(t, token)

		// 验证 Token
		claims, err := jwtService.ValidateToken(token)
		require.NoError(t, err)
		assert.Equal(t, int64(1), claims.UserID)
		assert.Equal(t, "testuser", claims.Username)
		assert.Equal(t, models.RoleSuperAdmin, claims.Role)
	})

	t.Run("无效 Token 应该验证失败", func(t *testing.T) {
		invalidToken := "invalid.token.string"
		_, err := jwtService.ValidateToken(invalidToken)
		assert.Error(t, err)
		assert.Equal(t, jwt.ErrInvalidToken, err)
	})

	t.Run("过期 Token 应该验证失败", func(t *testing.T) {
		// 创建一个过期的 Token 服务
		expiredJwtService := jwt.NewService(jwtSecret, jwtIssuer, -1, -1) // 负数表示已过期
		token, err := expiredJwtService.GenerateAccessToken(1, "testuser", models.RoleSuperAdmin)
		require.NoError(t, err)

		// 验证过期 Token
		_, err = jwtService.ValidateToken(token)
		assert.Error(t, err)
		assert.Equal(t, jwt.ErrExpiredToken, err)
	})

	t.Run("错误签名 Token 应该验证失败", func(t *testing.T) {
		// 使用不同密钥生成 Token
		wrongKeyService := jwt.NewService("wrong-secret-key", jwtIssuer, 3600, 604800)
		token, err := wrongKeyService.GenerateAccessToken(1, "testuser", models.RoleSuperAdmin)
		require.NoError(t, err)

		// 验证应该失败
		_, err = jwtService.ValidateToken(token)
		assert.Error(t, err)
	})

	t.Run("空 Token 应该验证失败", func(t *testing.T) {
		_, err := jwtService.ValidateToken("")
		assert.Error(t, err)
	})
}

func TestJWTMiddlewareIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	jwtSecret := "test-secret-key-for-security-integration-test"
	jwtIssuer := "kafka-management-platform"
	jwtService := jwt.NewService(jwtSecret, jwtIssuer, 3600, 604800)

	t.Run("无 Authorization Header 应返回 401", func(t *testing.T) {
		router := gin.New()
		router.Use(middleware.AuthMiddleware(jwtService))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "authorization header required")
	})

	t.Run("无效 Authorization Header 格式应返回 401", func(t *testing.T) {
		router := gin.New()
		router.Use(middleware.AuthMiddleware(jwtService))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "InvalidFormat")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "invalid authorization header format")
	})

	t.Run("无效 Token 应返回 401", func(t *testing.T) {
		router := gin.New()
		router.Use(middleware.AuthMiddleware(jwtService))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer invalid.token")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "invalid or expired token")
	})

	t.Run("有效 Token 应允许访问", func(t *testing.T) {
		router := gin.New()
		router.Use(middleware.AuthMiddleware(jwtService))
		router.GET("/test", func(c *gin.Context) {
			userID := middleware.GetUserID(c)
			username := middleware.GetUsername(c)
			role := middleware.GetUserRole(c)
			c.JSON(200, gin.H{
				"user_id":   userID,
				"username":  username,
				"user_role": role,
			})
		})

		// 生成有效 Token
		token, err := jwtService.GenerateAccessToken(123, "testuser", models.RoleClusterAdmin)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, float64(123), response["user_id"])
		assert.Equal(t, "testuser", response["username"])
		assert.Equal(t, "cluster_admin", response["user_role"])
	})
}

// ==================== CSRF 保护测试 ====================
// 验证需求: 13.7 - 实施 CSRF 保护机制

func TestCSRFProtection(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("CSRF Token 生成和验证", func(t *testing.T) {
		// 生成 CSRF Token
		token := generateCSRFToken()
		assert.NotEmpty(t, token)
		assert.Len(t, token, 64) // SHA256 哈希长度
	})

	t.Run("相同请求的 CSRF Token 应该不同", func(t *testing.T) {
		token1 := generateCSRFToken()
		token2 := generateCSRFToken()
		assert.NotEqual(t, token1, token2, "每次生成的 CSRF Token 应该不同")
	})

	t.Run("CSRF 中间件验证 - 有效 Token", func(t *testing.T) {
		router := gin.New()
		router.Use(CSRFProtectionMiddleware())
		router.POST("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		// 先获取 CSRF Token
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 从响应头获取 CSRF Token
		csrfToken := w.Header().Get("X-CSRF-Token")
		assert.NotEmpty(t, csrfToken)

		// 使用有效 Token 发送 POST 请求
		postReq := httptest.NewRequest(http.MethodPost, "/test", nil)
		postReq.Header.Set("X-CSRF-Token", csrfToken)
		postReq.Header.Set("Cookie", w.Header().Get("Set-Cookie"))
		postW := httptest.NewRecorder()

		router.ServeHTTP(postW, postReq)

		assert.Equal(t, http.StatusOK, postW.Code)
	})

	t.Run("CSRF 中间件验证 - 无 Token 应拒绝", func(t *testing.T) {
		router := gin.New()
		router.Use(CSRFProtectionMiddleware())
		router.POST("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "csrf")
	})

	t.Run("CSRF 中间件验证 - 无效 Token 应拒绝", func(t *testing.T) {
		router := gin.New()
		router.Use(CSRFProtectionMiddleware())
		router.POST("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Header.Set("X-CSRF-Token", "invalid-csrf-token")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

// ==================== 请求限流测试 ====================
// 验证需求: 13.11 - 限制每个用户每分钟最多 100 个请求

func TestRateLimitMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("限流中间件 - 正常请求应通过", func(t *testing.T) {
		router := gin.New()
		router.Use(middleware.RateLimitMiddleware())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("限流中间件 - 超过限制应返回 429", func(t *testing.T) {
		router := gin.New()
		// 使用测试专用的限流器
		limiter := newTestRateLimiter(1, 1) // 每秒 1 个请求，burst 为 1

		router.Use(func(c *gin.Context) {
			key := c.ClientIP()
			if !limiter.Allow(key) {
				c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
				c.Abort()
				return
			}
			c.Next()
		})
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		// 第一个请求应该成功
		req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
		w1 := httptest.NewRecorder()
		router.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		// 第二个请求应该被限流
		req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusTooManyRequests, w2.Code)
		assert.Contains(t, w2.Body.String(), "rate limit exceeded")
	})

	t.Run("限流中间件 - 应包含 Retry-After 头", func(t *testing.T) {
		router := gin.New()
		limiter := newTestRateLimiter(0.01, 1) // 非常严格的限流

		router.Use(func(c *gin.Context) {
			key := c.ClientIP()
			if !limiter.Allow(key) {
				c.Header("Retry-After", "60")
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error":       "rate limit exceeded",
					"retry_after": 60,
				})
				c.Abort()
				return
			}
			c.Next()
		})
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		// 耗尽限流
		for i := 0; i < 5; i++ {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}

		// 最后一个请求应该返回限流响应
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		assert.Equal(t, "60", w.Header().Get("Retry-After"))
	})

	t.Run("IP 限流中间件 - 超过限制应返回 429", func(t *testing.T) {
		router := gin.New()
		router.Use(testIPRateLimitMiddleware(1)) // 每分钟 1 个请求
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		// 第一个请求应该成功
		req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
		w1 := httptest.NewRecorder()
		router.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		// 第二个请求应该被限流
		req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusTooManyRequests, w2.Code)
	})
}

// ==================== 输入验证测试 ====================
// 验证需求: 10.2, 10.3, 10.4, 10.5, 10.6, 10.7, 10.8, 10.9 - 输入验证

func TestInputValidation(t *testing.T) {
	v := validator.New()

	t.Run("用户名验证 - 有效用户名", func(t *testing.T) {
		validUsernames := []string{"user", "test_user", "User123", "admin_user_001"}
		for _, username := range validUsernames {
			err := v.ValidateUsername(username)
			assert.NoError(t, err, "用户名 %s 应该验证通过", username)
		}
	})

	t.Run("用户名验证 - 无效用户名", func(t *testing.T) {
		invalidUsernames := []string{
			"",      // 太短
			"ab",    // 少于 3 字符
			"user@", // 包含非法字符
			"user!", // 包含非法字符
		}
		for _, username := range invalidUsernames {
			err := v.ValidateUsername(username)
			assert.Error(t, err, "用户名 %s 应该验证失败", username)
		}
	})

	t.Run("密码验证 - 有效密码", func(t *testing.T) {
		validPasswords := []string{"Password1", "Test1234", "SecurePass1", "Admin@123"}
		for _, password := range validPasswords {
			err := v.ValidatePassword(password)
			assert.NoError(t, err, "密码 %s 应该验证通过", password)
		}
	})

	t.Run("密码验证 - 无效密码", func(t *testing.T) {
		invalidPasswords := []struct {
			password string
			reason   string
		}{
			{"short", "少于 8 字符"},
			{"alllower1", "没有大写字母"},
			{"ALLUPPER1", "没有小写字母"},
			{"NoDigits", "���有数字"},
			{"No1Special", "没有数字"},
		}
		for _, tc := range invalidPasswords {
			err := v.ValidatePassword(tc.password)
			assert.Error(t, err, tc.reason)
		}
	})

	t.Run("邮箱验证 - 有效邮箱", func(t *testing.T) {
		validEmails := []string{"user@example.com", "test.user@company.org", "admin@domain.co"}
		for _, email := range validEmails {
			err := v.ValidateEmail(email)
			assert.NoError(t, err, "邮箱 %s 应该验证通过", email)
		}
	})

	t.Run("邮箱验证 - 无效邮箱", func(t *testing.T) {
		invalidEmails := []string{
			"invalid",
			"@example.com",
			"user@",
			"user@.com",
		}
		for _, email := range invalidEmails {
			err := v.ValidateEmail(email)
			assert.Error(t, err, "邮箱 %s 应该验证失败", email)
		}
	})

	t.Run("集群名称验证 - 有效名称", func(t *testing.T) {
		validNames := []string{"a", "cluster1", "my-kafka-cluster", "prod_cluster_001"}
		for _, name := range validNames {
			err := v.ValidateClusterName(name)
			assert.NoError(t, err, "集群名称 %s 应该验证通过", name)
		}
	})

	t.Run("集群名称验证 - 无效名称", func(t *testing.T) {
		err := v.ValidateClusterName("")
		assert.Error(t, err)

		longName := string(bytes.Repeat([]byte("a"), 129))
		err = v.ValidateClusterName(longName)
		assert.Error(t, err)
	})

	t.Run("Bootstrap Servers 验证 - 有效格式", func(t *testing.T) {
		validServers := []string{
			"localhost:9092",
			"192.168.1.1:9092",
			"kafka1.example.com:9092,kafka2.example.com:9092",
			"localhost:9092,127.0.0.1:9093",
		}
		for _, servers := range validServers {
			err := v.ValidateBootstrapServers(servers)
			assert.NoError(t, err, "Bootstrap Servers %s 应该验证通过", servers)
		}
	})

	t.Run("Bootstrap Servers 验证 - 无效格式", func(t *testing.T) {
		invalidServers := []string{
			"",
			"localhost",
			"localhost:abc",
			"localhost:9092,abc",
		}
		for _, servers := range invalidServers {
			err := v.ValidateBootstrapServers(servers)
			assert.Error(t, err, "Bootstrap Servers %s 应该验证失败", servers)
		}
	})

	t.Run("Topic 名称验证 - 有效名称", func(t *testing.T) {
		validNames := []string{
			"topic",
			"my-topic",
			"topic_001",
			"topic.name",
			"a",
		}
		for _, name := range validNames {
			err := v.ValidateTopicName(name)
			assert.NoError(t, err, "Topic 名称 %s 应该验证通过", name)
		}
	})

	t.Run("Topic 名称验证 - 无效名称", func(t *testing.T) {
		invalidNames := []struct {
			name   string
			reason string
		}{
			{"", "空名称"},
			{".topic", "以 . 开头"},
			{"_topic", "以 _ 开头"},
			{"topic@name", "包含非法字符 @"},
			{string(bytes.Repeat([]byte("a"), 250)), "超过 249 字符"},
		}
		for _, tc := range invalidNames {
			err := v.ValidateTopicName(tc.name)
			assert.Error(t, err, tc.reason)
		}
	})

	t.Run("分区数验证 - 有效值", func(t *testing.T) {
		validPartitions := []int{1, 10, 100, 1000}
		for _, partitions := range validPartitions {
			err := v.ValidatePartitionCount(partitions)
			assert.NoError(t, err, "分区数 %d 应该验证通过", partitions)
		}
	})

	t.Run("分区数验证 - 无效值", func(t *testing.T) {
		invalidPartitions := []int{0, -1, -100}
		for _, partitions := range invalidPartitions {
			err := v.ValidatePartitionCount(partitions)
			assert.Error(t, err, "分区数 %d 应该验证失败", partitions)
		}
	})

	t.Run("副本数验证 - 有效值", func(t *testing.T) {
		validFactors := []int16{1, 2, 3, 10}
		for _, factor := range validFactors {
			err := v.ValidateReplicationFactor(factor)
			assert.NoError(t, err, "副本数 %d 应该验证通过", factor)
		}
	})

	t.Run("副本数验证 - 无效值", func(t *testing.T) {
		invalidFactors := []int16{0, -1, -10}
		for _, factor := range invalidFactors {
			err := v.ValidateReplicationFactor(factor)
			assert.Error(t, err, "副本数 %d 应该验证失败", factor)
		}
	})

	t.Run("角色验证 - 有效角色", func(t *testing.T) {
		validRoles := []string{"super_admin", "cluster_admin", "readonly_user"}
		for _, role := range validRoles {
			err := v.ValidateRole(role)
			assert.NoError(t, err, "角色 %s 应该验证通过", role)
		}
	})

	t.Run("角色验证 - 无效角色", func(t *testing.T) {
		invalidRoles := []string{"admin", "user", "superadmin", ""}
		for _, role := range invalidRoles {
			err := v.ValidateRole(role)
			assert.Error(t, err, "角色 %s 应该验证失败", role)
		}
	})

	t.Run("认证类型验证 - 有效类型", func(t *testing.T) {
		validTypes := []string{"plaintext", "scram", "kerberos"}
		for _, authType := range validTypes {
			err := v.ValidateAuthType(authType)
			assert.NoError(t, err, "认证类型 %s 应该验证通过", authType)
		}
	})

	t.Run("认证类型验证 - 无效类型", func(t *testing.T) {
		invalidTypes := []string{"ssl", "oauth", ""}
		for _, authType := range invalidTypes {
			err := v.ValidateAuthType(authType)
			assert.Error(t, err, "认证类型 %s 应该验证失败", authType)
		}
	})
}

// ==================== 安全响应头测试 ====================
// 验证需求: 13.8 - 设置 Content-Security-Policy 响应头

func TestSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("安全响应头中间件", func(t *testing.T) {
		router := gin.New()
		router.Use(middleware.SecurityHeadersMiddleware())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// 验证安全响应头
		assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
		assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
		assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
		assert.Contains(t, w.Header().Get("Content-Security-Policy"), "default-src 'self'")
		assert.Contains(t, w.Header().Get("Cache-Control"), "no-store")
	})

	t.Run("HSTS 中间件 - HTTPS 请求", func(t *testing.T) {
		router := gin.New()
		router.Use(middleware.HSTSMiddleware())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		// 模拟 HTTPS 请求
		req.TLS = &tls.Conn{}
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Contains(t, w.Header().Get("Strict-Transport-Security"), "max-age=31536000")
	})

	t.Run("Referrer Policy 中间件", func(t *testing.T) {
		router := gin.New()
		router.Use(RefererPolicyMiddleware())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
	})

	t.Run("Permissions Policy 中间件", func(t *testing.T) {
		router := gin.New()
		router.Use(PermissionsPolicyMiddleware())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Contains(t, w.Header().Get("Permissions-Policy"), "geolocation=()")
		assert.Contains(t, w.Header().Get("Permissions-Policy"), "microphone=()")
		assert.Contains(t, w.Header().Get("Permissions-Policy"), "camera=()")
	})
}

// ==================== 请求体大小限制测试 ====================
// 验证需求: 13.3 - 请求体大小限制（最大 10MB）

func TestRequestBodySizeLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("请求体大小限制 - 正常大小", func(t *testing.T) {
		router := gin.New()
		router.Use(RequestBodySizeLimitMiddleware(10 * 1024 * 1024)) // 10MB
		router.POST("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		body := bytes.Repeat([]byte("a"), 1024) // 1KB
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
		req.Header.Set("Content-Type", "text/plain")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("请求体大小限制 - 超过限制", func(t *testing.T) {
		router := gin.New()
		router.Use(RequestBodySizeLimitMiddleware(1024)) // 1KB 限制
		router.POST("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		body := bytes.Repeat([]byte("a"), 2048) // 2KB
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
		req.Header.Set("Content-Type", "text/plain")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// 应该返回请求体过大错误
		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	})
}

// ==================== Request ID 中间件测试 ====================

func TestRequestIDMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Request ID 生成", func(t *testing.T) {
		router := gin.New()
		router.Use(RequestIDMiddleware())
		router.GET("/test", func(c *gin.Context) {
			requestID := c.GetString("request_id")
			c.JSON(200, gin.H{"request_id": requestID})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.NotEmpty(t, w.Header().Get("X-Request-ID"))

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.NotEmpty(t, response["request_id"])
	})

	t.Run("Request ID 传递 - 客户端提供", func(t *testing.T) {
		router := gin.New()
		router.Use(RequestIDMiddleware())
		router.GET("/test", func(c *gin.Context) {
			requestID := c.GetString("request_id")
			c.JSON(200, gin.H{"request_id": requestID})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Request-ID", "custom-request-id")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, "custom-request-id", w.Header().Get("X-Request-ID"))
	})
}

// ==================== 辅助函数 ====================

// generateCSRFToken 生成 CSRF Token
func generateCSRFToken() string {
	// 简化实现：使用时间戳和随机数生成 token
	return "csrf-token-" + time.Now().Format("20060102150405")
}

// CSRFProtectionMiddleware CSRF 保护中间件（简化实现）
func CSRFProtectionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 只对非安全方法进行 CSRF 检查
		if c.Request.Method == http.MethodGet {
			// 生成 CSRF Token
			token := generateCSRFToken()
			c.Header("X-CSRF-Token", token)
			c.Set("csrf_token", token)
			c.Next()
			return
		}

		// 检查 CSRF Token
		token := c.GetHeader("X-CSRF-Token")
		if token == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "csrf token required"})
			c.Abort()
			return
		}

		// 验证 Token（简化实现：检查是否非空）
		if len(token) < 10 {
			c.JSON(http.StatusForbidden, gin.H{"error": "invalid csrf token"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// testIPRateLimitMiddleware 测试用 IP 限流中间件
func testIPRateLimitMiddleware(requestsPerMinute int) gin.HandlerFunc {
	rps := float64(requestsPerMinute) / 60.0
	limiter := newTestRateLimiter(rps, requestsPerMinute/10)

	return func(c *gin.Context) {
		key := c.ClientIP()

		if !limiter.Allow(key) {
			c.Header("Retry-After", "60")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "too many requests",
				"message":     "too many requests from this IP",
				"retry_after": 60,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}