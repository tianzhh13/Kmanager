package validator

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidationError 验证错误
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Validator 验证器
type Validator struct{}

// New 创建验证器
func New() *Validator {
	return &Validator{}
}

// ValidateUsername 验证用户名（3-64字符，字母数字下划线）
func (v *Validator) ValidateUsername(username string) error {
	if len(username) < 3 || len(username) > 64 {
		return &ValidationError{Field: "username", Message: "username must be between 3 and 64 characters"}
	}

	matched, err := regexp.MatchString(`^[a-zA-Z0-9_]+$`, username)
	if err != nil {
		return &ValidationError{Field: "username", Message: "invalid username format"}
	}
	if !matched {
		return &ValidationError{Field: "username", Message: "username can only contain letters, numbers and underscores"}
	}

	return nil
}

// ValidatePassword 验证密码（至少8字符，包含大小写字母和数字）
func (v *Validator) ValidatePassword(password string) error {
	if len(password) < 8 {
		return &ValidationError{Field: "password", Message: "password must be at least 8 characters"}
	}

	hasUpper := false
	hasLower := false
	hasDigit := false

	for _, c := range password {
		switch {
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= '0' && c <= '9':
			hasDigit = true
		}
	}

	if !hasUpper {
		return &ValidationError{Field: "password", Message: "password must contain at least one uppercase letter"}
	}
	if !hasLower {
		return &ValidationError{Field: "password", Message: "password must contain at least one lowercase letter"}
	}
	if !hasDigit {
		return &ValidationError{Field: "password", Message: "password must contain at least one digit"}
	}

	return nil
}

// ValidateEmail 验证邮箱
func (v *Validator) ValidateEmail(email string) error {
	matched, err := regexp.MatchString(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`, email)
	if err != nil {
		return &ValidationError{Field: "email", Message: "invalid email format"}
	}
	if !matched {
		return &ValidationError{Field: "email", Message: "invalid email format"}
	}

	return nil
}

// ValidateClusterName 验证集群名称（1-128字符）
func (v *Validator) ValidateClusterName(name string) error {
	if len(name) < 1 || len(name) > 128 {
		return &ValidationError{Field: "cluster_name", Message: "cluster name must be between 1 and 128 characters"}
	}

	return nil
}

// ValidateBootstrapServers 验证 Bootstrap Servers（host:port 格式）
func (v *Validator) ValidateBootstrapServers(servers string) error {
	if strings.TrimSpace(servers) == "" {
		return &ValidationError{Field: "bootstrap_servers", Message: "bootstrap servers cannot be empty"}
	}

	// 验证格式：host:port,host:port,...
	serversList := strings.Split(servers, ",")
	for _, server := range serversList {
		server = strings.TrimSpace(server)
		if server == "" {
			continue
		}

		// 检查是否包含端口
		if !strings.Contains(server, ":") {
			return &ValidationError{Field: "bootstrap_servers", Message: fmt.Sprintf("invalid server format: %s (expected host:port)", server)}
		}

		parts := strings.Split(server, ":")
		if len(parts) != 2 {
			return &ValidationError{Field: "bootstrap_servers", Message: fmt.Sprintf("invalid server format: %s", server)}
		}

		host := parts[0]
		port := parts[1]

		if host == "" {
			return &ValidationError{Field: "bootstrap_servers", Message: "host cannot be empty"}
		}

		// 简单端口验证
		if len(port) < 1 || len(port) > 5 {
			return &ValidationError{Field: "bootstrap_servers", Message: "invalid port format"}
		}
	}

	return nil
}

// ValidateTopicName 验证 Topic 名称（Kafka 命名规范）
func (v *Validator) ValidateTopicName(name string) error {
	if len(name) < 1 || len(name) > 249 {
		return &ValidationError{Field: "topic_name", Message: "topic name must be between 1 and 249 characters"}
	}

	// Kafka Topic 名称不能以 . 或 _ 开头
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
		return &ValidationError{Field: "topic_name", Message: "topic name cannot start with . or _"}
	}

	// 只能包含字母、数字、.、- 和 _
	matched, err := regexp.MatchString(`^[a-zA-Z0-9._-]+$`, name)
	if err != nil {
		return &ValidationError{Field: "topic_name", Message: "invalid topic name format"}
	}
	if !matched {
		return &ValidationError{Field: "topic_name", Message: "topic name can only contain letters, numbers, ., - and _"}
	}

	return nil
}

// ValidatePartitionCount 验证分区数（大于 0）
func (v *Validator) ValidatePartitionCount(partitions int) error {
	if partitions <= 0 {
		return &ValidationError{Field: "partitions", Message: "partitions must be greater than 0"}
	}
	if partitions > 1000 {
		return &ValidationError{Field: "partitions", Message: "partitions cannot exceed 1000"}
	}

	return nil
}

// ValidateReplicationFactor 验证副本数（大于 0）
func (v *Validator) ValidateReplicationFactor(replicationFactor int16) error {
	if replicationFactor <= 0 {
		return &ValidationError{Field: "replication_factor", Message: "replication factor must be greater than 0"}
	}
	if replicationFactor > 10 {
		return &ValidationError{Field: "replication_factor", Message: "replication factor cannot exceed 10"}
	}

	return nil
}

// ValidateRole 验证角色
func (v *Validator) ValidateRole(role string) error {
	validRoles := map[string]bool{
		"super_admin":   true,
		"cluster_admin": true,
		"normal_user":   true,
	}

	if !validRoles[role] {
		return &ValidationError{Field: "role", Message: "invalid role. must be one of: super_admin, cluster_admin, normal_user"}
	}

	return nil
}

// ValidateAuthType 验证认证类型
func (v *Validator) ValidateAuthType(authType string) error {
	validAuthTypes := map[string]bool{
		"plaintext": true,
		"scram":     true,
		"kerberos":  true,
	}

	if !validAuthTypes[authType] {
		return &ValidationError{Field: "auth_type", Message: "invalid auth type. must be one of: plaintext, scram, kerberos"}
	}

	return nil
}

// ValidateResourceType 验证资源类型
func (v *Validator) ValidateResourceType(resourceType string) error {
	validTypes := map[string]bool{
		"topic":            true,
		"group":            true,
		"cluster":          true,
		"transactional_id": true,
	}

	if !validTypes[resourceType] {
		return &ValidationError{Field: "resource_type", Message: "invalid resource type"}
	}

	return nil
}

// ValidateOperation 验证操作类型
func (v *Validator) ValidateOperation(operation string) error {
	validOperations := map[string]bool{
		"read":           true,
		"write":          true,
		"create":         true,
		"delete":         true,
		"alter":          true,
		"describe":       true,
		"cluster_action": true,
	}

	if !validOperations[operation] {
		return &ValidationError{Field: "operation", Message: "invalid operation"}
	}

	return nil
}

// ValidatePermissionType 验证权限类型
func (v *Validator) ValidatePermissionType(permissionType string) error {
	if permissionType != "allow" && permissionType != "deny" {
		return &ValidationError{Field: "permission_type", Message: "permission type must be either allow or deny"}
	}

	return nil
}

// ValidateResourcePattern 验证资源匹配模式
func (v *Validator) ValidateResourcePattern(pattern string) error {
	validPatterns := map[string]bool{
		"literal": true,
		"prefix":  true,
	}

	if !validPatterns[pattern] {
		return &ValidationError{Field: "resource_pattern", Message: "resource pattern must be either literal or prefix"}
	}

	return nil
}
