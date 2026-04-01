package password

import (
	"errors"
	"regexp"

	"golang.org/x/crypto/bcrypt"
)

const (
	// BcryptCost bcrypt 加密成本（推荐值 12）
	BcryptCost = 12
	// MinPasswordLength 最小密码长度
	MinPasswordLength = 8
	// MaxPasswordLength 最大密码长度
	MaxPasswordLength = 128
)

var (
	// ErrPasswordTooShort 密码太短
	ErrPasswordTooShort = errors.New("password must be at least 8 characters")
	// ErrPasswordTooLong 密码太长
	ErrPasswordTooLong = errors.New("password must be at most 128 characters")
	// ErrPasswordTooWeak 密码太弱
	ErrPasswordTooWeak = errors.New("password must contain uppercase, lowercase letters and numbers")
)

// Hash 使用 bcrypt 加密密码
func Hash(password string) (string, error) {
	// 验证密码长度
	if len(password) < MinPasswordLength {
		return "", ErrPasswordTooShort
	}
	if len(password) > MaxPasswordLength {
		return "", ErrPasswordTooLong
	}

	// 使用 bcrypt 加密
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

// Verify 验证密码是否匹配
func Verify(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

// ValidateComplexity 验证密码复杂度
// 要求：至少 8 字符，包含大写字母、小写字母和数字
func ValidateComplexity(password string) error {
	// 检查长度
	if len(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	if len(password) > MaxPasswordLength {
		return ErrPasswordTooLong
	}

	// 检查是否包含大写字母
	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	// 检查是否包含小写字母
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	// 检查是否包含数字
	hasNumber := regexp.MustCompile(`[0-9]`).MatchString(password)

	if !hasUpper || !hasLower || !hasNumber {
		return ErrPasswordTooWeak
	}

	return nil
}
