import axios from './api'
import dayjs from 'dayjs'

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
// VictoriaMetrics 查询类型
// ============================================================

export interface VMQueryResponse {
  status: string
  data: {
    result: Array<{
      metric: Record<string, string>
      values: Array<[number, string]>
    }>
  }
}

// ============================================================
// 批量查询类型
// ============================================================

export interface BatchQueryItem {
  id: string
  query: string
  start: number   // unix timestamp
  end: number     // unix timestamp
  step: string
}

export interface BatchQueryResponse {
  results: Record<string, VMQueryResponse>
}

// ============================================================
// 批量查询结果提取工具函数
// ============================================================

/** 提取即时值（取最后一个数据点） */
export function extractInstantValue(response: VMQueryResponse | undefined): number | null {
  if (!response || response.status !== 'success' || response.data.result.length === 0) return null
  const values = response.data.result[0].values
  if (values.length === 0) return null
  return parseFloat(values[values.length - 1][1]) || 0
}

/** 提取范围值（时间序列） */
export function extractRangeValues(response: VMQueryResponse | undefined): Array<[number, string]> {
  if (!response || response.status !== 'success' || response.data.result.length === 0) return []
  return response.data.result[0].values
}

/** 提取多 series 值（按 broker_id 分组） */
export function extractMultiSeries(response: VMQueryResponse | undefined): {
  single: { times: string[]; values: number[] } | null
  brokers: Record<string, { times: string[]; values: number[] }>
} {
  if (!response || response.status !== 'success') return { single: null, brokers: {} }
  const results = response.data.result
  if (results.length === 0) return { single: null, brokers: {} }

  const brokers: Record<string, { times: string[]; values: number[] }> = {}
  results.forEach(r => {
    const brokerId = r.metric.broker_id || 'unknown'
    brokers[brokerId] = {
      times: r.values.map(v => dayjs.unix(v[0]).format('HH:mm')),
      values: r.values.map(v => parseFloat(v[1]) || 0),
    }
  })

  // 保留 single 字段向后兼容（getBrokerLatencyChartOption / 副本 Lag 使用）
  if (results.length === 1) {
    return {
      single: brokers[Object.keys(brokers)[0]] || null,
      brokers,
    }
  }
  return { single: null, brokers }
}

/** 提取错误速率数据（按 request/error 分组） */
export function extractErrorRate(response: VMQueryResponse | undefined): Record<string, { times: string[]; values: number[] }> {
  if (!response || response.status !== 'success') return {}
  const results = response.data.result
  if (results.length === 0) return {}
  const groups: Record<string, { times: string[]; values: number[] }> = {}
  results.forEach(r => {
    const key = `${r.metric.request || 'unknown'}/${r.metric.error || 'unknown'}`
    groups[key] = {
      times: r.values.map(v => dayjs.unix(v[0]).format('HH:mm')),
      values: r.values.map(v => parseFloat(v[1]) || 0),
    }
  })
  return groups
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

  // 批量查询指标（去重 + 缓存，替代多次 /metrics/history）
  batchQuery: (queries: BatchQueryItem[]) =>
    axios.post<BatchQueryResponse>('/metrics/batch-query', { queries }),
}
