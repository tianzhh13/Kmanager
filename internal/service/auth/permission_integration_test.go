package auth

import (
	"context"
	"testing"

	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockUserRepository 用户仓库 Mock
type MockUserRepository struct {
	users map[int64]*models.User
}

func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		users: make(map[int64]*models.User),
	}
}

func (m *MockUserRepository) Create(ctx context.Context, user *models.User) error {
	m.users[user.UserID] = user
	return nil
}

func (m *MockUserRepository) Update(ctx context.Context, user *models.User) error {
	m.users[user.UserID] = user
	return nil
}

func (m *MockUserRepository) Delete(ctx context.Context, userID int64) error {
	delete(m.users, userID)
	return nil
}

func (m *MockUserRepository) FindByID(ctx context.Context, userID int64) (*models.User, error) {
	user, ok := m.users[userID]
	if !ok {
		return nil, repository.ErrUserNotFound
	}
	return user, nil
}

func (m *MockUserRepository) FindByUsername(ctx context.Context, username string) (*models.User, error) {
	for _, user := range m.users {
		if user.Username == username {
			return user, nil
		}
	}
	return nil, repository.ErrUserNotFound
}

func (m *MockUserRepository) List(ctx context.Context, offset, limit int) ([]*models.User, int64, error) {
	var result []*models.User
	count := int64(0)
	for _, user := range m.users {
		count++
		if len(result) < limit {
			result = append(result, user)
		}
	}
	return result, count, nil
}

func (m *MockUserRepository) Search(ctx context.Context, keyword string, offset, limit int) ([]*models.User, int64, error) {
	return m.List(ctx, offset, limit)
}

// MockClusterUserRepository 集群用户关联仓库 Mock
type MockClusterUserRepository struct {
	relations []models.ClusterUserRelation
}

func NewMockClusterUserRepository() *MockClusterUserRepository {
	return &MockClusterUserRepository{
		relations: make([]models.ClusterUserRelation, 0),
	}
}

func (m *MockClusterUserRepository) Create(ctx context.Context, relation *models.ClusterUserRelation) error {
	m.relations = append(m.relations, *relation)
	return nil
}

func (m *MockClusterUserRepository) Delete(ctx context.Context, clusterID, userID int64) error {
	newRelations := make([]models.ClusterUserRelation, 0)
	for _, r := range m.relations {
		if !(r.ClusterID == clusterID && r.UserID == userID) {
			newRelations = append(newRelations, r)
		}
	}
	m.relations = newRelations
	return nil
}

func (m *MockClusterUserRepository) HasAccess(ctx context.Context, clusterID, userID int64) (bool, error) {
	for _, r := range m.relations {
		if r.ClusterID == clusterID && r.UserID == userID {
			return true, nil
		}
	}
	return false, nil
}

func (m *MockClusterUserRepository) ListUsersByCluster(ctx context.Context, clusterID int64) ([]*models.User, error) {
	return nil, nil
}

func (m *MockClusterUserRepository) ListClustersByUser(ctx context.Context, userID int64) ([]*models.Cluster, error) {
	return nil, nil
}

func (m *MockClusterUserRepository) DeleteByCluster(ctx context.Context, clusterID int64) error {
	newRelations := make([]models.ClusterUserRelation, 0)
	for _, r := range m.relations {
		if r.ClusterID != clusterID {
			newRelations = append(newRelations, r)
		}
	}
	m.relations = newRelations
	return nil
}

func (m *MockClusterUserRepository) DeleteByUser(ctx context.Context, userID int64) error {
	newRelations := make([]models.ClusterUserRelation, 0)
	for _, r := range m.relations {
		if r.UserID != userID {
			newRelations = append(newRelations, r)
		}
	}
	m.relations = newRelations
	return nil
}

// testSetup 测试所需的依赖
type testSetup struct {
	userRepo          *MockUserRepository
	clusterUserRepo   *MockClusterUserRepository
	permissionService *PermissionService
}

func setupTest(t *testing.T) *testSetup {
	userRepo := NewMockUserRepository()
	clusterUserRepo := NewMockClusterUserRepository()

	permissionService := NewPermissionService(userRepo, clusterUserRepo)

	return &testSetup{
		userRepo:          userRepo,
		clusterUserRepo:   clusterUserRepo,
		permissionService: permissionService,
	}
}

// 创建测试用户
func createTestUser(t *testing.T, setup *testSetup, username string, role models.UserRole, status models.UserStatus) *models.User {
	user := &models.User{
		UserID:       int64(len(setup.userRepo.users) + 1),
		Username:     username,
		PasswordHash: "$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/LewY5GyYzvz3Jf3Gi",
		Email:        username + "@example.com",
		Role:         role,
		Status:       status,
	}
	err := setup.userRepo.Create(context.Background(), user)
	require.NoError(t, err)
	return user
}

// 授权用户管理集群
func grantClusterAccess(t *testing.T, setup *testSetup, clusterID, userID int64) {
	relation := &models.ClusterUserRelation{
		ClusterID: clusterID,
		UserID:    userID,
	}
	err := setup.clusterUserRepo.Create(context.Background(), relation)
	require.NoError(t, err)
}

// TestSuperAdminHasAllPermissions 测试超级管理员拥有所有权限
// 验证需求: 2.1 - 当用户角色为 Super_Admin 时，系统应允许该用户执行所有操作
func TestSuperAdminHasAllPermissions(t *testing.T) {
	setup := setupTest(t)
	ctx := context.Background()

	// 创建超级管理员用户
	superAdmin := createTestUser(t, setup, "superadmin", models.RoleSuperAdmin, models.UserStatusActive)

	// 测试各种操作的权限
	testCases := []struct {
		name     string
		resource string
		action   string
		expected bool
	}{
		{"创建集群", "cluster", "create", true},
		{"更新集群", "cluster", "update", true},
		{"删除集群", "cluster", "delete", true},
		{"创建Topic", "topic", "create", true},
		{"删除Topic", "topic", "delete", true},
		{"更新Topic", "topic", "update", true},
		{"查询Topic", "topic", "list", true},
		{"创建ACL", "acl", "create", true},
		{"删除ACL", "acl", "delete", true},
		{"查询ACL", "acl", "list", true},
		{"用户管理", "user", "create", true},
		{"删除用户", "user", "delete", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hasPermission, err := setup.permissionService.CheckPermission(ctx, superAdmin.UserID, tc.resource, tc.action)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, hasPermission, "超级管理员应该拥有 %s 权限", tc.name)
		})
	}
}

// TestSuperAdminHasAllClusterPermissions 测试超级管理员拥有所有集群权限
// 验证需求: 2.1 - 超级管理员可以管理所有集群
func TestSuperAdminHasAllClusterPermissions(t *testing.T) {
	setup := setupTest(t)
	ctx := context.Background()

	// 创建超级管理员用户
	superAdmin := createTestUser(t, setup, "superadmin", models.RoleSuperAdmin, models.UserStatusActive)

	// 超级管理员应该对所有集群都有权限（包括不存在的集群）
	testCases := []struct {
		name      string
		clusterID int64
		expected  bool
	}{
		{"集群1", 1, true},
		{"集群2", 2, true},
		{"集群3", 3, true},
		{"不存在的集群", 99999, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hasPermission, err := setup.permissionService.CheckClusterPermission(ctx, superAdmin.UserID, tc.clusterID)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, hasPermission, "超级管理员应该对 %s 有权限", tc.name)
		})
	}
}

// TestClusterAdminPermissionsForAuthorizedCluster 测试集群管理员对授权集群的权限
// 验证需求: 2.3 - 当用户角色为 Cluster_Admin 且该用户被授权管理指定集群时，系统应允许该用户管理该集群的资源
func TestClusterAdminPermissionsForAuthorizedCluster(t *testing.T) {
	setup := setupTest(t)
	ctx := context.Background()

	// 创建集群管理员用户
	clusterAdmin := createTestUser(t, setup, "clusteradmin", models.RoleClusterAdmin, models.UserStatusActive)

	// 授权 clusterAdmin 管理 cluster1 (clusterID=1)
	grantClusterAccess(t, setup, 1, clusterAdmin.UserID)

	// 测试对授权集群的权限
	hasPermission, err := setup.permissionService.CheckClusterPermission(ctx, clusterAdmin.UserID, 1)
	assert.NoError(t, err)
	assert.True(t, hasPermission, "集群管理员对授权的集群应该有管理权限")

	// 测试对未授权集群的权限
	hasPermission, err = setup.permissionService.CheckClusterPermission(ctx, clusterAdmin.UserID, 2)
	assert.NoError(t, err)
	assert.False(t, hasPermission, "集群管理员对未授权的集群应该没有管理权限")
}

// TestClusterAdminPermissionsIsolation 测试集群管理员权限隔离
// 验证需求: 2.4 - 当用户角色为 Cluster_Admin 且该用户未被授权管理指定集群时，系统应拒绝该用户管理该集群的资源
func TestClusterAdminPermissionsIsolation(t *testing.T) {
	setup := setupTest(t)
	ctx := context.Background()

	// 创建两个集群管理员
	clusterAdmin1 := createTestUser(t, setup, "clusteradmin1", models.RoleClusterAdmin, models.UserStatusActive)
	clusterAdmin2 := createTestUser(t, setup, "clusteradmin2", models.RoleClusterAdmin, models.UserStatusActive)

	// 授权 clusterAdmin1 管理 cluster1
	grantClusterAccess(t, setup, 1, clusterAdmin1.UserID)

	// 授权 clusterAdmin2 管理 cluster2
	grantClusterAccess(t, setup, 2, clusterAdmin2.UserID)

	// 测试权限隔离
	testCases := []struct {
		name        string
		user        *models.User
		clusterID   int64
		expectOwner bool
	}{
		{
			name:        "clusterAdmin1 对 cluster1",
			user:        clusterAdmin1,
			clusterID:   1,
			expectOwner: true,
		},
		{
			name:        "clusterAdmin1 对 cluster2",
			user:        clusterAdmin1,
			clusterID:   2,
			expectOwner: false,
		},
		{
			name:        "clusterAdmin2 对 cluster1",
			user:        clusterAdmin2,
			clusterID:   1,
			expectOwner: false,
		},
		{
			name:        "clusterAdmin2 对 cluster2",
			user:        clusterAdmin2,
			clusterID:   2,
			expectOwner: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hasPermission, err := setup.permissionService.CheckClusterPermission(ctx, tc.user.UserID, tc.clusterID)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectOwner, hasPermission, "权限隔离验证失败: %s", tc.name)
		})
	}
}

// TestReadOnlyUserPermissions 测试只读用户权限限制
// 验证需求: 2.2 - 当用户角色为 Read_Only 时，系统应仅允许该用户执行查询操作
func TestReadOnlyUserPermissions(t *testing.T) {
	setup := setupTest(t)
	ctx := context.Background()

	// 创建只读用户
	readOnlyUser := createTestUser(t, setup, "readonly", models.RoleReadOnly, models.UserStatusActive)

	// 测试只读权限
	readOperations := []struct {
		name     string
		resource string
		action   string
	}{
		{"查询集群", "cluster", "list"},
		{"获取集群详情", "cluster", "get"},
		{"查询Topic", "topic", "list"},
		{"获取Topic详情", "topic", "get"},
		{"查询ACL", "acl", "list"},
		{"获取ACL详情", "acl", "get"},
		{"查询监控", "monitor", "query"},
		{"查询审计日志", "audit", "query"},
	}

	for _, op := range readOperations {
		t.Run("读操作-"+op.name, func(t *testing.T) {
			hasPermission, err := setup.permissionService.CheckPermission(ctx, readOnlyUser.UserID, op.resource, op.action)
			assert.NoError(t, err)
			assert.True(t, hasPermission, "只读用户应该能够执行读操作: %s", op.name)
		})
	}

	// 测试写操作权限限制
	writeOperations := []struct {
		name     string
		resource string
		action   string
	}{
		{"创建集群", "cluster", "create"},
		{"更新集群", "cluster", "update"},
		{"删除集群", "cluster", "delete"},
		{"创建Topic", "topic", "create"},
		{"删除Topic", "topic", "delete"},
		{"更新Topic", "topic", "update"},
		{"创建ACL", "acl", "create"},
		{"删除ACL", "acl", "delete"},
		{"创建用户", "user", "create"},
		{"删除用户", "user", "delete"},
	}

	for _, op := range writeOperations {
		t.Run("写操作-"+op.name, func(t *testing.T) {
			hasPermission, err := setup.permissionService.CheckPermission(ctx, readOnlyUser.UserID, op.resource, op.action)
			assert.NoError(t, err)
			assert.False(t, hasPermission, "只读用户不应该能够执行写操作: %s", op.name)
		})
	}
}

// TestReadOnlyUserNoClusterManagement 测试只读用户无集群管理权限
// 验证需求: 2.2 - 只读用户不能管理任何集群
func TestReadOnlyUserNoClusterManagement(t *testing.T) {
	setup := setupTest(t)
	ctx := context.Background()

	// 创建只读用户
	readOnlyUser := createTestUser(t, setup, "readonly", models.RoleReadOnly, models.UserStatusActive)

	// 授权只读用户访问集群（只读访问）
	grantClusterAccess(t, setup, 1, readOnlyUser.UserID)

	// 只读用户对集群没有管理权限
	hasPermission, err := setup.permissionService.CheckClusterPermission(ctx, readOnlyUser.UserID, 1)
	assert.NoError(t, err)
	assert.False(t, hasPermission, "只读用户不应该有集群管理权限")

	// 但有读权限
	hasReadPermission, err := setup.permissionService.CheckClusterReadPermission(ctx, readOnlyUser.UserID, 1)
	assert.NoError(t, err)
	assert.True(t, hasReadPermission, "只读用户应该有集群读权限")
}

// TestReadOnlyUserCannotManageAnyCluster 测试只读用户不能管理任何集群
// 验证需求: 2.2 - 只读用户即使被授权访问集群，也没有管理权限
func TestReadOnlyUserCannotManageAnyCluster(t *testing.T) {
	setup := setupTest(t)
	ctx := context.Background()

	// 创建只读用户
	readOnlyUser := createTestUser(t, setup, "readonly", models.RoleReadOnly, models.UserStatusActive)

	// 授权只读用户访问所有集群
	grantClusterAccess(t, setup, 1, readOnlyUser.UserID)
	grantClusterAccess(t, setup, 2, readOnlyUser.UserID)
	grantClusterAccess(t, setup, 3, readOnlyUser.UserID)

	// 只读用户对所有集群都没有管理权限
	for _, clusterID := range []int64{1, 2, 3} {
		hasPermission, err := setup.permissionService.CheckClusterPermission(ctx, readOnlyUser.UserID, clusterID)
		assert.NoError(t, err)
		assert.False(t, hasPermission, "只读用户不应该有集群 %d 的管理权限", clusterID)
	}
}

// TestPermissionDeniedForUnauthorizedAction 测试未授权操作被拒绝
// 验证需求: 2.5 - 当用户尝试执行超出其权限的操作时，系统应返回权限拒绝错误
func TestPermissionDeniedForUnauthorizedAction(t *testing.T) {
	setup := setupTest(t)
	ctx := context.Background()

	// 创建集群管理员（未授权任何集群）
	clusterAdmin := createTestUser(t, setup, "clusteradmin", models.RoleClusterAdmin, models.UserStatusActive)

	// 集群管理员未授权管理任何集群
	hasPermission, err := setup.permissionService.CheckClusterPermission(ctx, clusterAdmin.UserID, 1)
	assert.NoError(t, err)
	assert.False(t, hasPermission, "未授权的集群管理员不应该有集群管理权限")

	// 验证 CheckClusterReadPermission 也返回 false
	hasReadPermission, err := setup.permissionService.CheckClusterReadPermission(ctx, clusterAdmin.UserID, 1)
	assert.NoError(t, err)
	assert.False(t, hasReadPermission, "未授权的集群管理员不应该有集群读权限")
}

// TestRoleCheckFunctions 测试角色检查函数
// 验证需求: 2.1, 2.2, 2.3 - 验证角色检查函数正确性
func TestRoleCheckFunctions(t *testing.T) {
	setup := setupTest(t)
	ctx := context.Background()

	// 创建不同角色的用户
	superAdmin := createTestUser(t, setup, "superadmin", models.RoleSuperAdmin, models.UserStatusActive)
	clusterAdmin := createTestUser(t, setup, "clusteradmin", models.RoleClusterAdmin, models.UserStatusActive)
	readOnlyUser := createTestUser(t, setup, "readonly", models.RoleReadOnly, models.UserStatusActive)

	// 测试 IsSuperAdmin
	isSuperAdmin, err := setup.permissionService.IsSuperAdmin(ctx, superAdmin.UserID)
	assert.NoError(t, err)
	assert.True(t, isSuperAdmin, "超级管理员应该被识别")

	isSuperAdmin, err = setup.permissionService.IsSuperAdmin(ctx, clusterAdmin.UserID)
	assert.NoError(t, err)
	assert.False(t, isSuperAdmin, "集群管理员不应该被识别为超级管理员")

	// 测试 IsClusterAdmin
	isClusterAdmin, err := setup.permissionService.IsClusterAdmin(ctx, clusterAdmin.UserID)
	assert.NoError(t, err)
	assert.True(t, isClusterAdmin, "集群管理员应该被识别")

	isClusterAdmin, err = setup.permissionService.IsClusterAdmin(ctx, readOnlyUser.UserID)
	assert.NoError(t, err)
	assert.False(t, isClusterAdmin, "只读用户不应该被识别为集群管理员")

	// 测试 IsReadOnly
	isReadOnly, err := setup.permissionService.IsReadOnly(ctx, readOnlyUser.UserID)
	assert.NoError(t, err)
	assert.True(t, isReadOnly, "只读用户应该被识别")

	isReadOnly, err = setup.permissionService.IsReadOnly(ctx, superAdmin.UserID)
	assert.NoError(t, err)
	assert.False(t, isReadOnly, "超级管理员不应该被识别为只读用户")
}

// TestClusterAdminWithMultipleClusters 测试集群管理员管理多个集群
// 验证需求: 2.3 - 集群管理员可以被授权管理多个集群
func TestClusterAdminWithMultipleClusters(t *testing.T) {
	setup := setupTest(t)
	ctx := context.Background()

	// 创建集群管理员
	clusterAdmin := createTestUser(t, setup, "clusteradmin", models.RoleClusterAdmin, models.UserStatusActive)

	// 授权 clusterAdmin 管理前 3 个集群
	grantClusterAccess(t, setup, 1, clusterAdmin.UserID)
	grantClusterAccess(t, setup, 2, clusterAdmin.UserID)
	grantClusterAccess(t, setup, 3, clusterAdmin.UserID)

	// 验证权限
	for _, clusterID := range []int64{1, 2, 3, 4, 5} {
		hasPermission, err := setup.permissionService.CheckClusterPermission(ctx, clusterAdmin.UserID, clusterID)
		assert.NoError(t, err)
		if clusterID <= 3 {
			assert.True(t, hasPermission, "集群管理员应该有集群 %d 的管理权限", clusterID)
		} else {
			assert.False(t, hasPermission, "集群管理员不应该有集群 %d 的管理权限", clusterID)
		}
	}
}

// TestPermissionCheckWithInvalidUser 测试无效用户的权限检查
func TestPermissionCheckWithInvalidUser(t *testing.T) {
	setup := setupTest(t)
	ctx := context.Background()

	// 测试不存在的用户
	_, err := setup.permissionService.CheckPermission(ctx, 99999, "cluster", "list")
	assert.Error(t, err, "不存在的用户应该返回错误")

	_, err = setup.permissionService.CheckClusterPermission(ctx, 99999, 1)
	assert.Error(t, err, "不存在的用户应该返回错误")
}

// TestInactiveUserPermissions 测试非活跃用户权限
// 验证需求: 1.3 - 用户账户状态为非活跃时，应该被拒绝
func TestInactiveUserPermissions(t *testing.T) {
	setup := setupTest(t)
	ctx := context.Background()

	// 创建超级管理员但状态为非活跃
	inactiveSuperAdmin := createTestUser(t, setup, "inactivesuperadmin", models.RoleSuperAdmin, models.UserStatusInactive)

	// 非活跃用户查询应该返回错误（因为用户状态检查在认证服务中处理）
	// 权限服务本身不检查用户状态，只检查角色
	// 这里测试权限服务对非活跃用户的行为
	isSuperAdmin, err := setup.permissionService.IsSuperAdmin(ctx, inactiveSuperAdmin.UserID)
	assert.NoError(t, err)
	assert.True(t, isSuperAdmin, "非活跃的超级管理员角色仍然应该被识别")

	// 注意：实际的登录拒绝在 auth_service.go 的 Login 方法中处理
}

// TestClusterPermissionReadWrite 测试集群读写权限分离
// 验证需求: 2.2, 2.3 - 集群管理员和只读用户对集群的读写权限应该正确分离
func TestClusterPermissionReadWrite(t *testing.T) {
	setup := setupTest(t)
	ctx := context.Background()

	// 创建集群管理员
	clusterAdmin := createTestUser(t, setup, "clusteradmin", models.RoleClusterAdmin, models.UserStatusActive)

	// 创建只读用户
	readOnlyUser := createTestUser(t, setup, "readonly", models.RoleReadOnly, models.UserStatusActive)

	// 授权 clusterAdmin 管理 cluster
	grantClusterAccess(t, setup, 1, clusterAdmin.UserID)

	// 授权 readOnlyUser 访问 cluster（只读）
	grantClusterAccess(t, setup, 1, readOnlyUser.UserID)

	// 集群管理员有管理权限（读+写）
	hasMgmtPermission, err := setup.permissionService.CheckClusterPermission(ctx, clusterAdmin.UserID, 1)
	assert.NoError(t, err)
	assert.True(t, hasMgmtPermission, "集群管理员应该有集群管理权限")

	// 集群管理员有读权限
	hasReadPermission, err := setup.permissionService.CheckClusterReadPermission(ctx, clusterAdmin.UserID, 1)
	assert.NoError(t, err)
	assert.True(t, hasReadPermission, "集群管理员应该有集群读权限")

	// 只读用户没有管理权限
	hasMgmtPermission, err = setup.permissionService.CheckClusterPermission(ctx, readOnlyUser.UserID, 1)
	assert.NoError(t, err)
	assert.False(t, hasMgmtPermission, "只读用户不应该有集群管理权限")

	// 只读用户有读权限
	hasReadPermission, err = setup.permissionService.CheckClusterReadPermission(ctx, readOnlyUser.UserID, 1)
	assert.NoError(t, err)
	assert.True(t, hasReadPermission, "只读用户应该有集群读权限")
}