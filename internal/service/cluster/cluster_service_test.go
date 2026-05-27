package cluster

import (
	"context"
	"errors"
	"testing"

	"kafka-management-platform/internal/models"
	"kafka-management-platform/pkg/encryption"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockClusterRepository 是 ClusterRepository 的 mock 实现
type MockClusterRepository struct {
	mock.Mock
}

func (m *MockClusterRepository) Create(ctx context.Context, cluster *models.Cluster) error {
	args := m.Called(ctx, cluster)
	return args.Error(0)
}

func (m *MockClusterRepository) Update(ctx context.Context, cluster *models.Cluster) error {
	args := m.Called(ctx, cluster)
	return args.Error(0)
}

func (m *MockClusterRepository) Delete(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockClusterRepository) FindByID(ctx context.Context, id int64) (*models.Cluster, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Cluster), args.Error(1)
}

func (m *MockClusterRepository) FindByName(ctx context.Context, name string) (*models.Cluster, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Cluster), args.Error(1)
}

func (m *MockClusterRepository) List(ctx context.Context, offset, limit int) ([]*models.Cluster, int64, error) {
	args := m.Called(ctx, offset, limit)
	return args.Get(0).([]*models.Cluster), args.Get(1).(int64), args.Error(2)
}

func (m *MockClusterRepository) ListByUser(ctx context.Context, userID int64, role models.UserRole, offset, limit int) ([]*models.Cluster, int64, error) {
	args := m.Called(ctx, userID, role, offset, limit)
	return args.Get(0).([]*models.Cluster), args.Get(1).(int64), args.Error(2)
}

// MockClusterUserRepository 是 ClusterUserRepository 的 mock 实现
type MockClusterUserRepository struct {
	mock.Mock
}

func (m *MockClusterUserRepository) Create(ctx context.Context, relation *models.ClusterUserRelation) error {
	args := m.Called(ctx, relation)
	return args.Error(0)
}

func (m *MockClusterUserRepository) Delete(ctx context.Context, clusterID, userID int64) error {
	args := m.Called(ctx, clusterID, userID)
	return args.Error(0)
}

func (m *MockClusterUserRepository) HasAccess(ctx context.Context, clusterID, userID int64) (bool, error) {
	args := m.Called(ctx, clusterID, userID)
	return args.Bool(0), args.Error(1)
}

func (m *MockClusterUserRepository) ListUsersByCluster(ctx context.Context, clusterID int64) ([]*models.User, error) {
	args := m.Called(ctx, clusterID)
	return args.Get(0).([]*models.User), args.Error(1)
}

func (m *MockClusterUserRepository) ListClustersByUser(ctx context.Context, userID int64) ([]*models.Cluster, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]*models.Cluster), args.Error(1)
}

func (m *MockClusterUserRepository) DeleteByCluster(ctx context.Context, clusterID int64) error {
	args := m.Called(ctx, clusterID)
	return args.Error(0)
}

func (m *MockClusterUserRepository) DeleteByUser(ctx context.Context, userID int64) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

// 注意：由于 testKafkaConnection 方法需要真实的 Kafka 连接，
// 我们无法在单元测试中 mock 它。这里我们测试的是：
// 1. 集群名称唯一性检查
// 2. 认证配置加密
// 3. 数据库操作
// 连接测试的集成测试会在集成测试文件中进行

// TestCreateCluster_NameExists 测试创建集群时名称已存在的情况
func TestCreateCluster_NameExists(t *testing.T) {
	mockRepo := new(MockClusterRepository)
	mockUserRepo := new(MockClusterUserRepository)

	svc := NewService(
		mockRepo,
		mockUserRepo,
		nil, // encryption service not needed for this test
	)

	// 设置 mock：集群名称已存在
	mockRepo.On("FindByName", mock.Anything, "existing-cluster").Return(&models.Cluster{
		ClusterID:   1,
		ClusterName: "existing-cluster",
	}, nil)

	req := &CreateClusterRequest{
		ClusterName:      "existing-cluster",
		BootstrapServers: "localhost:9092",
		AuthType:         models.AuthTypePlaintext,
		CreatedBy:        1,
	}

	_, err := svc.CreateCluster(context.Background(), req)

	assert.ErrorIs(t, err, ErrClusterNameExists)
	mockRepo.AssertExpectations(t)
}

// TestCreateCluster_EncryptAuthConfig 测试创建集群时加密认证配置
func TestCreateCluster_EncryptAuthConfig(t *testing.T) {
	// 这个测试需要 mock Kafka 连接，暂时跳过
	// 将在集成测试中覆盖
	t.Skip("需要 mock Kafka 连接，将在集成测试中覆盖")
}

// TestUpdateCluster_Success 测试更新集群成功
func TestUpdateCluster_Success(t *testing.T) {
	mockRepo := new(MockClusterRepository)
	mockUserRepo := new(MockClusterUserRepository)

	svc := NewService(
		mockRepo,
		mockUserRepo,
		nil, // encryption service not needed for this test
	)

	// 设置 mock：找到现有集群
	mockRepo.On("FindByID", mock.Anything, int64(1)).Return(&models.Cluster{
		ClusterID:         1,
		ClusterName:       "test-cluster",
		BootstrapServers:  "localhost:9092",
		AuthType:          models.AuthTypePlaintext,
		Status:            models.ClusterStatusActive,
	}, nil)

	// 设置 mock：更新成功
	mockRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

	req := &UpdateClusterRequest{
		ClusterName:      "updated-cluster",
		BootstrapServers: "localhost:9093",
	}

	err := svc.UpdateCluster(context.Background(), 1, req)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// TestUpdateCluster_NotFound 测试更新不存在的集群
func TestUpdateCluster_NotFound(t *testing.T) {
	mockRepo := new(MockClusterRepository)
	mockUserRepo := new(MockClusterUserRepository)

	svc := NewService(
		mockRepo,
		mockUserRepo,
		nil,
	)

	// 设置 mock：集群不存在
	mockRepo.On("FindByID", mock.Anything, int64(999)).Return(nil, errors.New("not found"))

	req := &UpdateClusterRequest{
		ClusterName: "updated-cluster",
	}

	err := svc.UpdateCluster(context.Background(), 999, req)

	assert.Error(t, err)
	mockRepo.AssertExpectations(t)
}

// TestUpdateCluster_WithAuthConfig 测试更新集群时重新加密认证配置
func TestUpdateCluster_WithAuthConfig(t *testing.T) {
	mockRepo := new(MockClusterRepository)
	mockUserRepo := new(MockClusterUserRepository)
	
	// 创建真实的加密服务实例（使用测试密钥）
	// Base64 编码的 32 字节密钥 (32 bytes = 256 bits for AES-256)
	// 这是一个有效的 Base64 编码的 32 字节密钥
	testKey := "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=" // 32 bytes after base64 decode
	mockEnc, err := encryption.NewService(testKey)
	if err != nil {
		t.Fatalf("failed to create encryption service: %v", err)
	}

	svc := NewService(
		mockRepo,
		mockUserRepo,
		mockEnc,
	)

	// 设置 mock：找到现有集群
	mockRepo.On("FindByID", mock.Anything, int64(1)).Return(&models.Cluster{
		ClusterID:         1,
		ClusterName:       "test-cluster",
		BootstrapServers:  "localhost:9092",
		AuthType:          models.AuthTypeSCRAM,
		Status:            models.ClusterStatusActive,
	}, nil)

	// 设置 mock：更新成功
	mockRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

	req := &UpdateClusterRequest{
		AuthConfig: map[string]interface{}{
			"username": "testuser",
			"password": "testpass",
		},
	}

	err = svc.UpdateCluster(context.Background(), 1, req)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// TestDeleteCluster_Success 测试删除集群成功
func TestDeleteCluster_Success(t *testing.T) {
	mockRepo := new(MockClusterRepository)
	mockUserRepo := new(MockClusterUserRepository)

	svc := NewService(
		mockRepo,
		mockUserRepo,
		nil,
	)

	// 设置 mock：删除成功
	mockRepo.On("Delete", mock.Anything, int64(1)).Return(nil)

	err := svc.DeleteCluster(context.Background(), 1)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// TestGetCluster_Success 测试获取集群详情成功
func TestGetCluster_Success(t *testing.T) {
	mockRepo := new(MockClusterRepository)
	mockUserRepo := new(MockClusterUserRepository)
	
	// 创建真实的加密服务实例
	testKey := "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=" // 32 bytes after base64 decode
	mockEnc, err := encryption.NewService(testKey)
	if err != nil {
		t.Fatalf("failed to create encryption service: %v", err)
	}
	
	// 先加密测试数据
	encryptedConfig, err := mockEnc.EncryptString(`{"username":"test"}`)
	if err != nil {
		t.Fatalf("failed to encrypt test data: %v", err)
	}

	svc := NewService(
		mockRepo,
		mockUserRepo,
		mockEnc,
	)

	// 设置 mock：找到集群
	mockRepo.On("FindByID", mock.Anything, int64(1)).Return(&models.Cluster{
		ClusterID:         1,
		ClusterName:       "test-cluster",
		BootstrapServers:  "localhost:9092",
		AuthType:          models.AuthTypeSCRAM,
		AuthConfig:        encryptedConfig,
		Status:            models.ClusterStatusActive,
	}, nil)

	cluster, err := svc.GetCluster(context.Background(), 1)

	assert.NoError(t, err)
	assert.Equal(t, int64(1), cluster.ClusterID)
	assert.Equal(t, "test-cluster", cluster.ClusterName)
	assert.Equal(t, `{"username":"test"}`, cluster.AuthConfig)
	mockRepo.AssertExpectations(t)
}

// TestGetCluster_DecryptFailed 测试获取集群时解密失败
func TestGetCluster_DecryptFailed(t *testing.T) {
	mockRepo := new(MockClusterRepository)
	mockUserRepo := new(MockClusterUserRepository)
	
	// 创建真实的加密服务实例
	testKey := "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=" // 32 bytes after base64 decode
	mockEnc, err := encryption.NewService(testKey)
	if err != nil {
		t.Fatalf("failed to create encryption service: %v", err)
	}

	svc := NewService(
		mockRepo,
		mockUserRepo,
		mockEnc,
	)

	// 设置 mock：找到集群（使用无效的加密配置）
	mockRepo.On("FindByID", mock.Anything, int64(1)).Return(&models.Cluster{
		ClusterID:         1,
		ClusterName:       "test-cluster",
		BootstrapServers:  "localhost:9092",
		AuthType:          models.AuthTypeSCRAM,
		AuthConfig:        "invalid-encrypted-config",
		Status:            models.ClusterStatusActive,
	}, nil)

	cluster, err := svc.GetCluster(context.Background(), 1)

	assert.NoError(t, err)
	assert.Equal(t, "", cluster.AuthConfig) // 解密失败时返回空配置
	mockRepo.AssertExpectations(t)
}

// TestListClusters_Success 测试获取集群列表成功
func TestListClusters_Success(t *testing.T) {
	mockRepo := new(MockClusterRepository)
	mockUserRepo := new(MockClusterUserRepository)

	svc := NewService(
		mockRepo,
		mockUserRepo,
		nil,
	)

	// 设置 mock：返回集群列表
	mockRepo.On("ListByUser", mock.Anything, int64(1), models.UserRole("super_admin"), 0, 20).Return(
		[]*models.Cluster{
			{ClusterID: 1, ClusterName: "cluster-1"},
			{ClusterID: 2, ClusterName: "cluster-2"},
		},
		int64(2),
		nil,
	)

	clusters, total, err := svc.ListClusters(context.Background(), 1, models.UserRole("super_admin"), 0, 20)

	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, clusters, 2)
	mockRepo.AssertExpectations(t)
}

// TestGrantClusterAccess_Success 测试授予集群访问权限成功
func TestGrantClusterAccess_Success(t *testing.T) {
	mockRepo := new(MockClusterRepository)
	mockUserRepo := new(MockClusterUserRepository)

	svc := NewService(
		mockRepo,
		mockUserRepo,
		nil,
	)

	// 设置 mock：创建关联成功
	mockUserRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	err := svc.GrantClusterAccess(context.Background(), 1, 2)

	assert.NoError(t, err)
	mockUserRepo.AssertExpectations(t)
}

// TestRevokeClusterAccess_Success 测试撤销集群访问权限成功
func TestRevokeClusterAccess_Success(t *testing.T) {
	mockRepo := new(MockClusterRepository)
	mockUserRepo := new(MockClusterUserRepository)

	svc := NewService(
		mockRepo,
		mockUserRepo,
		nil,
	)

	// 设置 mock：删除关联成功
	mockUserRepo.On("Delete", mock.Anything, int64(1), int64(2)).Return(nil)

	err := svc.RevokeClusterAccess(context.Background(), 1, 2)

	assert.NoError(t, err)
	mockUserRepo.AssertExpectations(t)
}

// TestListClusterUsers_Success 测试获取集群授权用户列表成功
func TestListClusterUsers_Success(t *testing.T) {
	mockRepo := new(MockClusterRepository)
	mockUserRepo := new(MockClusterUserRepository)

	svc := NewService(
		mockRepo,
		mockUserRepo,
		nil,
	)

	// 设置 mock：返回用户列表
	mockUserRepo.On("ListUsersByCluster", mock.Anything, int64(1)).Return(
		[]*models.User{
			{UserID: 1, Username: "user1"},
			{UserID: 2, Username: "user2"},
		},
		nil,
	)

	users, err := svc.ListClusterUsers(context.Background(), 1)

	assert.NoError(t, err)
	assert.Len(t, users, 2)
	mockUserRepo.AssertExpectations(t)
}

// TestTestConnection_ClusterNotFound 测试连接测试时集群不存在
func TestTestConnection_ClusterNotFound(t *testing.T) {
	mockRepo := new(MockClusterRepository)
	mockUserRepo := new(MockClusterUserRepository)

	svc := NewService(
		mockRepo,
		mockUserRepo,
		nil,
	)

	// 设置 mock：集群不存在
	mockRepo.On("FindByID", mock.Anything, int64(999)).Return(nil, errors.New("not found"))

	err := svc.TestConnection(context.Background(), 999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get cluster")
	mockRepo.AssertExpectations(t)
}

// TestTestConnectionForCreate_Success 测试创建前连接测试成功
func TestTestConnectionForCreate_Success(t *testing.T) {
	// 这个测试需要 mock Kafka 连接，暂时跳过
	// 将在集成测试中覆盖
	t.Skip("需要 mock Kafka 连接，将在集成测试中覆盖")
}