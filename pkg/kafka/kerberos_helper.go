package kafka

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kafka-management-platform/internal/models"
)

// KerberosPathProvider 提供 Kerberos 文件路径
type KerberosPathProvider interface {
	GetKrb5ConfPath(clusterID int64) string
	GetKeytabPath(clusterID int64, filename string) string
	GetClusterDir(clusterID int64) string
}

// PrepareKerberosAuthConfig 准备 Kerberos 认证配置
// 添加运行时文件路径到 auth config
func PrepareKerberosAuthConfig(authConfigJSON string, clusterID int64, kerberosBaseDir string) (string, error) {
	if authConfigJSON == "" {
		return "", nil
	}

	var authConfig map[string]interface{}
	if err := json.Unmarshal([]byte(authConfigJSON), &authConfig); err != nil {
		return "", fmt.Errorf("failed to parse auth config: %w", err)
	}

	// 如果已经有 krb5_conf_path，说明已经准备好了
	if _, ok := authConfig["krb5_conf_path"]; ok {
		return authConfigJSON, nil
	}

	// 计算 kerberos 文件目录
	clusterDir := filepath.Join(kerberosBaseDir, fmt.Sprintf("cluster_%d", clusterID))

	// 添加 krb5.conf 路径
	krb5ConfPath := filepath.Join(clusterDir, "krb5.conf")
	authConfig["krb5_conf_path"] = krb5ConfPath

	// 查找 keytab 文件
	keytabFile, _ := authConfig["keytab_file"].(string)
	if keytabFile == "" {
		// 尝试查找目录下的 keytab 文件
		files, err := os.ReadDir(clusterDir)
		if err == nil {
			for _, f := range files {
				if filepath.Ext(f.Name()) == ".keytab" {
					keytabFile = f.Name()
					break
				}
			}
		}
	}
	if keytabFile != "" {
		authConfig["keytab_path"] = filepath.Join(clusterDir, keytabFile)
	}

	// 从 principal 提取 realm（如果没有）
	if _, ok := authConfig["realm"]; !ok {
		principal, _ := authConfig["principal"].(string)
		if principal != "" {
			atIndex := strings.LastIndex(principal, "@")
			if atIndex != -1 && atIndex < len(principal)-1 {
				authConfig["realm"] = principal[atIndex+1:]
			}
		}
	}

	jsonBytes, err := json.Marshal(authConfig)
	if err != nil {
		return "", fmt.Errorf("failed to marshal auth config: %w", err)
	}
	return string(jsonBytes), nil
}

// NewAdminClientWithKerberos 创建 Kafka Admin 客户端（支持 Kerberos 运行时配置）
func NewAdminClientWithKerberos(cluster *models.Cluster, authConfigJSON string, kerberosBaseDir string) (*AdminClient, error) {
	// 如果是 Kerberos 认证，准备运行时配置
	if cluster.AuthType == models.AuthTypeKerberos && kerberosBaseDir != "" {
		var err error
		authConfigJSON, err = PrepareKerberosAuthConfig(authConfigJSON, cluster.ClusterID, kerberosBaseDir)
		if err != nil {
			return nil, err
		}
	}
	return NewAdminClient(cluster, authConfigJSON)
}
