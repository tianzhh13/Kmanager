package user

import (
	"context"
	"errors"

	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/repository"
	"kafka-management-platform/pkg/password"

	"gorm.io/gorm"
)

var (
	ErrUserNotFound         = errors.New("user not found")
	ErrUserAlreadyExists    = errors.New("user already exists")
	ErrInvalidPassword      = errors.New("invalid password")
	ErrUserDisabled         = errors.New("user is disabled")
	ErrCannotDeleteSelf     = errors.New("cannot delete yourself")
	ErrCannotDisableSelf    = errors.New("cannot disable yourself")
)

// Service 用户管理服务
type Service struct {
	userRepo repository.UserRepository
}

// NewService 创建用户管理服务实例
func NewService(userRepo repository.UserRepository) *Service {
	return &Service{
		userRepo: userRepo,
	}
}

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Username string         `json:"username" binding:"required,min=3,max=64"`
	Email    string         `json:"email" binding:"required,email"`
	Password string         `json:"password" binding:"required,min=8"`
	Role     models.UserRole `json:"role" binding:"required"`
	Phone    string         `json:"phone"`
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	Email    string          `json:"email"`
	Role     models.UserRole `json:"role"`
	Phone    string          `json:"phone"`
	Status   models.UserStatus `json:"status"`
}

// UpdatePasswordRequest 更新密码请求
type UpdatePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// CreateUser 创建用户
func (s *Service) CreateUser(ctx context.Context, req *CreateUserRequest) (*models.User, error) {
	// 检查用户名是否已存在
	existing, err := s.userRepo.FindByUsername(ctx, req.Username)
	if err == nil && existing != nil {
		return nil, ErrUserAlreadyExists
	}

	// 验证密码复杂度
	if err := password.ValidatePassword(req.Password); err != nil {
		return nil, err
	}

	// 加密密码
	hashedPassword, err := password.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	// 创建用户
	user := &models.User{
		Username: req.Username,
		Email:    req.Email,
		Password: hashedPassword,
		Role:     req.Role,
		Phone:    req.Phone,
		Status:   models.UserStatusActive,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	// 返回时不包含密码
	user.Password = ""
	return user, nil
}

// GetUser 获取用户详情
func (s *Service) GetUser(ctx context.Context, userID int64) (*models.User, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	user.Password = ""
	return user, nil
}

// UpdateUser 更新用户
func (s *Service) UpdateUser(ctx context.Context, userID int64, req *UpdateUserRequest) (*models.User, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	// 更新字段
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Role != "" {
		user.Role = req.Role
	}
	if req.Phone != "" {
		user.Phone = req.Phone
	}
	if req.Status != "" {
		user.Status = req.Status
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	user.Password = ""
	return user, nil
}

// UpdatePassword 更新密码
func (s *Service) UpdatePassword(ctx context.Context, userID int64, req *UpdatePasswordRequest) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	// 验证旧密码
	if !password.CheckPassword(req.OldPassword, user.Password) {
		return ErrInvalidPassword
	}

	// 验证新密码复杂度
	if err := password.ValidatePassword(req.NewPassword); err != nil {
		return err
	}

	// 加密新密码
	hashedPassword, err := password.HashPassword(req.NewPassword)
	if err != nil {
		return err
	}

	user.Password = hashedPassword
	return s.userRepo.Update(ctx, user)
}

// DeleteUser 删除用户
func (s *Service) DeleteUser(ctx context.Context, userID, currentUserID int64) error {
	// 不能删除自己
	if userID == currentUserID {
		return ErrCannotDeleteSelf
	}

	return s.userRepo.Delete(ctx, userID)
}

// DisableUser 禁用用户
func (s *Service) DisableUser(ctx context.Context, userID, currentUserID int64) error {
	// 不能禁用自己
	if userID == currentUserID {
		return ErrCannotDisableSelf
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	user.Status = models.UserStatusDisabled
	return s.userRepo.Update(ctx, user)
}

// EnableUser 启用用户
func (s *Service) EnableUser(ctx context.Context, userID int64) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	user.Status = models.UserStatusActive
	return s.userRepo.Update(ctx, user)
}

// ListUsers 获取用户列表
func (s *Service) ListUsers(ctx context.Context, offset, limit int, keyword string) ([]*models.User, int64, error) {
	users, total, err := s.userRepo.List(ctx, offset, limit, keyword)
	if err != nil {
		return nil, 0, err
	}

	// 清除密码
	for _, user := range users {
		user.Password = ""
	}

	return users, total, nil
}