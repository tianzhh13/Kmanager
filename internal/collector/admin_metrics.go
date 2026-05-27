package collector

import (
	"context"
	"log"
	"strconv"
	"strings"

	"kafka-management-platform/internal/models"
	"kafka-management-platform/pkg/kafka"
	"kafka-management-platform/pkg/victoriametrics"
)

// collectAdminMetrics 采集 AdminClient 指标（元数据 + 消费者组 + 分区 Offset）
// 返回指标列表和 nil 错误表示成功，非 nil 错误表示严重失败
func (c *Collector) collectAdminMetrics(ctx context.Context, cluster *models.Cluster, partitionDetails []kafka.TopicPartitionInfo) []victoriametrics.Metric {
	// 1. 获取集群指标（包含 brokers, consumer groups, topic count）
	metrics, err := c.monitorSvc.GetClusterMetrics(ctx, cluster.ClusterID)
	if err != nil {
		log.Printf("[Collector] Failed to get cluster metrics for cluster %d: %v", cluster.ClusterID, err)
		return nil
	}

	baseLabels := map[string]string{
		"cluster_id":   strconv.FormatInt(cluster.ClusterID, 10),
		"cluster_name": cluster.ClusterName,
	}

	var vmMetrics []victoriametrics.Metric

	// 2. 集群级元数据指标
	vmMetrics = append(vmMetrics,
		victoriametrics.Metric{Name: "kafka_broker_count", Value: float64(metrics.BrokerCount), Labels: baseLabels},
		victoriametrics.Metric{Name: "kafka_topic_count", Value: float64(metrics.TopicCount), Labels: baseLabels},
		victoriametrics.Metric{Name: "kafka_consumer_group_count", Value: float64(len(metrics.ConsumerGroups)), Labels: baseLabels},
	)

	// 3. Broker 信息指标（用于 Broker 监控 Tab 筛选）
	for _, broker := range metrics.Brokers {
		brokerLabels := copyLabels(baseLabels)
		brokerLabels["broker_id"] = strconv.FormatInt(int64(broker.ID), 10)
		brokerLabels["broker_host"] = broker.Host
		brokerLabels["broker_port"] = strconv.FormatInt(int64(broker.Port), 10)
		if broker.Rack != "" {
			brokerLabels["broker_rack"] = broker.Rack
		}
		// kafka_broker_info 是一个 info 类型指标，值固定为 1
		vmMetrics = append(vmMetrics,
			victoriametrics.Metric{Name: "kafka_broker_info", Value: 1, Labels: brokerLabels},
		)
	}

	// 4. Topic 分区数指标
	topicPartitions, err := c.monitorSvc.GetTopicPartitionCount(ctx, cluster.ClusterID)
	if err == nil {
		for topic, partitionCount := range topicPartitions {
			labels := copyLabels(baseLabels)
			labels["topic"] = topic
			vmMetrics = append(vmMetrics,
				victoriametrics.Metric{Name: "kafka_topic_partitions", Value: float64(partitionCount), Labels: labels},
			)
		}
	} else {
		log.Printf("[Collector] Failed to get topic partition count for cluster %d: %v", cluster.ClusterID, err)
	}

	// 5. 分区详情指标（副本、ISR、Leader、Offset 等）
	if len(partitionDetails) > 0 {
		// 构建 topic -> partitions 映射用于获取 offset
		tpMap := make(map[string][]int32)
		for _, pd := range partitionDetails {
			tpMap[pd.Topic] = append(tpMap[pd.Topic], pd.Partition)
		}

		// 获取所有分区的 LogEndOffset
		endOffsets, err := c.monitorSvc.GetTopicPartitionOffsets(ctx, cluster.ClusterID, tpMap)
		if err != nil {
			log.Printf("[Collector] Failed to get end offsets for cluster %d: %v", cluster.ClusterID, err)
		}

		// 获取所有分区的 LogStartOffset
		startOffsets, err := c.monitorSvc.GetTopicPartitionStartOffsets(ctx, cluster.ClusterID, tpMap)
		if err != nil {
			log.Printf("[Collector] Failed to get start offsets for cluster %d: %v", cluster.ClusterID, err)
		}

		for _, pd := range partitionDetails {
			labels := copyLabels(baseLabels)
			labels["topic"] = pd.Topic
			labels["partition"] = strconv.FormatInt(int64(pd.Partition), 10)

			// 分区副本数
			vmMetrics = append(vmMetrics,
				victoriametrics.Metric{Name: "kafka_topic_partition_replicas", Value: float64(len(pd.Replicas)), Labels: labels},
			)

			// ISR 数量
			vmMetrics = append(vmMetrics,
				victoriametrics.Metric{Name: "kafka_topic_partition_in_sync_replica", Value: float64(len(pd.ISR)), Labels: labels},
			)

			// 是否是首选 Leader（1 或 0）
			preferredLeaderValue := float64(0)
			if pd.IsPreferredLeader {
				preferredLeaderValue = 1
			}
			vmMetrics = append(vmMetrics,
				victoriametrics.Metric{Name: "kafka_topic_partition_leader_is_preferred", Value: preferredLeaderValue, Labels: labels},
			)

			// 是否未同步（1 或 0）
			underReplicatedValue := float64(0)
			if pd.UnderReplicated {
				underReplicatedValue = 1
			}
			vmMetrics = append(vmMetrics,
				victoriametrics.Metric{Name: "kafka_topic_partition_under_replicated_partition", Value: underReplicatedValue, Labels: labels},
			)

			// 当前偏移量（LogEndOffset）
			logEndOffset := int64(0)
			if endOffsets != nil && endOffsets[pd.Topic] != nil {
				if offset, ok := endOffsets[pd.Topic][pd.Partition]; ok {
					logEndOffset = offset
				}
			}
			vmMetrics = append(vmMetrics,
				victoriametrics.Metric{Name: "kafka_topic_partition_current_offset", Value: float64(logEndOffset), Labels: labels},
			)

			// 最旧偏移量（LogStartOffset）
			logStartOffset := int64(0)
			if startOffsets != nil && startOffsets[pd.Topic] != nil {
				if offset, ok := startOffsets[pd.Topic][pd.Partition]; ok {
					logStartOffset = offset
				}
			}
			vmMetrics = append(vmMetrics,
				victoriametrics.Metric{Name: "kafka_topic_partition_oldest_offset", Value: float64(logStartOffset), Labels: labels},
			)
		}
	}

	// 5.1 Per-Broker Leader/Replica 数量（纯 AdminClient 计算指标，不依赖 JMX）
	if len(partitionDetails) > 0 {
		leaderCount := make(map[int32]int)
		replicaCount := make(map[int32]int)
		for _, pd := range partitionDetails {
			leaderCount[pd.Leader]++
			for _, replica := range pd.Replicas {
				replicaCount[replica]++
			}
		}

		for _, broker := range metrics.Brokers {
			brokerLabels := copyLabels(baseLabels)
			brokerLabels["broker_id"] = strconv.FormatInt(int64(broker.ID), 10)
			brokerLabels["broker_host"] = broker.Host

			vmMetrics = append(vmMetrics,
				victoriametrics.Metric{Name: "kafka_broker_leader_count", Value: float64(leaderCount[broker.ID]), Labels: brokerLabels},
				victoriametrics.Metric{Name: "kafka_broker_replica_count", Value: float64(replicaCount[broker.ID]), Labels: brokerLabels},
			)
		}
	}

	// 6. 消费者组详细指标
	var totalLag int64
	for _, cg := range metrics.ConsumerGroups {
		// 跳过内部消费者组
		if strings.HasPrefix(cg.GroupID, "__") {
			continue
		}

		// 消费者组成员数
		cgLabels := copyLabels(baseLabels)
		cgLabels["consumergroup"] = cg.GroupID
		vmMetrics = append(vmMetrics,
			victoriametrics.Metric{Name: "kafka_consumergroup_members", Value: float64(cg.Members), Labels: cgLabels},
		)

		// 按 Topic 汇总的 Lag
		for _, topicLag := range cg.Topics {
			// 跳过内部 Topic
			if strings.HasPrefix(topicLag.Topic, "__") {
				continue
			}

			topicLabels := copyLabels(baseLabels)
			topicLabels["consumergroup"] = cg.GroupID
			topicLabels["topic"] = topicLag.Topic
			vmMetrics = append(vmMetrics,
				victoriametrics.Metric{Name: "kafka_consumergroup_lag_sum", Value: float64(topicLag.Lag), Labels: topicLabels},
			)

			// 分区级别的 Lag 和 current_offset
			for _, partitionLag := range topicLag.Partitions {
				partitionLabels := copyLabels(topicLabels)
				partitionLabels["partition"] = strconv.FormatInt(int64(partitionLag.Partition), 10)
				vmMetrics = append(vmMetrics,
					victoriametrics.Metric{Name: "kafka_consumergroup_lag", Value: float64(partitionLag.Lag), Labels: partitionLabels},
					victoriametrics.Metric{Name: "kafka_consumergroup_current_offset", Value: float64(partitionLag.CurrentOffset), Labels: partitionLabels},
				)
				// 写入 lag_seconds（仅对有值的分区）
				if partitionLag.LagSeconds >= 0 {
					vmMetrics = append(vmMetrics,
						victoriametrics.Metric{Name: "kafka_consumergroup_lag_seconds", Value: float64(partitionLag.LagSeconds), Labels: partitionLabels},
					)
				}
			}
		}

		totalLag += cg.TotalLag
	}

	// 写入总延迟
	vmMetrics = append(vmMetrics,
		victoriametrics.Metric{Name: "kafka_total_lag", Value: float64(totalLag), Labels: baseLabels},
	)

	if len(vmMetrics) > 0 {
		log.Printf("[Collector] Cluster %d: collected %d admin metrics", cluster.ClusterID, len(vmMetrics))
	}

	return vmMetrics
}

// copyLabels 复制标签 map
func copyLabels(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
