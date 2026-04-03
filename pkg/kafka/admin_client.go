package kafka

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"kafka-management-platform/internal/models"

	"github.com/IBM/sarama"
	"github.com/xdg-go/scram"
)

var (
	// ErrInvalidAuthConfig 无效的认证配置
	ErrInvalidAuthConfig = errors.New("invalid auth config")
	// ErrUnsupportedAuthType 不支持的认证类型
	ErrUnsupportedAuthType = errors.New("unsupported auth type")
)

// AdminClient Kafka Admin 客户端封装
type AdminClient struct {
	admin sarama.ClusterAdmin
}

// NewAdminClient 创建 Kafka Admin 客户端
// 根据集群配置创建支持不同认证方式的客户端
func NewAdminClient(cluster *models.Cluster, authConfigJSON string) (*AdminClient, error) {
	config := sarama.NewConfig()
	config.Version = sarama.V2_8_0_0
	
	// 配置认证方式
	if err := configureAuth(config, cluster.AuthType, authConfigJSON); err != nil {
		return nil, fmt.Errorf("failed to configure auth: %w", err)
	}
	
	// 解析 Bootstrap Servers
	brokers := strings.Split(cluster.BootstrapServers, ",")
	for i, broker := range brokers {
		brokers[i] = strings.TrimSpace(broker)
	}
	
	// 创建 Admin 客户端
	admin, err := sarama.NewClusterAdmin(brokers, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster admin: %w", err)
	}
	
	return &AdminClient{admin: admin}, nil
}

// TestConnection 测试 Kafka 集群连接
// 通过获取集群元数据来验证连接是否正常
func (c *AdminClient) TestConnection() error {
	// 尝试获取集群元数据
	_, _, err := c.admin.DescribeCluster()
	if err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}
	return nil
}

// ListTopics 列出所有 Topic
func (c *AdminClient) ListTopics() (map[string]sarama.TopicDetail, error) {
	return c.admin.ListTopics()
}

// CreateTopic 创建 Topic
func (c *AdminClient) CreateTopic(name string, detail *sarama.TopicDetail, validateOnly bool) error {
	return c.admin.CreateTopic(name, detail, validateOnly)
}

// DeleteTopic 删除 Topic
func (c *AdminClient) DeleteTopic(name string) error {
	return c.admin.DeleteTopic(name)
}

// CreateACL 创建 ACL 规则
func (c *AdminClient) CreateACL(resource sarama.Resource, acl sarama.Acl) error {
	return c.admin.CreateACL(resource, acl)
}

// DeleteACL 删除 ACL 规则
func (c *AdminClient) DeleteACL(filter sarama.AclFilter, validateOnly bool) ([]sarama.MatchingAcl, error) {
	return c.admin.DeleteACL(filter, validateOnly)
}

// ListACLs 列出 ACL 规则
func (c *AdminClient) ListACLs(filter sarama.AclFilter) ([]sarama.ResourceAcls, error) {
	return c.admin.ListAcls(filter)
}

// Close 关闭客户端连接
func (c *AdminClient) Close() error {
	return c.admin.Close()
}

// configureAuth 配置 Kafka 认证方式
func configureAuth(config *sarama.Config, authType models.AuthType, authConfigJSON string) error {
	switch authType {
	case models.AuthTypePlaintext:
		// PLAINTEXT 无需额外配置
		return nil
		
	case models.AuthTypeSCRAM:
		return configureSCRAM(config, authConfigJSON)
		
	case models.AuthTypeKerberos:
		return configureKerberos(config, authConfigJSON)
		
	default:
		return ErrUnsupportedAuthType
	}
}

// configureSCRAM 配置 SCRAM 认证
func configureSCRAM(config *sarama.Config, authConfigJSON string) error {
	if authConfigJSON == "" {
		return ErrInvalidAuthConfig
	}
	
	var authConfig map[string]interface{}
	if err := json.Unmarshal([]byte(authConfigJSON), &authConfig); err != nil {
		return fmt.Errorf("failed to parse auth config: %w", err)
	}
	
	username, ok := authConfig["username"].(string)
	if !ok || username == "" {
		return fmt.Errorf("%w: missing username", ErrInvalidAuthConfig)
	}
	
	password, ok := authConfig["password"].(string)
	if !ok || password == "" {
		return fmt.Errorf("%w: missing password", ErrInvalidAuthConfig)
	}
	
	mechanism := "SCRAM-SHA-256"
	if m, ok := authConfig["mechanism"].(string); ok && m != "" {
		mechanism = m
	}
	
	config.Net.SASL.Enable = true
	config.Net.SASL.User = username
	config.Net.SASL.Password = password
	
	switch mechanism {
	case "SCRAM-SHA-256":
		config.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
		config.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
			return &XDGSCRAMClient{HashGeneratorFcn: scram.SHA256}
		}
	case "SCRAM-SHA-512":
		config.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
		config.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
			return &XDGSCRAMClient{HashGeneratorFcn: scram.SHA512}
		}
	default:
		return fmt.Errorf("%w: unsupported SCRAM mechanism: %s", ErrInvalidAuthConfig, mechanism)
	}
	
	// 启用 TLS（SCRAM 通常需要 TLS）
	config.Net.TLS.Enable = true
	config.Net.TLS.Config = &tls.Config{
		InsecureSkipVerify: false,
	}
	
	return nil
}

// configureKerberos 配置 Kerberos 认证
func configureKerberos(config *sarama.Config, authConfigJSON string) error {
	if authConfigJSON == "" {
		return ErrInvalidAuthConfig
	}
	
	var authConfig map[string]interface{}
	if err := json.Unmarshal([]byte(authConfigJSON), &authConfig); err != nil {
		return fmt.Errorf("failed to parse auth config: %w", err)
	}
	
	principal, ok := authConfig["principal"].(string)
	if !ok || principal == "" {
		return fmt.Errorf("%w: missing principal", ErrInvalidAuthConfig)
	}
	
	keytab, ok := authConfig["keytab"].(string)
	if !ok || keytab == "" {
		return fmt.Errorf("%w: missing keytab", ErrInvalidAuthConfig)
	}
	
	realm, ok := authConfig["realm"].(string)
	if !ok || realm == "" {
		return fmt.Errorf("%w: missing realm", ErrInvalidAuthConfig)
	}
	
	serviceName := "kafka"
	if sn, ok := authConfig["service_name"].(string); ok && sn != "" {
		serviceName = sn
	}
	
	config.Net.SASL.Enable = true
	config.Net.SASL.Mechanism = sarama.SASLTypeGSSAPI
	config.Net.SASL.GSSAPI.ServiceName = serviceName
	config.Net.SASL.GSSAPI.KerberosConfigPath = "/etc/krb5.conf"
	config.Net.SASL.GSSAPI.Realm = realm
	config.Net.SASL.GSSAPI.Username = principal
	config.Net.SASL.GSSAPI.AuthType = sarama.KRB5_KEYTAB_AUTH
	config.Net.SASL.GSSAPI.KeyTabPath = keytab
	
	return nil
}

// SCRAM 客户端实现
// Sarama 需要自定义 SCRAM 客户端来支持 SCRAM-SHA-256 和 SCRAM-SHA-512

// XDGSCRAMClient SCRAM 客户端实现
type XDGSCRAMClient struct {
	*scram.Client
	*scram.ClientConversation
	scram.HashGeneratorFcn
}

// Begin 开始 SCRAM 认证
func (x *XDGSCRAMClient) Begin(userName, password, authzID string) (err error) {
	x.Client, err = x.HashGeneratorFcn.NewClient(userName, password, authzID)
	if err != nil {
		return err
	}
	x.ClientConversation = x.Client.NewConversation()
	return nil
}

// Step 执行 SCRAM 认证步骤
func (x *XDGSCRAMClient) Step(challenge string) (response string, err error) {
	return x.ClientConversation.Step(challenge)
}

// Done 完成 SCRAM 认证
func (x *XDGSCRAMClient) Done() bool {
	return x.ClientConversation.Done()
}
