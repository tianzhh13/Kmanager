package collector

import (
	"context"
	"log"
	"strconv"

	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/service/monitor"
	"kafka-management-platform/pkg/kafka"
	"kafka-management-platform/pkg/victoriametrics"
)

// collectJMXMetrics 采集 JMX Exporter 指标（per-broker 粒度）+ Per-Broker Leader/Replica 统计
func (c *Collector) collectJMXMetrics(ctx context.Context, cluster *models.Cluster, partitionDetails []kafka.TopicPartitionInfo) []victoriametrics.Metric {
	if cluster.JMXExporterURLs == "" {
		return nil
	}

	urls := monitor.ParseJMXExporterURLs(cluster.JMXExporterURLs)
	if len(urls) == 0 {
		return nil
	}

	baseLabels := map[string]string{
		"cluster_id":   strconv.FormatInt(cluster.ClusterID, 10),
		"cluster_name": cluster.ClusterName,
	}

	multiClient := monitor.NewMultiJMXClient(urls)

	var vmMetrics []victoriametrics.Metric

	// 1. 获取原始 JMX 指标
	rawMetrics, err := multiClient.FetchAllBrokerRawMetrics(ctx)
	if err != nil {
		log.Printf("[Collector] Failed to fetch JMX metrics for cluster %d: %v", cluster.ClusterID, err)
	} else {
		// 构建查找索引：directMappings + brokerTopicMappings
		allDirectMap := make(map[string]string)
		for k, v := range directMappings {
			allDirectMap[k] = v
		}
		for _, dm := range dualNameMappings {
			allDirectMap[dm.OldName] = dm.VMName
			allDirectMap[dm.NewName] = dm.VMName
		}
		for k, v := range brokerTopicMappings {
			allDirectMap[k] = v
		}

		// 构建 quantile+request 查找索引
		qrMap := make(map[string]struct{ VMName, Quantile string })
		for _, qr := range quantileWithRequestMappings {
			qrMap[qr.JMXName] = struct{ VMName, Quantile string }{qr.VMName, qr.Quantile}
		}

		// 构建 quantile only 查找索引（有 quantile 但没有 request 标签的指标）
		qoMap := make(map[string]struct{ VMName, Quantile string })
		for _, qo := range quantileOnlyMappings {
			qoMap[qo.JMXName] = struct{ VMName, Quantile string }{qo.VMName, qo.Quantile}
		}

		// 构建 request+error 查找索引
		reMap := make(map[string]string)
		for _, re := range requestErrorMappings {
			reMap[re.JMXName] = re.VMName
		}

		// 构建 topic+partition 查找索引
		tpMap := make(map[string]string)
		for _, tp := range topicPartitionMappings {
			tpMap[tp.JMXName] = tp.VMName
		}

		// 构建 gc 查找索引
		gcMap := make(map[string]string)
		for _, gc := range gcMappings {
			gcMap[gc.JMXName] = gc.VMName
		}

		// 构建 pool 查找索引
		poolMap := make(map[string]string)
		for _, p := range poolMappings {
			poolMap[p.JMXName] = p.VMName
		}

		// 构建 purgatory 查找索引
		purgMap := make(map[string]string)
		for _, p := range purgatoryMappings {
			purgMap[p.JMXName] = p.VMName
		}

		for _, broker := range rawMetrics {
			brokerLabels := make(map[string]string)
			for k, v := range baseLabels {
				brokerLabels[k] = v
			}
			brokerLabels["broker_id"] = strconv.Itoa(broker.BrokerID)
			brokerLabels["broker_host"] = broker.BrokerHost

			for _, m := range broker.Metrics {
				// 1. quantile+request 模式
				if qr, ok := qrMap[m.Name]; ok {
					if quantile, qok := m.Labels["quantile"]; qok && quantile == qr.Quantile {
						if request, rok := m.Labels["request"]; rok {
							labels := copyLabels(brokerLabels)
							labels["request"] = request
							vmMetrics = append(vmMetrics, victoriametrics.Metric{
								Name: qr.VMName, Value: m.Value, Labels: labels,
							})
						}
					}
					continue
				}

				// 1.5 quantile only 模式（有 quantile 但没有 request 标签）
				if qo, ok := qoMap[m.Name]; ok {
					if quantile, qok := m.Labels["quantile"]; qok && quantile == qo.Quantile {
						vmMetrics = append(vmMetrics, victoriametrics.Metric{
							Name: qo.VMName, Value: m.Value, Labels: brokerLabels,
						})
					}
					continue
				}

				// 2. request+error 模式
				if vmName, ok := reMap[m.Name]; ok {
					if request, rok := m.Labels["request"]; rok {
						if errVal, eok := m.Labels["error"]; eok {
							labels := copyLabels(brokerLabels)
							labels["request"] = request
							labels["error"] = errVal
							vmMetrics = append(vmMetrics, victoriametrics.Metric{
								Name: vmName, Value: m.Value, Labels: labels,
							})
						}
					}
					continue
				}

				// 3. topic+partition 模式
				if vmName, ok := tpMap[m.Name]; ok {
					labels := copyLabels(brokerLabels)
					if topic, tok := m.Labels["topic"]; tok {
						labels["topic"] = topic
					}
					if partition, pok := m.Labels["partition"]; pok {
						labels["partition"] = partition
					}
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name: vmName, Value: m.Value, Labels: labels,
					})
					continue
				}

				// 4. gc 标签模式
				if vmName, ok := gcMap[m.Name]; ok {
					labels := copyLabels(brokerLabels)
					if gc, gok := m.Labels["gc"]; gok {
						labels["gc"] = gc
					}
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name: vmName, Value: m.Value, Labels: labels,
					})
					continue
				}

				// 5. pool 标签模式
				if vmName, ok := poolMap[m.Name]; ok {
					labels := copyLabels(brokerLabels)
					if pool, pok := m.Labels["pool"]; pok {
						labels["pool"] = pool
					}
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name: vmName, Value: m.Value, Labels: labels,
					})
					continue
				}

				// 6. purgatory 标签模式
				if vmName, ok := purgMap[m.Name]; ok {
					labels := copyLabels(brokerLabels)
					if purg, pok := m.Labels["purgatory"]; pok {
						labels["purgatory"] = purg
					}
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name: vmName, Value: m.Value, Labels: labels,
					})
					continue
				}

				// 7. 直接映射
				if vmName, ok := allDirectMap[m.Name]; ok {
					vmMetrics = append(vmMetrics, victoriametrics.Metric{
						Name: vmName, Value: m.Value, Labels: brokerLabels,
					})
				}
			}
		}
	}

	// 2. 从分区详情计算 Per-Broker Leader/Replica 数量
	if len(partitionDetails) > 0 {
		leaderCount := make(map[int32]int)
		replicaCount := make(map[int32]int)
		for _, pd := range partitionDetails {
			leaderCount[pd.Leader]++
			for _, replica := range pd.Replicas {
				replicaCount[replica]++
			}
		}

		// 获取集群元数据以拿到 broker 列表
		metadata, err := c.monitorSvc.GetClusterMetadata(ctx, cluster.ClusterID)
		if err != nil {
			log.Printf("[Collector] Failed to get cluster metadata for cluster %d: %v", cluster.ClusterID, err)
		} else {
			for _, broker := range metadata.Brokers {
				brokerLabels := make(map[string]string)
				for k, v := range baseLabels {
					brokerLabels[k] = v
				}
				brokerLabels["broker_id"] = strconv.FormatInt(int64(broker.ID), 10)
				brokerLabels["broker_host"] = broker.Host

				lc := float64(leaderCount[broker.ID])
				rc := float64(replicaCount[broker.ID])

				vmMetrics = append(vmMetrics,
					victoriametrics.Metric{Name: "kafka_broker_leader_count", Value: lc, Labels: brokerLabels},
					victoriametrics.Metric{Name: "kafka_broker_replica_count", Value: rc, Labels: brokerLabels},
				)
			}
		}
	}

	if len(vmMetrics) > 0 {
		log.Printf("[Collector] Cluster %d: collected %d JMX metrics", cluster.ClusterID, len(vmMetrics))
	}

	return vmMetrics
}
