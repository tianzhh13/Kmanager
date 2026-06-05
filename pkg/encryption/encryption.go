package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

const (
	// encryptionVersionGCM GCM 加密版本标识
	encryptionVersionGCM byte = 2
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
// 算法：AES-256-GCM（认证加密，提供密文完整性验证）
// 输入：明文字节数组
// 输出：Base64 编码的密文（包含版本标识 + nonce + 密文）
func (s *Service) Encrypt(plaintext []byte) (string, error) {
	if len(plaintext) == 0 {
		return "", ErrEmptyPlaintext
	}

	// 创建 AES cipher
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}

	// 使用 GCM 模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// 生成随机 nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// GCM 加密（自动附加认证标签）
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// 添加版本标识前缀（用于解密时自动识别算法）
	result := append([]byte{encryptionVersionGCM}, ciphertext...)

	return base64.StdEncoding.EncodeToString(result), nil
}

// Decrypt 解密数据
// 支持自动识别版本：v2(GCM) 向前兼容旧版无前缀(CFB)
// 输入：Base64 编码的密文
// 输出：明文字节数组
func (s *Service) Decrypt(ciphertextBase64 string) ([]byte, error) {
	if ciphertextBase64 == "" {
		return nil, ErrEmptyCiphertext
	}

	// Base64 解码
	data, err := base64.StdEncoding.DecodeString(ciphertextBase64)
	if err != nil {
		return nil, errors.New("failed to decode ciphertext: " + err.Error())
	}

	if len(data) == 0 {
		return nil, ErrEmptyCiphertext
	}

	// 根据版本标识选择解密算法
	if data[0] == encryptionVersionGCM {
		return s.decryptGCM(data[1:])
	}

	// 无版本标识 → 旧版 CFB（向后兼容）
	return s.decryptCFB(data)
}

// decryptGCM 使用 AES-256-GCM 解密
func (s *Service) decryptGCM(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, ErrInvalidCiphertext
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrInvalidCiphertext
	}

	return plaintext, nil
}

// decryptCFB 使用 AES-256-CFB 解密（向后兼容旧版加密数据）
func (s *Service) decryptCFB(data []byte) ([]byte, error) {
	if len(data) < aes.BlockSize {
		return nil, ErrInvalidCiphertext
	}

	iv := data[:aes.BlockSize]
	ciphertext := data[aes.BlockSize:]

	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}

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
