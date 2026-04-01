package jwt

import (
	"errors"
	"time"

	"kafka-management-platform/internal/models"

	"github.com/golang-jwt/jwt/v5"
)

var (
	// ErrInvalidToken 无效的 Token
	ErrInvalidToken = errors.New("invalid token")
	// ErrExpiredToken Token 已过期
	ErrExpiredToken = errors.New("token has expired")
)

// Claims JWT 声明
type Claims struct {
	UserID   int64            `json:"user_id"`
	Username string           `json:"username"`
	Role     models.UserRole  `json:"role"`
	jwt.RegisteredClaims
}

// Service JWT 服务
type Service struct {
	secret             []byte
	issuer             string
	accessTokenExpire  time.Duration
	refreshTokenExpire time.Duration
}

// NewService 创建 JWT 服务实例
func NewService(secret, issuer string, accessTokenExpire, refreshTokenExpire int) *Service {
	return &Service{
		secret:             []byte(secret),
		issuer:             issuer,
		accessTokenExpire:  time.Duration(accessTokenExpire) * time.Second,
		refreshTokenExpire: time.Duration(refreshTokenExpire) * time.Second,
	}
}

// GenerateAccessToken 生成访问 Token
func (s *Service) GenerateAccessToken(userID int64, username string, role models.UserRole) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   username,
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTokenExpire)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// GenerateRefreshToken 生成刷新 Token
func (s *Service) GenerateRefreshToken(userID int64, username string, role models.UserRole) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   username,
			ExpiresAt: jwt.NewNumericDate(now.Add(s.refreshTokenExpire)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// ValidateToken 验证 Token 并返回声明
func (s *Service) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名方法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.secret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

// RefreshAccessToken 使用刷新 Token 生成新的访问 Token
func (s *Service) RefreshAccessToken(refreshToken string) (string, error) {
	// 验证刷新 Token
	claims, err := s.ValidateToken(refreshToken)
	if err != nil {
		return "", err
	}

	// 生成新的访问 Token
	return s.GenerateAccessToken(claims.UserID, claims.Username, claims.Role)
}
