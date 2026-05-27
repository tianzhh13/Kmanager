package validator

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

var (
	ErrInvalidUsername     = errors.New("username must be 3-64 characters, containing only letters, numbers and underscores")
	ErrInvalidPassword     = errors.New("password must be at least 8 characters, containing uppercase, lowercase letters and numbers")
	ErrInvalidEmail        = errors.New("invalid email format")
	ErrInvalidClusterName  = errors.New("cluster name must be 1-128 characters")
	ErrInvalidBootstrapURL = errors.New("invalid bootstrap servers format, expected host:port")
	ErrInvalidTopicName    = errors.New("invalid topic name, must match Kafka naming rules")
	ErrInvalidPartition    = errors.New("partition number must be greater than 0")
	ErrInvalidReplica      = errors.New("replication factor must be greater than 0")
)

// UsernameValidator 用户名验证
// 要求：3-64 字符，字母数字下划线
func UsernameValidator(username string) error {
	if len(username) < 3 || len(username) > 64 {
		return ErrInvalidUsername
	}

	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_]+$`, username)
	if !matched {
		return ErrInvalidUsername
	}

	return nil
}

// EmailValidator 邮箱验证
func EmailValidator(email string) error {
	if email == "" {
		return nil // 邮箱可选
	}

	matched, _ := regexp.MatchString(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`, email)
	if !matched {
		return ErrInvalidEmail
	}

	return nil
}

// ClusterNameValidator 集群名称验证
// 要求：1-128 字符
func ClusterNameValidator(name string) error {
	if len(name) < 1 || len(name) > 128 {
		return ErrInvalidClusterName
	}

	return nil
}

// BootstrapServersValidator Bootstrap Servers 验证
// 要求：host:port 格式，支持多个地址用逗号分隔
func BootstrapServersValidator(servers string) error {
	if servers == "" {
		return ErrInvalidBootstrapURL
	}

	// 支持多个地址
	addresses := strings.Split(servers, ",")
	for _, addr := range addresses {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}

		// 检查格式 host:port
		parts := strings.Split(addr, ":")
		if len(parts) != 2 {
			return ErrInvalidBootstrapURL
		}

		host := parts[0]
		port := parts[1]

		if host == "" {
			return ErrInvalidBootstrapURL
		}

		// 简单端口验证
		if len(port) == 0 || len(port) > 5 {
			return ErrInvalidBootstrapURL
		}

		for _, c := range port {
			if c < '0' || c > '9' {
				return ErrInvalidBootstrapURL
			}
		}

		// 验证端口范围 1-65535
		portNum, err := strconv.Atoi(port)
		if err != nil || portNum < 1 || portNum > 65535 {
			return ErrInvalidBootstrapURL
		}
	}

	return nil
}

// TopicNameValidator Topic 名称验证
// Kafka Topic 命名规则：
// - 不能超过 249 字符
// - 只能包含字母、数字、点号(.)、下划线(_)和连字符(-)
// - 不能以点号(.)开头或结尾
// - 不能有连续的点号(..)
func TopicNameValidator(name string) error {
	if name == "" {
		return ErrInvalidTopicName
	}

	if len(name) > 249 {
		return ErrInvalidTopicName
	}

	// 不能以点号开头或结尾
	if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") {
		return ErrInvalidTopicName
	}

	// 不能有连续的点号
	if strings.Contains(name, "..") {
		return ErrInvalidTopicName
	}

	// 只能包含字母、数字、点号、下划线和连字符
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9._-]+$`, name)
	if !matched {
		return ErrInvalidTopicName
	}

	return nil
}

// PartitionValidator 分区数验证
func PartitionValidator(partition int) error {
	if partition <= 0 {
		return ErrInvalidPartition
	}

	// Kafka 最大分区数限制
	if partition > 10000 {
		return errors.New("partition number exceeds maximum limit of 10000")
	}

	return nil
}

// ReplicationFactorValidator 副本数验证
func ReplicationFactorValidator(replica int) error {
	if replica <= 0 {
		return ErrInvalidReplica
	}

	// Kafka 最大副本数限制
	if replica > 100 {
		return errors.New("replication factor exceeds maximum limit of 100")
	}

	return nil
}

// PhoneValidator 手机号验证（中国大陆）
func PhoneValidator(phone string) error {
	if phone == "" {
		return nil // 手机号可选
	}

	matched, _ := regexp.MatchString(`^1[3-9]\d{9}$`, phone)
	if !matched {
		return errors.New("invalid phone number format")
	}

	return nil
}

// URLValidator URL 验证
func URLValidator(url string) error {
	if url == "" {
		return nil // URL 可选
	}

	matched, _ := regexp.MatchString(`^https?://[^\s]+$`, url)
	if !matched {
		return errors.New("invalid URL format")
	}

	return nil
}