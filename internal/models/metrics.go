package models

import "time"

// ClusterMetrics 集群级别指标
type ClusterMetrics struct {
	ClusterID    int64     `json:"cluster_id"`
	BrokerCount  int       `json:"broker_count"`
	TopicCount   int       `json:"topic_count"`
	MessageRate  float64   `json:"message_rate"`   // 每秒消息数
	BytesInRate  float64   `json:"bytes_in_rate"`  // 字节流入速率
	BytesOutRate float64   `json:"bytes_out_rate"` // 字节流出速率
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
}

// BrokerMetrics Broker 级别指标
type BrokerMetrics struct {
	ClusterID      int64     `json:"cluster_id"`
	BrokerHost     string    `json:"broker_host"`
	CPUUsage       float64   `json:"cpu_usage"`       // CPU 使用率百分比
	MemoryUsage    float64   `json:"memory_usage"`    // 内存使用（字节）
	DiskUsage      float64   `json:"disk_usage"`      // 磁盘使用（字节）
	NetworkInRate  float64   `json:"network_in_rate"` // 网络流入速率
	NetworkOutRate float64   `json:"network_out_rate"`
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time"`
}

// TopicMetrics Topic 级别指标
type TopicMetrics struct {
	ClusterID      int64     `json:"cluster_id"`
	TopicName      string    `json:"topic_name"`
	PartitionCount int       `json:"partition_count"`
	MessageRateIn  float64   `json:"message_rate_in"`  // 消息流入速率
	MessageRateOut float64   `json:"message_rate_out"` // 消息流出速率
	BytesRateIn    float64   `json:"bytes_rate_in"`    // 字节流入速率
	BytesRateOut   float64   `json:"bytes_rate_out"`   // 字节流出速率
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time"`
}

// ConsumerGroupMetrics 消费组指标
type ConsumerGroupMetrics struct {
	ClusterID     int64     `json:"cluster_id"`
	ConsumerGroup string    `json:"consumer_group"`
	Lag           float64   `json:"lag"`          // 消费延迟
	ConsumeRate   float64   `json:"consume_rate"` // 消费速率
	MemberCount   int       `json:"member_count"` // 成员数量
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
}
