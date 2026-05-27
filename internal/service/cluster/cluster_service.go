package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/repository"
	"kafka-management-platform/pkg/encryption"
	"kafka-management-platform/pkg/kafka"
	"kafka-management-platform/pkg/kerberos"
)

var (
	// ErrClusterNameExists 集群名称已存在
	ErrClusterNameExists = errors.New("cluster name already exists")
	// ErrConnectionTestFailed 连接测试失败
	ErrConnectionTestFailed = errors.New("connection test failed")
	// ErrInvalidKeytabTempID 无效的 keytab 临时 ID
	ErrInvalidKeytabTempID = errors.New("invalid or expired keytab temp id")
)

// tempKeytabStore 临时 keytab 文件存储
type tempKeytabStore struct {
	mu      sync.RWMutex
	store   map[string][]byte    // tempID -> keytab data
	expiry  map[string]time.Time // tempID -> expiry time
	stopCh  chan struct{}
	started bool
	once    sync.Once
}

var globalTempKeytabStore = &tempKeytabStore{
	store:  make(map[string][]byte),
	expiry: make(map[string]time.Time),
}

// ensureStarted 确保 cleanupLoop 已启动（惰性初始化）
func (t *tempKeytabStore) ensureStarted() {
	t.once.Do(func() {
		t.stopCh = make(chan struct{})
		t.started = true
		go t.cleanupLoop()
	})
}

// set 保存临时 keytab
func (t *tempKeytabStore) set(tempID string, data []byte) {
	t.ensureStarted()
	t.mu.Lock()
	defer t.mu.Unlock()
	t.store[tempID] = data
	t.expiry[tempID] = time.Now().Add(30 * time.Minute) // 30 分钟过期
}

// get 获取临时 keytab（不删除，允许重复获取）
func (t *tempKeytabStore) get(tempID string) ([]byte, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	data, ok := t.store[tempID]
	if !ok {
		return nil, false
	}
	// 检查是否过期
	if time.Now().After(t.expiry[tempID]) {
		return nil, false
	}
	return data, true
}

// delete 删除临时 keytab
func (t *tempKeytabStore) delete(tempID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.store, tempID)
	delete(t.expiry, tempID)
}

// cleanupLoop 定期清理过期的临时 keytab
func (t *tempKeytabStore) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			t.mu.Lock()
			now := time.Now()
			for id, exp := range t.expiry {
				if now.After(exp) {
					delete(t.store, id)
					delete(t.expiry, id)
				}
			}
			t.mu.Unlock()
		case <-t.stopCh:
			return
		}
	}
}

// Service 集群管理服务
type Service struct {
	clusterRepo     repository.ClusterRepository
	clusterUserRepo repository.ClusterUserRepository
	encryptionSvc   *encryption.Service
	kerberosMgr     *kerberos.Manager
}

// NewService 创建集群管理服务实例
func NewService(
	clusterRepo repository.ClusterRepository,
	clusterUserRepo repository.ClusterUserRepository,
	encryptionSvc *encryption.Service,
	kerberosMgr *kerberos.Manager,
) *Service {
	// 确保 kerberos 基础目录存在
	if kerberosMgr != nil {
		kerberosMgr.EnsureBaseDir()
	}
	return &Service{
		clusterRepo:     clusterRepo,
		clusterUserRepo: clusterUserRepo,
		encryptionSvc:   encryptionSvc,
		kerberosMgr:     kerberosMgr,
	}
}

// CreateClusterRequest 创建集群请求
type CreateClusterRequest struct {
	ClusterName      string                 `json:"cluster_name" binding:"required"`
	BootstrapServers string                 `json:"bootstrap_servers" binding:"required"`
	AuthType         models.AuthType        `json:"auth_type" binding:"required"`
	AuthConfig       map[string]interface{} `json:"auth_config"`
	JMXExporterURLs  string                 `json:"jmx_exporter_urls"` // 多个 URL 逗号分隔
	Description      string                 `json:"description"`
	CreatedBy        int64                  `json:"-"`
}

// UpdateClusterRequest 更新集群请求
// 只有集群名称、JMX Exporter URLs、描述可以修改
// Bootstrap Servers 和认证配置不可修改，避免需要重新测试连接
type UpdateClusterRequest struct {
	ClusterName     string               `json:"cluster_name"`
	JMXExporterURLs string               `json:"jmx_exporter_urls"` // 多个 URL 逗号分隔
	Description     string               `json:"description"`
	Status          models.ClusterStatus `json:"status"`
}

// CreateCluster 创建集群
// 在创建前会先测试 Kafka 集群连接，只有连接成功才会保存集群配置
func (s *Service) CreateCluster(ctx context.Context, req *CreateClusterRequest) (*models.Cluster, error) {
	// 检查集群名称是否已存在
	existing, err := s.clusterRepo.FindByName(ctx, req.ClusterName)
	if err == nil && existing != nil {
		return nil, ErrClusterNameExists
	}

	// 处理认证配置
	authConfig := req.AuthConfig
	if authConfig == nil {
		authConfig = make(map[string]interface{})
	}

	// 如果是 Kerberos 认证，处理临时文件
	if req.AuthType == models.AuthTypeKerberos {
		authConfig, err = s.prepareKerberosAuthConfigForCreate(authConfig, 0)
		if err != nil {
			return nil, err
		}
	}

	// 准备认证配置 JSON
	var authConfigJSON string
	if len(authConfig) > 0 {
		jsonBytes, err := json.Marshal(authConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal auth config: %w", err)
		}
		authConfigJSON = string(jsonBytes)
	}

	// 创建临时集群对象用于测试连接
	tempCluster := &models.Cluster{
		BootstrapServers: req.BootstrapServers,
		AuthType:         req.AuthType,
	}

	// 测试 Kafka 集群连接
	if err := s.testKafkaConnection(tempCluster, authConfigJSON); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConnectionTestFailed, err)
	}

	// 连接测试成功，保存 Kerberos 文件到正式目录（如果有）
	// 注意：必须在 finalize 之前提取 _keytab_data，因为 finalize 会删除它
	var keytabDataToSave []byte
	if req.AuthType == models.AuthTypeKerberos {
		// 提取 keytab 数据用于后续保存
		if data, ok := authConfig["_keytab_data"].([]byte); ok {
			keytabDataToSave = data
		}

		authConfig, err = s.finalizeKerberosAuthConfig(authConfig, 0)
		if err != nil {
			return nil, err
		}
		// 重新序列化
		jsonBytes, err := json.Marshal(authConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal auth config: %w", err)
		}
		authConfigJSON = string(jsonBytes)
	}

	// 提取 SASL 机制（用于前端过滤）
	var saslMechanism string
	if req.AuthType == models.AuthTypeSCRAM {
		if m, ok := authConfig["mechanism"].(string); ok {
			saslMechanism = m
		}
	}

	// 加密认证配置
	var authConfigEncrypted string
	if authConfigJSON != "" {
		authConfigEncrypted, err = s.encryptionSvc.EncryptString(authConfigJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt auth config: %w", err)
		}
	}

	// 创建集群记录
	cluster := &models.Cluster{
		ClusterName:      req.ClusterName,
		BootstrapServers: req.BootstrapServers,
		AuthType:         req.AuthType,
		SASLMechanism:    saslMechanism,
		AuthConfig:       authConfigEncrypted,
		JMXExporterURLs:  req.JMXExporterURLs,
		Description:      req.Description,
		Status:           models.ClusterStatusActive,
		CreatedBy:        req.CreatedBy,
	}

	if err := s.clusterRepo.Create(ctx, cluster); err != nil {
		return nil, err
	}

	// 创建成功后，将 Kerberos 文件保存到集群目录
	if req.AuthType == models.AuthTypeKerberos {
		if err := s.saveKerberosFiles(authConfig, cluster.ClusterID, keytabDataToSave); err != nil {
			// 文件保存失败，但集群已创建，记录错误但不回滚
			fmt.Printf("[WARN] failed to save kerberos files: %v\n", err)
		}
		// 删除临时 keytab 数据
		if keytabTempID, ok := req.AuthConfig["keytab_temp_id"].(string); ok {
			globalTempKeytabStore.delete(keytabTempID)
		}
	}

	return cluster, nil
}

// SaveTempKeytab 保存临时 keytab 文件，返回临时 ID
func (s *Service) SaveTempKeytab(ctx context.Context, data []byte) (string, error) {
	// 生成临时 ID
	tempID := fmt.Sprintf("%d", time.Now().UnixNano())
	globalTempKeytabStore.set(tempID, data)
	return tempID, nil
}

// DeleteTempKeytab 删除临时 keytab 文件
func (s *Service) DeleteTempKeytab(ctx context.Context, tempID string) error {
	if tempID == "" {
		return nil
	}
	globalTempKeytabStore.delete(tempID)
	return nil
}

// prepareKerberosAuthConfigForCreate 准备 Kerberos 认证配置（用于测试连接）
// 创建临时文件用于测试
func (s *Service) prepareKerberosAuthConfigForCreate(authConfig map[string]interface{}, clusterID int64) (map[string]interface{}, error) {
	// 获取 krb5.conf 内容
	krb5Content, _ := authConfig["krb5_content"].(string)
	if krb5Content == "" {
		return nil, fmt.Errorf("missing krb5_content in auth config")
	}

	// 获取 keytab 临时 ID
	keytabTempID, _ := authConfig["keytab_temp_id"].(string)
	if keytabTempID == "" {
		return nil, fmt.Errorf("missing keytab_temp_id in auth config")
	}

	// 获取临时 keytab 数据
	keytabData, ok := globalTempKeytabStore.get(keytabTempID)
	if !ok {
		return nil, ErrInvalidKeytabTempID
	}

	// 创建临时目录
	tempDir := filepath.Join(os.TempDir(), "kmanager-kerberos")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// 写入临时 krb5.conf
	krb5TempPath := filepath.Join(tempDir, fmt.Sprintf("krb5_%d.conf", time.Now().UnixNano()))
	if err := os.WriteFile(krb5TempPath, []byte(krb5Content), 0600); err != nil {
		return nil, fmt.Errorf("failed to write temp krb5.conf: %w", err)
	}

	// 写入临时 keytab
	keytabTempPath := filepath.Join(tempDir, fmt.Sprintf("keytab_%d.keytab", time.Now().UnixNano()))
	if err := os.WriteFile(keytabTempPath, keytabData, 0600); err != nil {
		os.Remove(krb5TempPath)
		return nil, fmt.Errorf("failed to write temp keytab: %w", err)
	}

	// 创建新的配置，包含临时文件路径
	newConfig := make(map[string]interface{})
	for k, v := range authConfig {
		newConfig[k] = v
	}
	newConfig["krb5_conf_path"] = krb5TempPath
	newConfig["keytab_path"] = keytabTempPath
	newConfig["_keytab_data"] = keytabData // 保存数据用于后续移动

	// 从 principal 中提取 realm（如果没有提供）
	if _, ok := newConfig["realm"]; !ok {
		principal, _ := newConfig["principal"].(string)
		if principal != "" {
			realm, err := kerberos.ExtractRealm(principal)
			if err != nil {
				return nil, err
			}
			newConfig["realm"] = realm
		}
	}

	return newConfig, nil
}

// finalizeKerberosAuthConfig 最终化 Kerberos 配置（去除临时路径，保留内容）
func (s *Service) finalizeKerberosAuthConfig(authConfig map[string]interface{}, clusterID int64) (map[string]interface{}, error) {
	newConfig := make(map[string]interface{})
	for k, v := range authConfig {
		// 跳过临时路径和内部数据
		if k == "krb5_conf_path" || k == "keytab_path" || k == "_keytab_data" || k == "keytab_temp_id" {
			continue
		}
		newConfig[k] = v
	}
	return newConfig, nil
}

// saveKerberosFiles 保存 Kerberos 文件到集群目录
func (s *Service) saveKerberosFiles(authConfig map[string]interface{}, clusterID int64, keytabData []byte) error {
	if s.kerberosMgr == nil {
		return nil
	}

	// 获取 krb5.conf 内容
	krb5Content, _ := authConfig["krb5_content"].(string)

	if krb5Content == "" {
		return nil
	}

	// 保存 krb5.conf
	if _, err := s.kerberosMgr.SaveKrb5Conf(clusterID, krb5Content); err != nil {
		return fmt.Errorf("failed to save krb5.conf: %w", err)
	}

	// 保存 keytab
	if len(keytabData) > 0 {
		if _, err := s.kerberosMgr.SaveKeytab(clusterID, keytabData); err != nil {
			return fmt.Errorf("failed to save keytab: %w", err)
		}
	}

	return nil
}

// UpdateCluster 更新集群
// 只允许更新集群名称、JMX Exporter URL、描述
// Bootstrap Servers 和认证配置不可修改
func (s *Service) UpdateCluster(ctx context.Context, clusterID int64, req *UpdateClusterRequest) error {
	// 获取现有集群
	cluster, err := s.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return err
	}

	// 更新字段
	if req.ClusterName != "" {
		cluster.ClusterName = req.ClusterName
	}
	if req.JMXExporterURLs != "" {
		cluster.JMXExporterURLs = req.JMXExporterURLs
	}
	if req.Description != "" {
		cluster.Description = req.Description
	}
	if req.Status != "" {
		cluster.Status = req.Status
	}

	return s.clusterRepo.Update(ctx, cluster)
}

// prepareKerberosAuthConfigForUpdate 准备更新时的 Kerberos 配置
func (s *Service) prepareKerberosAuthConfigForUpdate(authConfig map[string]interface{}, clusterID int64, newKeytabData []byte) (map[string]interface{}, error) {
	// 获取 krb5.conf 内容
	krb5Content, _ := authConfig["krb5_content"].(string)
	if krb5Content == "" {
		return nil, fmt.Errorf("missing krb5_content in auth config")
	}

	// 创建临时目录
	tempDir := filepath.Join(os.TempDir(), "kmanager-kerberos")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// 写入临时 krb5.conf
	krb5TempPath := filepath.Join(tempDir, fmt.Sprintf("krb5_%d.conf", time.Now().UnixNano()))
	if err := os.WriteFile(krb5TempPath, []byte(krb5Content), 0600); err != nil {
		return nil, fmt.Errorf("failed to write temp krb5.conf: %w", err)
	}

	// 处理 keytab
	var keytabData []byte
	if len(newKeytabData) > 0 {
		keytabData = newKeytabData
	} else {
		// 使用现有的 keytab 文件
		if s.kerberosMgr != nil {
			keytabFile, _ := authConfig["keytab_file"].(string)
			if keytabFile == "" {
				keytabFile = "current.keytab"
			}
			keytabPath := s.kerberosMgr.GetKeytabPath(clusterID, keytabFile)
			data, err := os.ReadFile(keytabPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read existing keytab: %w", err)
			}
			keytabData = data
		}
	}

	// 写入临时 keytab
	keytabTempPath := filepath.Join(tempDir, fmt.Sprintf("keytab_%d.keytab", time.Now().UnixNano()))
	if err := os.WriteFile(keytabTempPath, keytabData, 0600); err != nil {
		os.Remove(krb5TempPath)
		return nil, fmt.Errorf("failed to write temp keytab: %w", err)
	}

	// 创建新的配置
	newConfig := make(map[string]interface{})
	for k, v := range authConfig {
		newConfig[k] = v
	}
	newConfig["krb5_conf_path"] = krb5TempPath
	newConfig["keytab_path"] = keytabTempPath
	newConfig["_keytab_data"] = keytabData

	// 从 principal 中提取 realm（如果没有提供）
	if _, ok := newConfig["realm"]; !ok {
		principal, _ := newConfig["principal"].(string)
		if principal != "" {
			realm, err := kerberos.ExtractRealm(principal)
			if err != nil {
				return nil, err
			}
			newConfig["realm"] = realm
		}
	}

	return newConfig, nil
}

// saveKerberosFilesForUpdate 保存更新时的 Kerberos 文件
func (s *Service) saveKerberosFilesForUpdate(authConfig map[string]interface{}, clusterID int64, keytabData []byte) error {
	if s.kerberosMgr == nil {
		return nil
	}

	krb5Content, _ := authConfig["krb5_content"].(string)
	if krb5Content == "" {
		return nil
	}

	// 保存 krb5.conf
	if _, err := s.kerberosMgr.SaveKrb5Conf(clusterID, krb5Content); err != nil {
		return fmt.Errorf("failed to save krb5.conf: %w", err)
	}

	// 保存 keytab
	if len(keytabData) > 0 {
		if _, err := s.kerberosMgr.SaveKeytab(clusterID, keytabData); err != nil {
			return fmt.Errorf("failed to save keytab: %w", err)
		}
	}

	return nil
}

// DeleteCluster 删除集群
func (s *Service) DeleteCluster(ctx context.Context, clusterID int64) error {
	return s.clusterRepo.Delete(ctx, clusterID)
}

// GetCluster 获取集群详情
func (s *Service) GetCluster(ctx context.Context, clusterID int64) (*models.Cluster, error) {
	cluster, err := s.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	// 解密认证配置（用于管理员查看）
	if cluster.AuthConfig != "" {
		decrypted, err := s.encryptionSvc.DecryptString(cluster.AuthConfig)
		if err != nil {
			// 解密失败，返回空配置
			cluster.AuthConfig = ""
		} else {
			cluster.AuthConfig = decrypted
		}
	}

	return cluster, nil
}

// ListClusters 获取集群列表
func (s *Service) ListClusters(ctx context.Context, userID int64, role models.UserRole, offset, limit int) ([]*models.Cluster, int64, error) {
	return s.clusterRepo.ListByUser(ctx, userID, role, offset, limit)
}

// GrantClusterAccess 授予用户集群访问权限
func (s *Service) GrantClusterAccess(ctx context.Context, clusterID, userID int64) error {
	relation := &models.ClusterUserRelation{
		ClusterID: clusterID,
		UserID:    userID,
	}
	return s.clusterUserRepo.Create(ctx, relation)
}

// RevokeClusterAccess 撤销用户集群访问权限
func (s *Service) RevokeClusterAccess(ctx context.Context, clusterID, userID int64) error {
	return s.clusterUserRepo.Delete(ctx, clusterID, userID)
}

// ListClusterUsers 获取集群的授权用户列表
func (s *Service) ListClusterUsers(ctx context.Context, clusterID int64) ([]*models.User, error) {
	return s.clusterUserRepo.ListUsersByCluster(ctx, clusterID)
}

// ListUserClusters 获取用户已授权的集群列表
func (s *Service) ListUserClusters(ctx context.Context, userID int64) ([]*models.Cluster, error) {
	return s.clusterUserRepo.ListClustersByUser(ctx, userID)
}

// TestConnection 测试集群连接
func (s *Service) TestConnection(ctx context.Context, clusterID int64) error {
	// 获取集群配置
	cluster, err := s.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}

	// 解密认证配置
	var authConfigJSON string
	if cluster.AuthConfig != "" {
		decrypted, err := s.encryptionSvc.DecryptString(cluster.AuthConfig)
		if err != nil {
			return fmt.Errorf("failed to decrypt auth config: %w", err)
		}
		authConfigJSON = decrypted
	}

	// 如果是 Kerberos，需要准备运行时文件路径
	if cluster.AuthType == models.AuthTypeKerberos {
		authConfigJSON, err = s.prepareKerberosRuntimeConfig(authConfigJSON, clusterID)
		if err != nil {
			return err
		}
	}

	return s.testKafkaConnection(cluster, authConfigJSON)
}

// TestConnectionForCreate 在创建集群前测试连接配置
// 用于前端在提交创建请求前验证连接
func (s *Service) TestConnectionForCreate(ctx context.Context, req *CreateClusterRequest) error {
	// 处理认证配置
	authConfig := req.AuthConfig
	if authConfig == nil {
		authConfig = make(map[string]interface{})
	}

	// 如果是 Kerberos，准备临时文件
	if req.AuthType == models.AuthTypeKerberos {
		var err error
		authConfig, err = s.prepareKerberosAuthConfigForCreate(authConfig, 0)
		if err != nil {
			return err
		}
	}

	// 准备认证配置 JSON
	var authConfigJSON string
	if len(authConfig) > 0 {
		jsonBytes, err := json.Marshal(authConfig)
		if err != nil {
			return fmt.Errorf("failed to marshal auth config: %w", err)
		}
		authConfigJSON = string(jsonBytes)
	}

	// 创建临时集群对象用于测试连接
	tempCluster := &models.Cluster{
		BootstrapServers: req.BootstrapServers,
		AuthType:         req.AuthType,
	}

	return s.testKafkaConnection(tempCluster, authConfigJSON)
}

// prepareKerberosRuntimeConfig 准备 Kerberos 运行时配置（从已保存的文件读取）
func (s *Service) prepareKerberosRuntimeConfig(authConfigJSON string, clusterID int64) (string, error) {
	var authConfig map[string]interface{}
	if err := json.Unmarshal([]byte(authConfigJSON), &authConfig); err != nil {
		return "", fmt.Errorf("failed to parse auth config: %w", err)
	}

	// 添加文件路径
	if s.kerberosMgr != nil {
		authConfig["krb5_conf_path"] = s.kerberosMgr.GetKrb5ConfPath(clusterID)
		keytabFile, _ := authConfig["keytab_file"].(string)
		if keytabFile == "" {
			// 尝试查找目录下的 keytab 文件
			clusterDir := s.kerberosMgr.GetClusterDir(clusterID)
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
			authConfig["keytab_path"] = s.kerberosMgr.GetKeytabPath(clusterID, keytabFile)
		}
	}

	// 从 principal 提取 realm（如果没有）
	if _, ok := authConfig["realm"]; !ok {
		principal, _ := authConfig["principal"].(string)
		if principal != "" {
			realm, err := kerberos.ExtractRealm(principal)
			if err != nil {
				return "", err
			}
			authConfig["realm"] = realm
		}
	}

	jsonBytes, err := json.Marshal(authConfig)
	if err != nil {
		return "", fmt.Errorf("failed to marshal auth config: %w", err)
	}
	return string(jsonBytes), nil
}

// testKafkaConnection 测试 Kafka 集群连接的内部方法
func (s *Service) testKafkaConnection(cluster *models.Cluster, authConfigJSON string) error {
	// 创建 Kafka Admin 客户端
	adminClient, err := kafka.NewAdminClient(cluster, authConfigJSON)
	if err != nil {
		return fmt.Errorf("failed to create kafka admin client: %w", err)
	}
	defer adminClient.Close()

	// 测试连接
	if err := adminClient.TestConnection(); err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}

	return nil
}

// GetAuthConfigForCluster 获取集群的认证配置（包含运行时路径）
// 用于其他服务创建 Kafka Admin 客户端
func (s *Service) GetAuthConfigForCluster(ctx context.Context, clusterID int64) (*models.Cluster, string, error) {
	cluster, err := s.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get cluster: %w", err)
	}

	// 解密认证配置
	var authConfigJSON string
	if cluster.AuthConfig != "" {
		decrypted, err := s.encryptionSvc.DecryptString(cluster.AuthConfig)
		if err != nil {
			return nil, "", fmt.Errorf("failed to decrypt auth config: %w", err)
		}
		authConfigJSON = decrypted
	}

	// 如果是 Kerberos，准备运行时配置
	if cluster.AuthType == models.AuthTypeKerberos {
		authConfigJSON, err = s.prepareKerberosRuntimeConfig(authConfigJSON, clusterID)
		if err != nil {
			return nil, "", err
		}
	}

	return cluster, authConfigJSON, nil
}
