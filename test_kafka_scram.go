package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/IBM/sarama"
	"github.com/xdg-go/scram"
)

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

func main() {
	// Kafka 集群配置
	brokers := []string{"192.170.1.106:9093"}
	username := "admin"
	password := "admin1234asdf"
	mechanism := "SCRAM-SHA-256"
	securityProtocol := "SASL_PLAINTEXT" // SASL_SSL 或 SASL_PLAINTEXT

	fmt.Println("========================================")
	fmt.Println("Kafka SCRAM 连接测试")
	fmt.Println("========================================")
	fmt.Printf("Brokers: %v\n", brokers)
	fmt.Printf("Username: %s\n", username)
	fmt.Printf("Mechanism: %s\n", mechanism)
	fmt.Printf("Security Protocol: %s\n", securityProtocol)
	fmt.Println("========================================")

	// 创建配置
	config := sarama.NewConfig()
	config.Version = sarama.V2_8_0_0

	// 配置 SASL
	config.Net.SASL.Enable = true
	config.Net.SASL.User = username
	config.Net.SASL.Password = password

	// 配置 SCRAM 机制
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
	}

	// 根据安全协议配置 TLS
	switch strings.ToUpper(securityProtocol) {
	case "SASL_SSL":
		fmt.Println("配置: SASL over TLS")
		config.Net.TLS.Enable = true
		config.Net.TLS.Config = &tls.Config{
			InsecureSkipVerify: true, // 测试环境跳过证书验证
		}
	case "SASL_PLAINTEXT":
		fmt.Println("配置: SASL over 明文传输")
		config.Net.TLS.Enable = false
	default:
		log.Fatalf("不支持的安全协议: %s", securityProtocol)
	}

	fmt.Println("========================================")
	fmt.Println("正在连接 Kafka 集群...")

	// 测试 1: 创建同步生产者（测试连接）
	fmt.Println("\n[测试 1] 创建同步生产者...")
	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		log.Printf("❌ 创建生产者失败: %v", err)
	} else {
		fmt.Println("✅ 创建生产者成功")
		producer.Close()
	}

	// 测试 2: 创建客户端（测试连接）
	fmt.Println("\n[测试 2] 创建客户端...")
	client, err := sarama.NewClient(brokers, config)
	if err != nil {
		log.Printf("❌ 创建客户端失败: %v", err)
	} else {
		fmt.Println("✅ 创建客户端成功")
		
		// 获取集群信息
		fmt.Println("\n[测试 3] 获取集群信息...")
		brokersInfo := client.Brokers()
		fmt.Printf("发现 %d 个 Broker:\n", len(brokersInfo))
		for _, b := range brokersInfo {
			fmt.Printf("  - Broker ID: %d, Address: %s\n", b.ID(), b.Addr())
		}
		
		// 获取 Topic 列表
		fmt.Println("\n[测试 4] 获取 Topic 列表...")
		topics, err := client.Topics()
		if err != nil {
			log.Printf("❌ 获取 Topic 列表失败: %v", err)
		} else {
			fmt.Printf("✅ 发现 %d 个 Topic:\n", len(topics))
			for i, topic := range topics {
				if i < 10 { // 只显示前 10 个
					fmt.Printf("  - %s\n", topic)
				}
			}
			if len(topics) > 10 {
				fmt.Printf("  ... 还有 %d 个 Topic\n", len(topics)-10)
			}
		}
		
		client.Close()
	}

	// 测试 3: 创建 Admin 客户端
	fmt.Println("\n[测试 5] 创建 Admin 客户端...")
	admin, err := sarama.NewClusterAdmin(brokers, config)
	if err != nil {
		log.Printf("❌ 创建 Admin 客户端失败: %v", err)
	} else {
		fmt.Println("✅ 创建 Admin 客户端成功")
		
		// 描述集群
		fmt.Println("\n[测试 6] 描述集群...")
		brokersInfo, controllerID, err := admin.DescribeCluster()
		if err != nil {
			log.Printf("❌ 描述集群失败: %v", err)
		} else {
			fmt.Printf("✅ 集群信息:\n")
			fmt.Printf("  Controller ID: %d\n", controllerID)
			fmt.Printf("  Brokers: %d\n", len(brokersInfo))
			for _, b := range brokersInfo {
				fmt.Printf("    - ID: %d, Addr: %s\n", b.ID(), b.Addr())
			}
		}
		
		// 列出 Topics
		fmt.Println("\n[测试 7] 列出 Topics...")
		topics, err := admin.ListTopics()
		if err != nil {
			log.Printf("❌ 列出 Topics 失败: %v", err)
		} else {
			fmt.Printf("✅ 发现 %d 个 Topic\n", len(topics))
		}
		
		admin.Close()
	}

	fmt.Println("\n========================================")
	fmt.Println("测试完成")
	fmt.Println("========================================")

	// 打印认证配置 JSON（用于前端）
	authConfig := map[string]interface{}{
		"username":          username,
		"password":          password,
		"mechanism":         mechanism,
		"security_protocol": securityProtocol,
	}
	authConfigJSON, _ := json.MarshalIndent(authConfig, "", "  ")
	fmt.Println("\n前端认证配置 JSON:")
	fmt.Println(string(authConfigJSON))
}
