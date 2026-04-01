package auth

import (
	"context"
	"errors"

	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/repository"
	"kafka-management-platform/pkg/jwt"
	"kafka-management-platform/pkg/password"
)

var (
	// ErrInvalidCredentials 无效的凭证
	ErrInvalidCredentials = errors.New("invalid username or password")
	// ErrUserInactive 用户未激活
	ErrUserInactive = errors.New("user account is inactive")
)

// Service 认证服务
type Service struct {
	userRepo repository.UserRepository
	jwtSvc   *jwt.Service
}

// NewService 创建认证服务实例
func NewService(userRepo repository.UserRepository, jwtSvc *jwt.Service) *Service {
	return &Service{
		userRepo: userRepo,
		jwtSvc:   jwtSvc,
	}
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresIn    int64     `json:"expires_in"`
	UserInfo     *UserInfo `json:"user_info"`
}

// UserInfo 用户信息
type UserInfo struct {
	UserID   int64           `json:"user_id"`
	Username string          `json:"username"`
	Email    string          `json:"email"`
	Role     models.UserRole `json:"role"`
}

// Login 用户登录
func (s *Service) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	// 步骤 1：从数据库查询用户
	user, err := s.userRepo.FindByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	// 步骤 2：验证密码
	if !password.Verify(user.PasswordHash, req.Password) {
		return nil, ErrInvalidCredentials
	}

	// 步骤 3：检查用户状态
	if user.Status != models.UserStatusActive {
		return nil, ErrUserInactive
	}

	// 步骤 4：生成 JWT Token
	accessToken, err := s.jwtSvc.GenerateAccessToken(user.UserID, user.Username, user.Role)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.jwtSvc.GenerateRefreshToken(user.UserID, user.Username, user.Role)
	if err != nil {
		return nil, err
	}

	// 步骤 5：返回登录响应
	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    3600, // 1 小时
		UserInfo: &UserInfo{
			UserID:   user.UserID,
			Username: user.Username,
			Email:    user.Email,
			Role:     user.Role,
		},
	}, nil
}

// RefreshToken 刷新访问 Token
func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (*LoginResponse, error) {
	// 验证刷新 Token 并生成新的访问 Token
	accessToken, err := s.jwtSvc.RefreshAccessToken(refreshToken)
	if err != nil {
		return nil, err
	}

	// 从刷新 Token 中提取用户信息
	claims, err := s.jwtSvc.ValidateToken(refreshToken)
	if err != nil {
		return nil, err
	}

	// 获取最新的用户信息
	user, err := s.userRepo.FindByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}

	// 检查用户状态
	if user.Status != models.UserStatusActive {
		return nil, ErrUserInactive
	}

	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    3600,
		UserInfo: &UserInfo{
			UserID:   user.UserID,
			Username: user.Username,
			Email:    user.Email,
			Role:     user.Role,
		},
	}, nil
}

// ValidateToken 验证 Token 并返回用户信息
func (s *Service) ValidateToken(ctx context.Context, token string) (*UserInfo, error) {
	claims, err := s.jwtSvc.ValidateToken(token)
	if err != nil {
		return nil, err
	}

	// 获取最新的用户信息
	user, err := s.userRepo.FindByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}

	// 检查用户状态
	if user.Status != models.UserStatusActive {
		return nil, ErrUserInactive
	}

	return &UserInfo{
		UserID:   user.UserID,
		Username: user.Username,
		Email:    user.Email,
		Role:     user.Role,
	}, nil
}
