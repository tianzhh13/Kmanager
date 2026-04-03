package user

import (
	"context"
	"errors"
	"fmt"

	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/repository"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrInvalidPassword    = errors.New("invalid password")
	ErrCannotDeleteSelf   = errors.New("cannot delete yourself")
)

// Service 用户服务
type Service struct {
	userRepo repository.UserRepository
}

// NewService 创建用户服务
func NewService(userRepo repository.UserRepository) *Service {
	return &Service{
		userRepo: userRepo,
	}
}

// CreateUser 创建用户
func (s *Service) CreateUser(ctx context.Context, username, email, password, role string) (*models.User, error) {
	// 验证用户名唯一性
	existing, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to check username: %w", err)
	}
	if existing != nil {
		return nil, ErrUserAlreadyExists
	}

	// 加密密码
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &models.User{
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
		Role:         models.UserRole(role),
		Status:       models.UserStatusActive,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// 不返回密码
	user.PasswordHash = ""
	return user, nil
}

// UpdateUser 更新用户信息
func (s *Service) UpdateUser(ctx context.Context, userID int64, email, role string) (*models.User, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// 更新邮箱
	if email != "" {
		user.Email = email
	}

	// 更新角色
	if role != "" {
		user.Role = models.UserRole(role)
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	user.PasswordHash = ""
	return user, nil
}

// UpdatePassword 更新密码
func (s *Service) UpdatePassword(ctx context.Context, userID int64, oldPassword, newPassword string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("failed to get user: %w", err)
	}

	// 验证旧密码
	if !password.Verify(user.PasswordHash, oldPassword) {
		return ErrInvalidPassword
	}

	// 加密新密码
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user.PasswordHash = string(hash)
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// DeleteUser 删除用户
func (s *Service) DeleteUser(ctx context.Context, userID, currentUserID int64) error {
	// 不能删除自己
	if userID == currentUserID {
		return ErrCannotDeleteSelf
	}

	_, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("failed to get user: %w", err)
	}

	return s.userRepo.Delete(ctx, userID)
}

// DisableUser 禁用用户
func (s *Service) DisableUser(ctx context.Context, userID int64) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("failed to get user: %w", err)
	}

	user.Status = models.UserStatusInactive
	return s.userRepo.Update(ctx, user)
}

// EnableUser 启用用户
func (s *Service) EnableUser(ctx context.Context, userID int64) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("failed to get user: %w", err)
	}

	user.Status = models.UserStatusActive
	return s.userRepo.Update(ctx, user)
}

// ListUsers 获取用户列表
func (s *Service) ListUsers(ctx context.Context, page, pageSize int, keyword string) ([]*models.User, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	offset := (page - 1) * pageSize

	if keyword != "" {
		return s.userRepo.Search(ctx, keyword, offset, pageSize)
	}
	return s.userRepo.List(ctx, offset, pageSize)
}

// GetUser 获取用户详情
func (s *Service) GetUser(ctx context.Context, userID int64) (*models.User, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	user.PasswordHash = ""
	return user, nil
}