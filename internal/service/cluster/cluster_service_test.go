package cluster

import (
	"context"
	"testing"

	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/repository"
	"kafka-management-platform/pkg/encryption"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockClusterRepository 模拟集群仓库
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

func (m *MockClusterRepository) Delete(ctx context.Context, clusterID int64) error {
	args := m.Called(ctx, clusterID)
	return args.Error(0)
}

func (m *MockClusterRepository) FindByID(ctx context.Context, clusterID int64) (*models.Cluster, error) {
	args := m.Called(ctx, clusterID)
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

// MockClusterUserRepository 模拟集群用户关联仓库
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
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.User), args.Error(1)
}

func (m *MockClusterUserRepository) ListClustersByUser(ctx context.Context, userID int64) ([]*models.Cluster, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
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

// TestTestConnection_ClusterNotFound 测试集群不存在的情况
func TestTestConnection_ClusterNotFound(t *testing.T) {
	mockClusterRepo := new(MockClusterRepository)
	mockClusterUserRepo := new(MockClusterUserRepository)
	encryptionSvc, _ := encryption.NewService("12345678901234567890123456789012")
	
	service := NewService(mockClusterRepo, mockClusterUserRepo, encryptionSvc)
	
	ctx := context.Background()
	clusterID := int64(999)
	
	// 模拟集群不存在
	mockClusterRepo.On("FindByID", ctx, clusterID).Return(nil, repository.ErrClusterNotFound)
	
	err := service.TestConnection(ctx, clusterID)
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get cluster")
	mockClusterRepo.AssertExpectations(t)
}

// TestTestConnection_InvalidBootstrapServers 测试无效的 Bootstrap Servers
// 注意：此测试需要实际尝试连接 Kafka，会比较慢
func TestTestConnection_InvalidBootstrapServers(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要网络连接的测试")
	}
	
	mockClusterRepo := new(MockClusterRepository)
	mockClusterUserRepo := new(MockClusterUserRepository)
	encryptionSvc, _ := encryption.NewService("12345678901234567890123456789012")
	
	service := NewService(mockClusterRepo, mockClusterUserRepo, encryptionSvc)
	
	ctx := context.Background()
	clusterID := int64(1)
	
	cluster := &models.Cluster{
		ClusterID:        clusterID,
		ClusterName:      "test-cluster",
		BootstrapServers: "invalid-host:9092",
		AuthType:         models.AuthTypePlaintext,
	}
	
	mockClusterRepo.On("FindByID", ctx, clusterID).Return(cluster, nil)
	
	err := service.TestConnection(ctx, clusterID)
	
	// 连接测试应该失败（因为没有实际的 Kafka 集群）
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection test failed")
	mockClusterRepo.AssertExpectations(t)
}
