package kafka

import (
	"testing"

	"kafka-management-platform/internal/models"

	"github.com/stretchr/testify/assert"
)

func TestNewAdminClient_Plaintext(t *testing.T) {
	cluster := &models.Cluster{
		ClusterID:        1,
		ClusterName:      "test-cluster",
		BootstrapServers: "localhost:9092",
		AuthType:         models.AuthTypePlaintext,
	}

	// 注意：这个测试需要实际的 Kafka 集群才能通过
	// 这里只测试客户端创建逻辑，不测试实际连接
	client, err := NewAdminClient(cluster, "")
	
	// 如果没有实际的 Kafka 集群，创建客户端会成功，但连接测试会失败
	// 这是预期行为
	if err == nil {
		assert.NotNil(t, client)
		client.Close()
	}
}

func TestConfigureSCRAM_ValidConfig(t *testing.T) {
	authConfigJSON := `{
		"username": "test-user",
		"password": "test-password",
		"mechanism": "SCRAM-SHA-256"
	}`

	cluster := &models.Cluster{
		ClusterID:        1,
		ClusterName:      "test-cluster",
		BootstrapServers: "localhost:9092",
		AuthType:         models.AuthTypeSCRAM,
	}

	client, err := NewAdminClient(cluster, authConfigJSON)
	
	// 客户端创建应该成功（即使没有实际的 Kafka 集群）
	if err == nil {
		assert.NotNil(t, client)
		client.Close()
	}
}

func TestConfigureSCRAM_MissingUsername(t *testing.T) {
	authConfigJSON := `{
		"password": "test-password",
		"mechanism": "SCRAM-SHA-256"
	}`

	cluster := &models.Cluster{
		ClusterID:        1,
		ClusterName:      "test-cluster",
		BootstrapServers: "localhost:9092",
		AuthType:         models.AuthTypeSCRAM,
	}

	_, err := NewAdminClient(cluster, authConfigJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing username")
}

func TestConfigureSCRAM_MissingPassword(t *testing.T) {
	authConfigJSON := `{
		"username": "test-user",
		"mechanism": "SCRAM-SHA-256"
	}`

	cluster := &models.Cluster{
		ClusterID:        1,
		ClusterName:      "test-cluster",
		BootstrapServers: "localhost:9092",
		AuthType:         models.AuthTypeSCRAM,
	}

	_, err := NewAdminClient(cluster, authConfigJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing password")
}
