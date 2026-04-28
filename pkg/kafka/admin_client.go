package kafka

import (
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

// CreateUser 创建 SCRAM 用户
func (c *AdminClient) CreateUser(username, password string, mechanism string) error {
	var iterations int32 = 8192
	var salt []byte = nil // 让 Kafka 自动生成 salt

	var scramMechanism sarama.ScramMechanismType
	switch mechanism {
	case "SCRAM-SHA-256":
		scramMechanism = sarama.SCRAM_MECHANISM_SHA_256
	case "SCRAM-SHA-512":
		scramMechanism = sarama.SCRAM_MECHANISM_SHA_512
	default:
		scramMechanism = sarama.SCRAM_MECHANISM_SHA_256
	}

	upsert := []sarama.AlterUserScramCredentialsUpsert{
		{
			Name:       username,
			Iterations: iterations,
			Salt:       salt,
			Password:   []byte(password),
			Mechanism:  scramMechanism,
		},
	}

	results, err := c.admin.UpsertUserScramCredentials(upsert)
	if err != nil {
		return err
	}

	if len(results) > 0 && results[0].ErrorCode != 0 {
		errMsg := "unknown error"
		if results[0].ErrorMessage != nil {
			errMsg = *results[0].ErrorMessage
		}
		return fmt.Errorf("failed to create user: %s", errMsg)
	}

	return nil
}

// DeleteUser 删除 SCRAM 用户
func (c *AdminClient) DeleteUser(username string, mechanism string) error {
	var scramMechanism sarama.ScramMechanismType
	switch mechanism {
	case "SCRAM-SHA-256":
		scramMechanism = sarama.SCRAM_MECHANISM_SHA_256
	case "SCRAM-SHA-512":
		scramMechanism = sarama.SCRAM_MECHANISM_SHA_512
	default:
		scramMechanism = sarama.SCRAM_MECHANISM_SHA_256
	}

	deletions := []sarama.AlterUserScramCredentialsDelete{
		{
			Name:      username,
			Mechanism: scramMechanism,
		},
	}

	results, err := c.admin.DeleteUserScramCredentials(deletions)
	if err != nil {
		return err
	}

	if len(results) > 0 && results[0].ErrorCode != 0 {
		errMsg := "unknown error"
		if results[0].ErrorMessage != nil {
			errMsg = *results[0].ErrorMessage
		}
		return fmt.Errorf("failed to delete user: %s", errMsg)
	}

	return nil
}

// ListUsers 列出所有 SCRAM 用户
func (c *AdminClient) ListUsers() ([]*sarama.DescribeUserScramCredentialsResult, error) {
	// 传入空数组表示列出所有用户
	results, err := c.admin.DescribeUserScramCredentials(nil)
	if err != nil {
		return nil, err
	}

	return results, nil
}

// DescribeUser 获取用户详情
func (c *AdminClient) DescribeUser(username string) (*sarama.DescribeUserScramCredentialsResult, error) {
	results, err := c.admin.DescribeUserScramCredentials([]string{username})
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("user not found: %s", username)
	}

	if results[0].ErrorCode != 0 {
		errMsg := "unknown error"
		if results[0].ErrorMessage != nil {
			errMsg = *results[0].ErrorMessage
		}
		return nil, fmt.Errorf("error describing user: %s", errMsg)
	}

	return results[0], nil
}

// ListConsumerGroups 列出所有消费者组
func (c *AdminClient) ListConsumerGroups() (map[string]string, error) {
	return c.admin.ListConsumerGroups()
}

// DescribeConsumerGroups 描述消费者组
func (c *AdminClient) DescribeConsumerGroups(groups []string) ([]*sarama.GroupDescription, error) {
	return c.admin.DescribeConsumerGroups(groups)
}

// ListConsumerGroupOffsets 列出消费者组的 Offset
func (c *AdminClient) ListConsumerGroupOffsets(group string, topicPartitions map[string][]int32) (*sarama.OffsetFetchResponse, error) {
	return c.admin.ListConsumerGroupOffsets(group, topicPartitions)
}

// DescribeCluster 描述集群
func (c *AdminClient) DescribeCluster() ([]*sarama.Broker, int32, error) {
	return c.admin.DescribeCluster()
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

// configureSCRAM 配置 SASL 认证（支持 PLAIN / SCRAM-SHA-256 / SCRAM-SHA-512）
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

	mechanism := "PLAIN"
	if m, ok := authConfig["mechanism"].(string); ok && m != "" {
		mechanism = m
	}

	// 固定使用 SASL_PLAINTEXT（无 TLS）
	config.Net.SASL.Enable = true
	config.Net.SASL.User = username
	config.Net.SASL.Password = password
	config.Net.TLS.Enable = false

	switch mechanism {
	case "PLAIN":
		config.Net.SASL.Mechanism = sarama.SASLTypePlaintext
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
		return fmt.Errorf("%w: unsupported SASL mechanism: %s", ErrInvalidAuthConfig, mechanism)
	}

	return nil
}

// configureKerberos 配置 Kerberos 认证
// authConfigJSON 包含以下字段：
// - principal: Kerberos 主体（如 user/hostname@REALM）
// - realm: Kerberos 域（可从 principal 解析）
// - service_name: 服务名称（默认 kafka）
// - krb5_conf_path: krb5.conf 文件完整路径（运行时写入的临时文件）
// - keytab_path: keytab 文件完整路径（运行时写入的临时文件）
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

	// 优先使用显式提供的 realm，否则从 principal 解析
	realm, _ := authConfig["realm"].(string)
	if realm == "" {
		// 从 principal 中提取 realm（格式：user@REALM 或 user/hostname@REALM）
		atIndex := -1
		for i := len(principal) - 1; i >= 0; i-- {
			if principal[i] == '@' {
				atIndex = i
				break
			}
		}
		if atIndex == -1 || atIndex == len(principal)-1 {
			return fmt.Errorf("%w: cannot extract realm from principal, expected format: user@REALM", ErrInvalidAuthConfig)
		}
		realm = principal[atIndex+1:]
	}

	// 从 principal 中提取 username（去掉 @REALM 部分）
	// sarama/gokrb5 要求 Username 和 Realm 分开传
	atIndex := -1
	for i := len(principal) - 1; i >= 0; i-- {
		if principal[i] == '@' {
			atIndex = i
			break
		}
	}
	username := principal
	if atIndex > 0 {
		username = principal[:atIndex]
	}

	// 获取 krb5.conf 路径
	krb5ConfPath, _ := authConfig["krb5_conf_path"].(string)
	if krb5ConfPath == "" {
		return fmt.Errorf("%w: missing krb5_conf_path", ErrInvalidAuthConfig)
	}

	// 获取 keytab 路径
	keytabPath, _ := authConfig["keytab_path"].(string)
	if keytabPath == "" {
		return fmt.Errorf("%w: missing keytab_path", ErrInvalidAuthConfig)
	}

	serviceName := "kafka"
	if sn, ok := authConfig["service_name"].(string); ok && sn != "" {
		serviceName = sn
	}

	config.Net.SASL.Enable = true
	config.Net.SASL.Mechanism = sarama.SASLTypeGSSAPI
	config.Net.SASL.GSSAPI.ServiceName = serviceName
	config.Net.SASL.GSSAPI.KerberosConfigPath = krb5ConfPath
	config.Net.SASL.GSSAPI.Realm = realm
	config.Net.SASL.GSSAPI.Username = username // 只传用户名部分，不含 @REALM
	config.Net.SASL.GSSAPI.AuthType = sarama.KRB5_KEYTAB_AUTH
	config.Net.SASL.GSSAPI.KeyTabPath = keytabPath

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
