import axios from './api'

export interface ClusterMetricsParams {
  start: string
  end: string
}

export interface ClusterMetrics {
  cluster_id: number
  broker_count: number
  topic_count: number
  message_rate: number
  bytes_in_rate: number
  bytes_out_rate: number
  start_time: string
  end_time: string
}

export interface BrokerMetrics {
  cluster_id: number
  broker_host: string
  cpu_usage: number
  memory_usage: number
  network_in_rate: number
  network_out_rate: number
  start_time: string
  end_time: string
}

export interface TopicMetrics {
  cluster_id: number
  topic_name: string
  message_rate_in: number
  bytes_rate_in: number
  bytes_rate_out: number
  partition_count: number
  start_time: string
  end_time: string
}

export interface ConsumerGroupMetrics {
  cluster_id: number
  consumer_group: string
  lag: number
  consume_rate: number
  member_count: number
  start_time: string
  end_time: string
}

export interface TimeSeriesPoint {
  timestamp: string
  value: number
}

export interface QueryResult {
  query: string
  data: TimeSeriesPoint[]
}

export const metricsAPI = {
  // 获取集群级别指标
  getClusterMetrics: (clusterId: number, params: ClusterMetricsParams) => 
    axios.get<ClusterMetrics>(`/metrics/cluster/${clusterId}`, { params }),
  
  // 获取 Broker 级别指标
  getBrokerMetrics: (clusterId: number, params: ClusterMetricsParams & { host: string }) =>
    axios.get<BrokerMetrics>(`/metrics/broker/${clusterId}`, { params }),
  
  // 获取 Topic 级别指标
  getTopicMetrics: (clusterId: number, params: ClusterMetricsParams & { topic: string }) =>
    axios.get<TopicMetrics>(`/metrics/topic/${clusterId}`, { params }),
  
  // 获取消费组指标
  getConsumerGroupMetrics: (clusterId: number, params: ClusterMetricsParams & { group: string }) =>
    axios.get<ConsumerGroupMetrics>(`/metrics/consumer-group/${clusterId}`, { params }),
  
  // 自定义 PromQL 查询
  queryPromQL: (clusterId: number, params: ClusterMetricsParams & { query: string; step?: string }) =>
    axios.get<QueryResult>(`/metrics/query/${clusterId}`, { params })
}