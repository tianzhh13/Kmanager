import axios from './api'

// ============================================================
// Broker 指标（来自 JMX Exporter）
// ============================================================

export interface BrokerMetrics {
  bytes_in_per_sec: number
  bytes_out_per_sec: number
  messages_in_per_sec: number
  total_log_size_bytes: number
  under_replicated_partitions: number
  offline_partitions_count: number
  active_controller_count: number
}

// ============================================================
// 消费者组信息（来自内置 Kafka Exporter）
// ============================================================

export interface TopicLag {
  topic: string
  partition: number
  lag: number
  log_end_offset: number
  consumer_offset: number
}

export interface ConsumerGroupInfo {
  group_id: string
  state: string
  member_count: number
  total_lag: number
  topics: TopicLag[]
}

// ============================================================
// 集群指标响应（整合 JMX + Kafka Exporter）
// ============================================================

export interface ClusterMetricsResponse {
  cluster_id: number
  broker_metrics: BrokerMetrics | null
  consumer_groups: ConsumerGroupInfo[]
  broker_count: number
  topic_count: number
  jmx_exporter_available: boolean
  kafka_exporter_available: boolean
}

// ============================================================
// 历史指标（用于折线图）
// ============================================================

export interface MetricsHistoryItem {
  id: number
  cluster_id: number
  timestamp: string
  messages_in_per_sec: number
  bytes_in_per_sec: number
  bytes_out_per_sec: number
  total_log_size_bytes: number
  under_replicated_partitions: number
  offline_partitions_count: number
  total_lag: number
  consumer_group_count: number
  broker_count: number
  topic_count: number
}

// ============================================================
// API
// ============================================================

export const metricsAPI = {
  // 获取集群指标（整合所有数据）
  getClusterMetrics: (clusterId: number) =>
    axios.get<ClusterMetricsResponse>(`/metrics/cluster/${clusterId}`),

  // 获取 Broker 指标（JMX Exporter）
  getBrokerMetrics: (clusterId: number) =>
    axios.get<BrokerMetrics>(`/metrics/broker/${clusterId}`),

  // 获取消费者组 Lag 列表（内置 Kafka Exporter）
  getConsumerGroupLags: (clusterId: number) =>
    axios.get<{ data: ConsumerGroupInfo[] }>(`/metrics/consumer-groups/${clusterId}`),

  // 获取单个消费者组详情
  getConsumerGroupInfo: (clusterId: number, groupId: string) =>
    axios.get<ConsumerGroupInfo>(`/metrics/consumer-group/${clusterId}`, { params: { group: groupId } }),

  // 获取历史指标（用于折线图）
  getMetricsHistory: (clusterId: number, duration: string = '1h') =>
    axios.get<MetricsHistoryItem[]>(`/metrics/history/${clusterId}`, { params: { duration } }),
}
