package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

var (
	// ErrEmptyPlaintext 空明文错误
	ErrEmptyPlaintext = errors.New("plaintext cannot be empty")
	// ErrEmptyCiphertext 空密文错误
	ErrEmptyCiphertext = errors.New("ciphertext cannot be empty")
	// ErrInvalidKeySize 无效密钥长度错误
	ErrInvalidKeySize = errors.New("key must be 32 bytes for AES-256")
	// ErrInvalidCiphertext 无效密文错误
	ErrInvalidCiphertext = errors.New("invalid ciphertext")
)

// Service 加密服务
type Service struct {
	key []byte
}

// NewService 创建加密服务实例
// key: 32 字节的 AES-256 密钥（Base64 编码）
func NewService(keyBase64 string) (*Service, error) {
	// Base64 解码密钥
	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return nil, errors.New("failed to decode encryption key: " + err.Error())
	}

	// 验证密钥长度
	if len(key) != 32 {
		return nil, ErrInvalidKeySize
	}

	return &Service{key: key}, nil
}

// Encrypt 加密数据
// 算法：AES-256-CFB
// 输入：明文字节数组
// 输出：Base64 编码的密文（包含 IV）
func (s *Service) Encrypt(plaintext []byte) (string, error) {
	// 前置条件检查
	if len(plaintext) == 0 {
		return "", ErrEmptyPlaintext
	}

	// 步骤 1：创建 AES cipher
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}

	// 步骤 2：生成随机 IV（初始化向量）
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	// 步骤 3：使用 CFB 模式加密
	stream := cipher.NewCFBEncrypter(block, iv)
	ciphertext := make([]byte, len(plaintext))
	stream.XORKeyStream(ciphertext, plaintext)

	// 步骤 4：将 IV 和密文组合（IV + ciphertext）
	result := append(iv, ciphertext...)

	// 步骤 5：Base64 编码
	encoded := base64.StdEncoding.EncodeToString(result)

	return encoded, nil
}

// Decrypt 解密数据
// 算法：AES-256-CFB
// 输入：Base64 编码的密文（包含 IV）
// 输出：明文字节数组
func (s *Service) Decrypt(ciphertextBase64 string) ([]byte, error) {
	// 前置条件检查
	if ciphertextBase64 == "" {
		return nil, ErrEmptyCiphertext
	}

	// 步骤 1：Base64 解码
	data, err := base64.StdEncoding.DecodeString(ciphertextBase64)
	if err != nil {
		return nil, errors.New("failed to decode ciphertext: " + err.Error())
	}

	// 步骤 2：检查数据长度（至少包含 IV）
	if len(data) < aes.BlockSize {
		return nil, ErrInvalidCiphertext
	}

	// 步骤 3：分离 IV 和密文
	iv := data[:aes.BlockSize]
	ciphertext := data[aes.BlockSize:]

	// 步骤 4：创建 AES cipher
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}

	// 步骤 5：使用 CFB 模式解密
	stream := cipher.NewCFBDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	stream.XORKeyStream(plaintext, ciphertext)

	return plaintext, nil
}

// EncryptString 加密字符串
func (s *Service) EncryptString(plaintext string) (string, error) {
	return s.Encrypt([]byte(plaintext))
}

// DecryptString 解密字符串
func (s *Service) DecryptString(ciphertext string) (string, error) {
	plaintext, err := s.Decrypt(ciphertext)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
