package kerberos

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Manager Kerberos 配置文件管理器
type Manager struct {
	baseDir string // 基础目录，如 ./kerberos
}

// NewManager 创建 Kerberos 管理器
func NewManager(baseDir string) *Manager {
	return &Manager{baseDir: baseDir}
}

// GetClusterDir 获取集群的 Kerberos 配置目录
func (m *Manager) GetClusterDir(clusterID int64) string {
	return filepath.Join(m.baseDir, fmt.Sprintf("cluster_%d", clusterID))
}

// SaveKrb5Conf 保存 krb5.conf 文件
// 返回文件路径（相对路径）
func (m *Manager) SaveKrb5Conf(clusterID int64, content string) (string, error) {
	clusterDir := m.GetClusterDir(clusterID)
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cluster directory: %w", err)
	}

	filePath := filepath.Join(clusterDir, "krb5.conf")
	if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
		return "", fmt.Errorf("failed to write krb5.conf: %w", err)
	}

	return "krb5.conf", nil
}

// SaveKeytab 保存 keytab 文件
// 返回文件名（相对路径）
func (m *Manager) SaveKeytab(clusterID int64, data []byte) (string, error) {
	clusterDir := m.GetClusterDir(clusterID)
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cluster directory: %w", err)
	}

	// 生成随机文件名
	uuid := make([]byte, 16)
	if _, err := rand.Read(uuid); err != nil {
		return "", fmt.Errorf("failed to generate uuid: %w", err)
	}
	filename := hex.EncodeToString(uuid) + ".keytab"

	filePath := filepath.Join(clusterDir, filename)
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write keytab: %w", err)
	}

	return filename, nil
}

// GetKrb5ConfPath 获取 krb5.conf 完整路径
func (m *Manager) GetKrb5ConfPath(clusterID int64) string {
	return filepath.Join(m.GetClusterDir(clusterID), "krb5.conf")
}

// GetKeytabPath 获取 keytab 完整路径
func (m *Manager) GetKeytabPath(clusterID int64, filename string) string {
	return filepath.Join(m.GetClusterDir(clusterID), filename)
}

// DeleteClusterFiles 删除集群的所有 Kerberos 配置文件
func (m *Manager) DeleteClusterFiles(clusterID int64) error {
	clusterDir := m.GetClusterDir(clusterID)
	return os.RemoveAll(clusterDir)
}

// ExtractRealm 从 Principal 中提取 Realm
// Principal 格式: user/hostname@REALM 或 user@REALM
func ExtractRealm(principal string) (string, error) {
	atIndex := strings.LastIndex(principal, "@")
	if atIndex == -1 || atIndex == len(principal)-1 {
		return "", fmt.Errorf("invalid principal format: missing realm (expected format: user@REALM or user/hostname@REALM)")
	}
	return principal[atIndex+1:], nil
}

// EnsureBaseDir 确保基础目录存在
func (m *Manager) EnsureBaseDir() error {
	return os.MkdirAll(m.baseDir, 0755)
}
