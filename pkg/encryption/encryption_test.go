package encryption

import (
	"encoding/base64"
	"testing"
)

// 生成测试用的 32 字节密钥
func getTestKey() string {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return base64.StdEncoding.EncodeToString(key)
}

func TestNewService(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "valid key",
			key:     getTestKey(),
			wantErr: false,
		},
		{
			name:    "invalid base64",
			key:     "invalid-base64!!!",
			wantErr: true,
		},
		{
			name:    "wrong key size",
			key:     base64.StdEncoding.EncodeToString([]byte("short")),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewService(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewService() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEncryptDecrypt(t *testing.T) {
	service, err := NewService(getTestKey())
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	tests := []struct {
		name      string
		plaintext string
	}{
		{
			name:      "simple text",
			plaintext: "hello world",
		},
		{
			name:      "json data",
			plaintext: `{"username":"admin","password":"secret"}`,
		},
		{
			name:      "chinese text",
			plaintext: "你好世界",
		},
		{
			name:      "special characters",
			plaintext: "!@#$%^&*()_+-=[]{}|;:',.<>?/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 加密
			ciphertext, err := service.EncryptString(tt.plaintext)
			if err != nil {
				t.Fatalf("Encrypt() error = %v", err)
			}

			// 验证密文不为空
			if ciphertext == "" {
				t.Error("Encrypt() returned empty ciphertext")
			}

			// 解密
			decrypted, err := service.DecryptString(ciphertext)
			if err != nil {
				t.Fatalf("Decrypt() error = %v", err)
			}

			// 验证解密结果与原文一致
			if decrypted != tt.plaintext {
				t.Errorf("Decrypt() = %v, want %v", decrypted, tt.plaintext)
			}
		})
	}
}

func TestEncryptEmpty(t *testing.T) {
	service, err := NewService(getTestKey())
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	_, err = service.Encrypt([]byte{})
	if err != ErrEmptyPlaintext {
		t.Errorf("Encrypt(empty) error = %v, want %v", err, ErrEmptyPlaintext)
	}
}

func TestDecryptEmpty(t *testing.T) {
	service, err := NewService(getTestKey())
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	_, err = service.Decrypt("")
	if err != ErrEmptyCiphertext {
		t.Errorf("Decrypt(empty) error = %v, want %v", err, ErrEmptyCiphertext)
	}
}

func TestDecryptInvalid(t *testing.T) {
	service, err := NewService(getTestKey())
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	tests := []struct {
		name       string
		ciphertext string
	}{
		{
			name:       "invalid base64",
			ciphertext: "invalid-base64!!!",
		},
		{
			name:       "too short",
			ciphertext: base64.StdEncoding.EncodeToString([]byte("short")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.Decrypt(tt.ciphertext)
			if err == nil {
				t.Error("Decrypt() expected error, got nil")
			}
		})
	}
}

func TestEncryptDifferentIV(t *testing.T) {
	service, err := NewService(getTestKey())
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	plaintext := "test data"

	// 加密两次相同的明文
	ciphertext1, err := service.EncryptString(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	ciphertext2, err := service.EncryptString(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// 验证两次加密结果不同（因为 IV 随机）
	if ciphertext1 == ciphertext2 {
		t.Error("Encrypt() produced same ciphertext for same plaintext (IV should be random)")
	}

	// 验证两次解密结果都正确
	decrypted1, err := service.DecryptString(ciphertext1)
	if err != nil || decrypted1 != plaintext {
		t.Errorf("Decrypt(ciphertext1) = %v, want %v", decrypted1, plaintext)
	}

	decrypted2, err := service.DecryptString(ciphertext2)
	if err != nil || decrypted2 != plaintext {
		t.Errorf("Decrypt(ciphertext2) = %v, want %v", decrypted2, plaintext)
	}
}
