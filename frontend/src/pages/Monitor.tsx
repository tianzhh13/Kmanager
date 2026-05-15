import { useState, useEffect } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Card, Row, Col, Select, Spin, message, Statistic, Table, Tabs, Space, Tag, Alert, DatePicker, Button, Checkbox } from 'antd'
import { CalendarOutlined } from '@ant-design/icons'
import ReactECharts from 'echarts-for-react'
import dayjs, { Dayjs } from 'dayjs'
import { clusterAPI } from '../services/cluster'
import { metricsAPI, ClusterMetricsResponse } from '../services/metrics'
import axios from '../services/api'
import DashboardGrid from '../components/DashboardGrid'

interface ClusterOption {
  cluster_id: number
  cluster_name: string
}

// Topic 信息接口
interface TopicInfo {
  name: string
  partitions: number
  replication_factor: number
  log_size_bytes?: number
}

// Topic 分区指标
interface PartitionMetric {
  partition: number
  values: { time: string; value: number }[]
}

const Monitor: React.FC = () => {
  const [searchParams] = useSearchParams()
  const [loading, setLoading] = useState(false)
  const [clusters, setClusters] = useState<ClusterOption[]>([])
  const [selectedCluster, setSelectedCluster] = useState<ClusterOption | null>(null)
  const [metrics, setMetrics] = useState<ClusterMetricsResponse | null>(null)
  const [activeTab, setActiveTab] = useState('overview')

  // 时间范围选择
  const [timeRange, setTimeRange] = useState<'quick' | 'custom'>('quick')
  const [quickRange, setQuickRange] = useState<string>('1h')
  const [customRange, setCustomRange] = useState<[Dayjs, Dayjs] | null>(null)

  // Topic 监控 - 状态
  const [topics, setTopics] = useState<TopicInfo[]>([])
  const [selectedTopic, setSelectedTopic] = useState<string | null>(null)
  const [selectedConsumerGroup, setSelectedConsumerGroup] = useState<string | null>(null)
  const [topicConsumerGroups, setTopicConsumerGroups] = useState<string[]>([])
  const [topicLoading, setTopicLoading] = useState(false)
  const [selectedPartitions, setSelectedPartitions] = useState<number[]>([])
  const [partitionMetrics, setPartitionMetrics] = useState<{
    produceRate: PartitionMetric[]
    consumeRate: PartitionMetric[]
    lag: PartitionMetric[]
  }>({ produceRate: [], consumeRate: [], lag: [] })
  // 批次8新增：Topic 分区级指标
  const [topicLogSizeData, setTopicLogSizeData] = useState<PartitionMetric[]>([])
  const [topicLogEndOffsetData, setTopicLogEndOffsetData] = useState<PartitionMetric[]>([])
  const [topicIsrVsReplicaData, setTopicIsrVsReplicaData] = useState<{ isr: PartitionMetric[], replica: PartitionMetric[] }>({ isr: [], replica: [] })
  const [topicUnderReplicatedCount, setTopicUnderReplicatedCount] = useState(0)

  // Broker 监控 - 状态
  const [brokerOverviewData, setBrokerOverviewData] = useState<any[]>([])
  const [brokerOverviewLoading, setBrokerOverviewLoading] = useState(false)
  const [selectedBroker, setSelectedBroker] = useState<string>('all')
  const [brokerList, setBrokerList] = useState<{ id: string; host: string }[]>([])
  const [brokerRequestLatencyData, setBrokerRequestLatencyData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerReplicaLagData, setBrokerReplicaLagData] = useState<{ times: string[], values: number[] }>({ times: [], values: [] })
  const [brokerBytesInData, setBrokerBytesInData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerBytesOutData, setBrokerBytesOutData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerChartLoading, setBrokerChartLoading] = useState(false)
  // 批次2新增：请求延迟分解数据
  const [brokerQueueTimeData, setBrokerQueueTimeData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerLocalTimeData, setBrokerLocalTimeData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerRemoteTimeData, setBrokerRemoteTimeData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerThrottleTimeData, setBrokerThrottleTimeData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerErrorsData, setBrokerErrorsData] = useState<Record<string, { times: string[], values: number[] }>>({})
  // 批次3新增：额外流量数据
  const [brokerReplicationInData, setBrokerReplicationInData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerReplicationOutData, setBrokerReplicationOutData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerReassignmentInData, setBrokerReassignmentInData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerReassignmentOutData, setBrokerReassignmentOutData] = useState<Record<string, { times: string[], values: number[] }>>({})
  // 批次5新增：副本-detail 数据
  const [brokerIsrShrinksData, setBrokerIsrShrinksData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerIsrExpandsData, setBrokerIsrExpandsData] = useState<Record<string, { times: string[], values: number[] }>>({})
  // 批次6新增：网络/线程数据
  const [brokerResponseQueueData, setBrokerResponseQueueData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerHandlerIdleData, setBrokerHandlerIdleData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerNetworkIdleData, setBrokerNetworkIdleData] = useState<Record<string, { times: string[], values: number[] }>>({})
  // 批次7新增：Broker 状态数据
  const [brokerDiskReadData, setBrokerDiskReadData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerDiskWriteData, setBrokerDiskWriteData] = useState<Record<string, { times: string[], values: number[] }>>({})
  // 新增：ISR 更新失败总数
  const [brokerIsrUpdatesFailed, setBrokerIsrUpdatesFailed] = useState<number>(0)
  // 新增：Controller 事件、延迟操作、Replica Fetcher、Log Flush、系统进程
  const [brokerControllerEventQueueData, setBrokerControllerEventQueueData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerUncleanLeaderElections, setBrokerUncleanLeaderElections] = useState<number>(0)
  const [brokerDelayedOperationsData, setBrokerDelayedOperationsData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerPurgatorySizeData, setBrokerPurgatorySizeData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerDelayedFetchExpires, setBrokerDelayedFetchExpires] = useState<number>(0)
  const [brokerMinFetchRateData, setBrokerMinFetchRateData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerFailedPartitionsCount, setBrokerFailedPartitionsCount] = useState<number>(0)
  const [brokerDeadThreadCount, setBrokerDeadThreadCount] = useState<number>(0)
  const [brokerLogFlushTimeData, setBrokerLogFlushTimeData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerProcessCpuData, setBrokerProcessCpuData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerProcessResidentMemoryData, setBrokerProcessResidentMemoryData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerProcessVirtualMemoryData, setBrokerProcessVirtualMemoryData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerProcessStartTime, setBrokerProcessStartTime] = useState<number>(0)
  const [brokerProcessMaxFds, setBrokerProcessMaxFds] = useState<number>(0)
  const [brokerProcessOpenFdsData, setBrokerProcessOpenFdsData] = useState<Record<string, { times: string[], values: number[] }>>({})
  // 新增：Log Cleaner
  const [brokerLogCleanerMaxDirtyData, setBrokerLogCleanerMaxDirtyData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerLogCleanerTimeSinceLastRunData, setBrokerLogCleanerTimeSinceLastRunData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerLogCleanerUncleanableBytesData, setBrokerLogCleanerUncleanableBytesData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerLogCleanerUncleanablePartitions, setBrokerLogCleanerUncleanablePartitions] = useState<number>(0)
  const [brokerLogCleanerRecopyData, setBrokerLogCleanerRecopyData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerLogCleanerDeadThreads, setBrokerLogCleanerDeadThreads] = useState<number>(0)
  const [brokerLogCleanerMaxBufferData, setBrokerLogCleanerMaxBufferData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerLogCleanerMaxCleanTimeData, setBrokerLogCleanerMaxCleanTimeData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerLogCleanerMaxCompactionDelayData, setBrokerLogCleanerMaxCompactionDelayData] = useState<Record<string, { times: string[], values: number[] }>>({})
  // 新增：JVM
  const [brokerJvmGcData, setBrokerJvmGcData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerJvmGcCountData, setBrokerJvmGcCountData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerJvmMemoryPoolData, setBrokerJvmMemoryPoolData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerJvmThreadsData, setBrokerJvmThreadsData] = useState<Record<string, { times: string[], values: number[] }>>({})
  const [brokerJvmDeadlockedThreads, setBrokerJvmDeadlockedThreads] = useState<number>(0)
  const [brokerJvmBufferPoolData, setBrokerJvmBufferPoolData] = useState<Record<string, { times: string[], values: number[] }>>({})

  // 集群概览 - 统计指标（从 VM 查询）
  const [overviewStats, setOverviewStats] = useState({
    topicPartitionTotal: 0,
    consumerGroupMemberCount: 0,
    consumerGroupMemberTotal: 0,
    isrTotal: 0,
    nonPreferredLeaderCount: 0,
    // 批次1新增指标
    activeBrokerCount: 0,
    fencedBrokerCount: 0,
    globalPartitionCount: 0,
    globalTopicCount: 0,
    preferredReplicaImbalance: 0,
    // 新增：离线分区数、活跃 Controller 数量
    offlinePartitionsCount: 0,
    activeControllerCount: 0,
    // 新增：离线日志目录数、日志目录离线状态
    offlineLogDirectoryCount: 0,
    logDirectoryOffline: 0,
  })

  // 集群概览 - 趋势数据
  const [produceRateData, setProduceRateData] = useState<{ times: string[], values: number[] }>({ times: [], values: [] })
  const [consumeRateData, setConsumeRateData] = useState<{ times: string[], values: number[] }>({ times: [], values: [] })
  const [lagTrendData, setLagTrendData] = useState<{ times: string[], values: number[] }>({ times: [], values: [] })
  const [bytesInData, setBytesInData] = useState<{ times: string[], values: number[] }>({ times: [], values: [] })
  const [bytesOutData, setBytesOutData] = useState<{ times: string[], values: number[] }>({ times: [], values: [] })
  const [bytesRejectedData, setBytesRejectedData] = useState<{ times: string[], values: number[] }>({ times: [], values: [] })
  // 新增：消息流入速率趋势数据
  const [messagesInRateData, setMessagesInRateData] = useState<{ times: string[], values: number[] }>({ times: [], values: [] })
  // 批次4新增：错误数据
  const [failedProduceRateData, setFailedProduceRateData] = useState<{ times: string[], values: number[] }>({ times: [], values: [] })
  const [failedFetchRateData, setFailedFetchRateData] = useState<{ times: string[], values: number[] }>({ times: [], values: [] })
  const [dataCorruptionStats, setDataCorruptionStats] = useState({
    invalidMagicNumber: 0,
    invalidCrc: 0,
    invalidOffset: 0,
  })

  // 加载集群列表
  useEffect(() => {
    loadClusters()
  }, [])

  // 加载监控数据
  useEffect(() => {
    if (selectedCluster) {
      loadMetrics()
    }
  }, [selectedCluster])

  // 加载历史数据
  useEffect(() => {
    if (selectedCluster) {
      loadHistory()
    }
  }, [selectedCluster, quickRange, customRange, timeRange])

  // 当切换到 Topic 监控 Tab 时加载 Topic 列表
  useEffect(() => {
    if (activeTab === 'topic' && selectedCluster) {
      loadTopics()
    }
  }, [activeTab, selectedCluster])

  // 当选择 Topic 时加载该 Topic 的消费组列表
  useEffect(() => {
    if (selectedTopic && selectedCluster && metrics?.consumer_groups) {
      // 从 metrics 中过滤出消费该 Topic 的消费组
      const cgs = metrics.consumer_groups
        .filter(cg => cg.topics.some(t => t.topic === selectedTopic))
        .map(cg => cg.group_id)
      setTopicConsumerGroups(cgs)
      // 如果当前选择的消费组不在列表中，清空选择
      if (selectedConsumerGroup && !cgs.includes(selectedConsumerGroup)) {
        setSelectedConsumerGroup(null)
      }
    } else {
      setTopicConsumerGroups([])
      setSelectedConsumerGroup(null)
    }
  }, [selectedTopic, metrics?.consumer_groups])

  // 当选择 Topic 时重置分区指标
  useEffect(() => {
    setPartitionMetrics({ produceRate: [], consumeRate: [], lag: [] })
    setSelectedPartitions([])
  }, [selectedTopic])

  // 当选择 ConsumerGroup 时重置消费相关指标
  useEffect(() => {
    setPartitionMetrics(prev => ({ ...prev, consumeRate: [], lag: [] }))
  }, [selectedConsumerGroup])

  // 当选择 Topic 或 ConsumerGroup 时加载分区指标
  useEffect(() => {
    if (selectedTopic && selectedCluster && activeTab === 'topic') {
      loadPartitionMetrics()
    }
  }, [selectedTopic, selectedConsumerGroup, selectedCluster, quickRange, customRange, timeRange, activeTab])

  // 当切换到 Broker 监控 Tab 时加载数据
  useEffect(() => {
    if (activeTab === 'broker' && selectedCluster) {
      loadBrokerOverview()
      loadBrokerChartData()
    }
  }, [activeTab, selectedCluster])

  // Broker 选择或时间范围变化时重新加载图表
  useEffect(() => {
    if (activeTab === 'broker' && selectedCluster) {
      loadBrokerChartData()
    }
  }, [selectedBroker, quickRange, customRange, timeRange])

  const loadClusters = async () => {
    try {
      const res = await clusterAPI.list()
      setClusters(res.data || [])
      if (res.data?.length > 0) {
        // 读取 URL 参数
        const clusterIdParam = searchParams.get('clusterId')
        const tabParam = searchParams.get('tab')
        const topicNameParam = searchParams.get('topicName')

        // 根据 clusterId 参数选择集群，否则选第一个
        if (clusterIdParam) {
          const targetCluster = res.data.find((c: ClusterOption) => c.cluster_id === parseInt(clusterIdParam))
          if (targetCluster) {
            setSelectedCluster(targetCluster)
          } else {
            setSelectedCluster(res.data[0])
          }
        } else {
          setSelectedCluster(res.data[0])
        }

        // 设置 Tab
        if (tabParam && ['overview', 'broker', 'topic'].includes(tabParam)) {
          setActiveTab(tabParam)
        }

        // 设置 Topic（需等待 topics 加载，这里只设置标记）
        if (topicNameParam && tabParam === 'topic') {
          setSelectedTopic(topicNameParam)
        }
      }
    } catch (error) {
      message.error('加载集群列表失败')
    }
  }

  const loadMetrics = async () => {
    if (!selectedCluster) return
    
    setLoading(true)
    try {
      const res = await metricsAPI.getClusterMetrics(selectedCluster.cluster_id)
      setMetrics(res.data)
    } catch (error) {
      message.error('加载监控数据失败')
    } finally {
      setLoading(false)
    }
  }

  // 加载 Topic 列表
  const loadTopics = async () => {
    if (!selectedCluster) return
    
    setTopicLoading(true)
    try {
      // 从 VM 获取 Topic 列表（通过 kafka_topic_partitions 指标）
      const res = await axios.get<VMQueryResponse>('/metrics/history', {
        params: {
          query: `kafka_topic_partitions{cluster_id="${selectedCluster.cluster_id}"}`,
          start: dayjs().subtract(1, 'minute').unix(),
          end: dayjs().unix(),
          step: '60s'
        }
      })
      
      if (res.data.status === 'success') {
        const topicMap = new Map<string, TopicInfo>()
        res.data.data.result.forEach(r => {
          const name = r.metric.topic
          if (name && !topicMap.has(name)) {
            topicMap.set(name, {
              name,
              partitions: parseInt(r.metric.partition_count || '0') || 1,
              replication_factor: 1,
            })
          }
        })
        setTopics(Array.from(topicMap.values()))
      }
    } catch (error) {
      console.error('Failed to load topics', error)
    } finally {
      setTopicLoading(false)
    }
  }

  // 加载 Broker 总览数据
  const loadBrokerOverview = async () => {
    if (!selectedCluster) return
    
    setBrokerOverviewLoading(true)
    try {
      const res = await axios.get(`/metrics/broker-overview/${selectedCluster.cluster_id}`)
      const data = res.data?.data || []
      setBrokerOverviewData(data)
      // 更新 broker 列表
      const list = data.map((b: any) => ({ id: String(b.broker_id), host: b.broker_host }))
      setBrokerList(list)
    } catch (error) {
      console.error('Failed to load broker overview', error)
    } finally {
      setBrokerOverviewLoading(false)
    }
  }

  // 加载 Broker 图表数据
  const loadBrokerChartData = async () => {
    if (!selectedCluster) return
    
    setBrokerChartLoading(true)
    try {
      const { start, end, step } = getTimeRange()
      const clusterId = selectedCluster.cluster_id
      const brokerFilter = selectedBroker === 'all' ? '' : `,broker_id="${selectedBroker}"`

      // 分批查询避免 429 限流（每批最多 15 个请求）
      const batchSize = 15
      const queries = [
        // 批次1：基础指标
        () => queryVMMulti(`kafka_broker_request_latency_ms{cluster_id="${clusterId}",request="Produce"${brokerFilter}}`, start, end, step),
        () => queryVMMulti(`kafka_broker_request_latency_ms{cluster_id="${clusterId}",request="FetchConsumer"${brokerFilter}}`, start, end, step),
        () => queryVMMulti(`kafka_broker_request_latency_ms{cluster_id="${clusterId}",request="FetchFollower"${brokerFilter}}`, start, end, step),
        () => queryVMMulti(`kafka_broker_replica_max_lag{cluster_id="${clusterId}"${brokerFilter}}`, start, end, step),
        () => queryVMMulti(`rate(kafka_broker_bytes_in_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`, start, end, step),
        () => queryVMMulti(`rate(kafka_broker_bytes_out_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`, start, end, step),
        () => queryVMMulti(`kafka_broker_request_queue_time_ms{cluster_id="${clusterId}",request="Produce"${brokerFilter}}`, start, end, step),
        () => queryVMMulti(`kafka_broker_request_local_time_ms{cluster_id="${clusterId}",request="Produce"${brokerFilter}}`, start, end, step),
        () => queryVMMulti(`kafka_broker_request_remote_time_ms{cluster_id="${clusterId}",request="FetchConsumer"${brokerFilter}}`, start, end, step),
        () => queryVMMulti(`kafka_broker_throttle_time_ms{cluster_id="${clusterId}",request="Produce"${brokerFilter}}`, start, end, step),
        () => queryVMErrorRate(`sum by (request, error) (rate(kafka_broker_request_errors_total{cluster_id="${clusterId}",error!~"NONE"${brokerFilter}}[30s]))`, start, end, step),
        () => queryVMMulti(`rate(kafka_broker_replication_bytes_in_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`, start, end, step),
        () => queryVMMulti(`rate(kafka_broker_replication_bytes_out_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`, start, end, step),
        () => queryVMMulti(`rate(kafka_broker_reassignment_bytes_in_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`, start, end, step),
        () => queryVMMulti(`rate(kafka_broker_reassignment_bytes_out_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`, start, end, step),
        // 批次2：ISR + 网络 + 磁盘
        () => queryVMMulti(`rate(kafka_broker_isr_shrinks_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`, start, end, step),
        () => queryVMMulti(`rate(kafka_broker_isr_expands_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`, start, end, step),
        () => queryVMMulti(`kafka_broker_response_queue_size{cluster_id="${clusterId}"${brokerFilter}}`, start, end, step),
        () => queryVMMulti(`kafka_broker_request_handler_avg_idle_percent{cluster_id="${clusterId}"${brokerFilter}}`, start, end, step),
        () => queryVMMulti(`kafka_broker_network_processor_avg_idle_percent{cluster_id="${clusterId}"${brokerFilter}}`, start, end, step),
        () => queryVMMulti(`rate(kafka_broker_disk_read_bytes{cluster_id="${clusterId}"${brokerFilter}}[30s])`, start, end, step),
        () => queryVMMulti(`rate(kafka_broker_disk_write_bytes{cluster_id="${clusterId}"${brokerFilter}}[30s])`, start, end, step),
        () => queryVMInstant(`sum(kafka_broker_isr_updates_failed_total{cluster_id="${clusterId}"${brokerFilter}})`),
        () => queryVMMulti(`kafka_broker_controller_event_queue_time_ms{cluster_id="${clusterId}"${brokerFilter}}`, start, end, step),
        () => queryVMInstant(`sum(kafka_broker_unclean_leader_elections_total{cluster_id="${clusterId}"${brokerFilter}})`),
        // 批次3：延迟操作 + Replica Fetcher + Log Flush + 系统进程
        () => queryVMMulti(`kafka_broker_delayed_operations{cluster_id="${clusterId}"${brokerFilter}}`, start, end, step),
        () => queryVMMulti(`kafka_broker_purgatory_size{cluster_id="${clusterId}"${brokerFilter}}`, start, end, step),
        () => queryVMInstant(`sum(kafka_broker_delayed_fetch_expires_total{cluster_id="${clusterId}"${brokerFilter}})`),
        () => queryVMMulti(`kafka_broker_min_fetch_rate{cluster_id="${clusterId}"${brokerFilter}}`, start, end, step),
        () => queryVMInstant(`max(kafka_broker_failed_partitions_count{cluster_id="${clusterId}"${brokerFilter}})`),
        () => queryVMInstant(`max(kafka_broker_dead_thread_count{cluster_id="${clusterId}"${brokerFilter}})`),
        () => queryVMMulti(`kafka_broker_log_flush_time_ms{cluster_id="${clusterId}"${brokerFilter}}`, start, end, step),
        () => queryVMMulti(`rate(kafka_broker_process_cpu_seconds_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`, start, end, step),
        () => queryVMMulti(`kafka_broker_process_resident_memory_bytes{cluster_id="${clusterId}"${brokerFilter}}`, start, end, step),
        () => queryVMMulti(`kafka_broker_process_virtual_memory_bytes{cluster_id="${clusterId}"${brokerFilter}}`, start, end, step),
        // 批次4：系统进程 + Log Cleaner + JVM
        () => queryVMInstant(`max(kafka_broker_process_start_time_seconds{cluster_id="${clusterId}"${brokerFilter}})`),
        () => queryVMInstant(`max(kafka_broker_process_max_fds{cluster_id="${clusterId}"${brokerFilter}})`),
        () => queryVMMulti(`kafka_broker_process_open_fds{cluster_id="${clusterId}"${brokerFilter}}`, start, end, step),
        () => queryVMMulti(`kafka_broker_log_cleaner_max_dirty_percent{cluster_id="${clusterId}"${brokerFilter}}`, start, end, step),
        () => queryVMMulti(`kafka_broker_log_cleaner_time_since_last_run_ms{cluster_id="${clusterId}"${brokerFilter}}`, start, end, step),
        () => queryVMMulti(`kafka_broker_log_cleaner_uncleanable_bytes{cluster_id="${clusterId}"${brokerFilter}}`, start, end, step),
        () => queryVMInstant(`max(kafka_broker_log_cleaner_uncleanable_partitions_count{cluster_id="${clusterId}"${brokerFilter}})`),
        () => queryVMMulti(`kafka_broker_log_cleaner_recopy_percent{cluster_id="${clusterId}"${brokerFilter}}`, start, end, step),
        () => queryVMInstant(`max(kafka_broker_log_cleaner_dead_thread_count{cluster_id="${clusterId}"${brokerFilter}})`),
        () => queryVMMulti(`kafka_broker_log_cleaner_max_buffer_utilization_percent{cluster_id="${clusterId}"${brokerFilter}}`, start, end, step),
        () => queryVMMulti(`kafka_broker_log_cleaner_max_clean_time_secs{cluster_id="${clusterId}"${brokerFilter}}`, start, end, step),
        () => queryVMMulti(`kafka_broker_log_cleaner_max_compaction_delay_secs{cluster_id="${clusterId}"${brokerFilter}}`, start, end, step),
        // 批次5：JVM
        () => queryVMMulti(`kafka_broker_jvm_gc_seconds_sum{cluster_id="${clusterId}"${brokerFilter}}`, start, end, step),
        () => queryVMMulti(`kafka_broker_jvm_gc_count{cluster_id="${clusterId}"${brokerFilter}}`, start, end, step),
        () => queryVMMulti(`kafka_broker_jvm_memory_pool_used_bytes{cluster_id="${clusterId}"${brokerFilter}}`, start, end, step),
        () => queryVMMulti(`kafka_broker_jvm_threads_current{cluster_id="${clusterId}"${brokerFilter}}`, start, end, step),
        () => queryVMInstant(`max(kafka_broker_jvm_threads_deadlocked{cluster_id="${clusterId}"${brokerFilter}})`),
        () => queryVMMulti(`kafka_broker_jvm_buffer_pool_used_bytes{cluster_id="${clusterId}"${brokerFilter}}`, start, end, step),
      ]

      // 分批执行，每批之间等待 100ms 避免限流
      const results: any[] = []
      for (let i = 0; i < queries.length; i += batchSize) {
        const batch = queries.slice(i, i + batchSize)
        const batchResults = await Promise.all(batch.map(q => q()))
        results.push(...batchResults)
        if (i + batchSize < queries.length) {
          await new Promise(resolve => setTimeout(resolve, 100))
        }
      }

      const [
        proRes, fetchRes, followerRes, lagRes, bytesInRes, bytesOutRes,
        queueTimeRes, localTimeRes, remoteTimeRes, throttleTimeRes, errorsRes,
        replicationInRes, replicationOutRes, reassignmentInRes, reassignmentOutRes,
        isrShrinksRes, isrExpandsRes, responseQueueRes, handlerIdleRes, networkIdleRes,
        diskReadRes, diskWriteRes, isrUpdatesFailedRes, controllerEventQueueRes, uncleanLeaderElectionsRes,
        delayedOpsRes, purgatorySizeRes, delayedFetchExpiresRes, minFetchRateRes, failedPartitionsRes,
        deadThreadRes, logFlushTimeRes, processCpuRes, processResMemRes, processVirtMemRes,
        processStartRes, processMaxFdsRes, processOpenFdsRes,
        cleanerMaxDirtyRes, cleanerTimeSinceRes, cleanerUncleanableBytesRes, cleanerUncleanablePartRes,
        cleanerRecopyRes, cleanerDeadThreadsRes, cleanerMaxBufRes, cleanerMaxCleanRes, cleanerMaxCompactRes,
        jvmGcSumRes, jvmGcCountRes, jvmMemPoolRes, jvmThreadsRes, jvmDeadlockedRes, jvmBufPoolRes
      ] = results

      // 请求延迟数据
      setBrokerRequestLatencyData({
        produce: proRes,
        fetchConsumer: fetchRes,
        fetchFollower: followerRes,
      })

      // 副本 Lag（取所有 broker 的 max）
      if (selectedBroker === 'all' && lagRes.brokers && Object.keys(lagRes.brokers).length > 0) {
        // 合并所有 broker 的 lag，取每个时间点的最大值
        const allTimes = new Set<string>()
        const lagBrokers = lagRes.brokers as Record<string, { times: string[], values: number[] }>
        Object.values(lagBrokers).forEach((b: { times: string[], values: number[] }) => b.times.forEach(t => allTimes.add(t)))
        const times = Array.from(allTimes).sort()
        const values = times.map(t => {
          let maxVal = 0
          Object.values(lagBrokers).forEach((b: { times: string[], values: number[] }) => {
            const idx = b.times.indexOf(t)
            if (idx >= 0 && b.values[idx] > maxVal) maxVal = b.values[idx]
          })
          return maxVal
        })
        setBrokerReplicaLagData({ times, values })
      } else {
        setBrokerReplicaLagData(lagRes.single || { times: [], values: [] })
      }

      // 字节流入/流出
      setBrokerBytesInData(bytesInRes)
      setBrokerBytesOutData(bytesOutRes)

      // 批次2新增：延迟分解数据
      setBrokerQueueTimeData(queueTimeRes)
      setBrokerLocalTimeData(localTimeRes)
      setBrokerRemoteTimeData(remoteTimeRes)
      setBrokerThrottleTimeData(throttleTimeRes)
      
      // 请求错误速率：按 request+error 组合分组
      setBrokerErrorsData(errorsRes || {})

      // 批次3新增：额外流量数据
      setBrokerReplicationInData(replicationInRes)
      setBrokerReplicationOutData(replicationOutRes)
      setBrokerReassignmentInData(reassignmentInRes)
      setBrokerReassignmentOutData(reassignmentOutRes)

      // 批次5新增：ISR 收缩/扩展数据
      setBrokerIsrShrinksData(isrShrinksRes)
      setBrokerIsrExpandsData(isrExpandsRes)

      // 批次6新增：网络/线程数据
      setBrokerResponseQueueData(responseQueueRes)
      setBrokerHandlerIdleData(handlerIdleRes)
      setBrokerNetworkIdleData(networkIdleRes)

      // 批次7新增：磁盘读写数据
      setBrokerDiskReadData(diskReadRes)
      setBrokerDiskWriteData(diskWriteRes)

      // 新增：ISR 更新失败总数
      setBrokerIsrUpdatesFailed(isrUpdatesFailedRes || 0)

      // 新增：Controller 事件
      setBrokerControllerEventQueueData(controllerEventQueueRes)
      setBrokerUncleanLeaderElections(uncleanLeaderElectionsRes || 0)

      // 新增：延迟操作
      setBrokerDelayedOperationsData(delayedOpsRes)
      setBrokerPurgatorySizeData(purgatorySizeRes)
      setBrokerDelayedFetchExpires(delayedFetchExpiresRes || 0)

      // 新增：Replica Fetcher
      setBrokerMinFetchRateData(minFetchRateRes)
      setBrokerFailedPartitionsCount(failedPartitionsRes || 0)
      setBrokerDeadThreadCount(deadThreadRes || 0)

      // 新增：Log Flush
      setBrokerLogFlushTimeData(logFlushTimeRes)

      // 新增：系统进程
      setBrokerProcessCpuData(processCpuRes)
      setBrokerProcessResidentMemoryData(processResMemRes)
      setBrokerProcessVirtualMemoryData(processVirtMemRes)
      setBrokerProcessStartTime(processStartRes || 0)
      setBrokerProcessMaxFds(processMaxFdsRes || 0)
      setBrokerProcessOpenFdsData(processOpenFdsRes)

      // 新增：Log Cleaner
      setBrokerLogCleanerMaxDirtyData(cleanerMaxDirtyRes)
      setBrokerLogCleanerTimeSinceLastRunData(cleanerTimeSinceRes)
      setBrokerLogCleanerUncleanableBytesData(cleanerUncleanableBytesRes)
      setBrokerLogCleanerUncleanablePartitions(cleanerUncleanablePartRes || 0)
      setBrokerLogCleanerRecopyData(cleanerRecopyRes)
      setBrokerLogCleanerDeadThreads(cleanerDeadThreadsRes || 0)
      setBrokerLogCleanerMaxBufferData(cleanerMaxBufRes)
      setBrokerLogCleanerMaxCleanTimeData(cleanerMaxCleanRes)
      setBrokerLogCleanerMaxCompactionDelayData(cleanerMaxCompactRes)

      // 新增：JVM
      setBrokerJvmGcData(jvmGcSumRes)
      setBrokerJvmGcCountData(jvmGcCountRes)
      setBrokerJvmMemoryPoolData(jvmMemPoolRes)
      setBrokerJvmThreadsData(jvmThreadsRes)
      setBrokerJvmDeadlockedThreads(jvmDeadlockedRes || 0)
      setBrokerJvmBufferPoolData(jvmBufPoolRes)

    } catch (error) {
      console.error('Failed to load broker chart data', error)
    } finally {
      setBrokerChartLoading(false)
    }
  }

  // 查询多 series 的 VM 数据（按 broker 分组）
  const queryVMMulti = async (query: string, start: Dayjs, end: Dayjs, step: string): Promise<any> => {
    try {
      const res = await axios.get<VMQueryResponse>('/metrics/history', {
        params: { query, start: start.unix(), end: end.unix(), step }
      })
      if (res.data.status !== 'success') return { single: { times: [], values: [] }, brokers: {} }

      const results = res.data.data.result
      if (results.length === 0) return { single: { times: [], values: [] }, brokers: {} }
      if (results.length === 1) {
        return {
          single: {
            times: results[0].values.map((v: [number, string]) => dayjs.unix(v[0]).format('HH:mm')),
            values: results[0].values.map((v: [number, string]) => parseFloat(v[1]) || 0),
          },
          brokers: {}
        }
      }

      // 多 series：按 broker 分组
      const brokers: Record<string, { times: string[], values: number[] }> = {}
      results.forEach(r => {
        const brokerId = r.metric.broker_id || 'unknown'
        brokers[brokerId] = {
          times: r.values.map((v: [number, string]) => dayjs.unix(v[0]).format('HH:mm')),
          values: r.values.map((v: [number, string]) => parseFloat(v[1]) || 0),
        }
      })
      return { single: null, brokers }
    } catch (error) {
      console.error('VM multi query failed:', query, error)
      return { single: { times: [], values: [] }, brokers: {} }
    }
  }

  // 查询错误速率数据（按 request+error 组合分组）
  const queryVMErrorRate = async (query: string, start: Dayjs, end: Dayjs, step: string): Promise<Record<string, { times: string[], values: number[] }>> => {
    try {
      const res = await axios.get<VMQueryResponse>('/metrics/history', {
        params: { query, start: start.unix(), end: end.unix(), step }
      })
      if (res.data.status !== 'success') return {}

      const results = res.data.data.result
      if (results.length === 0) return {}

      // 按 request+error 组合分组
      const errorGroups: Record<string, { times: string[], values: number[] }> = {}
      results.forEach(r => {
        const request = r.metric.request || 'unknown'
        const error = r.metric.error || 'unknown'
        const key = `${request}/${error}`
        errorGroups[key] = {
          times: r.values.map((v: [number, string]) => dayjs.unix(v[0]).format('HH:mm')),
          values: r.values.map((v: [number, string]) => parseFloat(v[1]) || 0),
        }
      })
      return errorGroups
    } catch (error) {
      console.error('VM error rate query failed:', query, error)
      return {}
    }
  }

  // 加载分区级别的指标
  const loadPartitionMetrics = async () => {
    if (!selectedCluster || !selectedTopic) return
    
    setTopicLoading(true)
    try {
      const { start, end, step } = getTimeRange()
      const clusterId = selectedCluster.cluster_id

      // 查询 Topic 分区级别的生产速率
      const produceRes = await axios.get<VMQueryResponse>('/metrics/history', {
        params: {
          query: `rate(kafka_topic_partition_current_offset{cluster_id="${clusterId}",topic="${selectedTopic}"}[30s])`,
          start: start.unix(),
          end: end.unix(),
          step
        }
      })

      // 查询消费组分区级别的消费速率和 Lag
      let consumeRes = { data: { status: 'success', data: { result: [] as any[] } } }
      let lagRes = { data: { status: 'success', data: { result: [] as any[] } } }
      
      if (selectedConsumerGroup) {
        consumeRes = await axios.get<VMQueryResponse>('/metrics/history', {
          params: {
            query: `rate(kafka_consumergroup_current_offset{cluster_id="${clusterId}",topic="${selectedTopic}",consumergroup="${selectedConsumerGroup}"}[30s])`,
            start: start.unix(),
            end: end.unix(),
            step
          }
        })

        lagRes = await axios.get<VMQueryResponse>('/metrics/history', {
          params: {
            query: `kafka_consumergroup_lag{cluster_id="${clusterId}",topic="${selectedTopic}",consumergroup="${selectedConsumerGroup}"}`,
            start: start.unix(),
            end: end.unix(),
            step
          }
        })
      }

      // 解析分区指标（按分区去重，保留最后一个）
      const parsePartitionMetrics = (result: any[]): PartitionMetric[] => {
        const partitionMap = new Map<number, PartitionMetric>()
        result.forEach(r => {
          const partition = parseInt(r.metric.partition || '0')
          partitionMap.set(partition, {
            partition,
            values: r.values.map((v: [number, string]) => ({
              time: dayjs.unix(v[0]).format('HH:mm'),
              value: parseFloat(v[1]) || 0
            }))
          })
        })
        return Array.from(partitionMap.values())
      }

      setPartitionMetrics({
        produceRate: parsePartitionMetrics(produceRes.data.data.result),
        consumeRate: parsePartitionMetrics(consumeRes.data.data.result),
        lag: parsePartitionMetrics(lagRes.data.data.result)
      })

      // 批次8新增：查询 Topic 分区级 JMX 指标
      const [logSizeRes, logEndOffsetRes, isrCountRes, replicaCountRes, underReplicatedRes] = await Promise.all([
        axios.get<VMQueryResponse>('/metrics/history', {
          params: {
            query: `kafka_topic_log_size{cluster_id="${clusterId}",topic="${selectedTopic}"}`,
            start: start.unix(), end: end.unix(), step
          }
        }),
        axios.get<VMQueryResponse>('/metrics/history', {
          params: {
            query: `kafka_topic_log_end_offset{cluster_id="${clusterId}",topic="${selectedTopic}"}`,
            start: start.unix(), end: end.unix(), step
          }
        }),
        axios.get<VMQueryResponse>('/metrics/history', {
          params: {
            query: `kafka_topic_partition_isr_count{cluster_id="${clusterId}",topic="${selectedTopic}"}`,
            start: start.unix(), end: end.unix(), step
          }
        }),
        axios.get<VMQueryResponse>('/metrics/history', {
          params: {
            query: `kafka_topic_partition_replica_count{cluster_id="${clusterId}",topic="${selectedTopic}"}`,
            start: start.unix(), end: end.unix(), step
          }
        }),
        axios.get<VMQueryResponse>('/metrics/history', {
          params: {
            query: `sum(kafka_topic_partition_under_replicated{cluster_id="${clusterId}",topic="${selectedTopic}"})`,
            start: start.unix(), end: end.unix(), step
          }
        }),
      ])

      setTopicLogSizeData(parsePartitionMetrics(logSizeRes.data.data.result))
      setTopicLogEndOffsetData(parsePartitionMetrics(logEndOffsetRes.data.data.result))
      setTopicIsrVsReplicaData({
        isr: parsePartitionMetrics(isrCountRes.data.data.result),
        replica: parsePartitionMetrics(replicaCountRes.data.data.result),
      })

      // Under Replicated 分区数（取最新值）
      const underReplicatedResult = underReplicatedRes.data.data.result
      if (underReplicatedResult.length > 0) {
        const lastValues = underReplicatedResult[0].values
        if (lastValues.length > 0) {
          setTopicUnderReplicatedCount(parseFloat(lastValues[lastValues.length - 1][1]) || 0)
        }
      }

      // 获取所有分区号
      const allPartitions = new Set<number>()
      produceRes.data.data.result.forEach((r: any) => {
        if (r.metric.partition) {
          allPartitions.add(parseInt(r.metric.partition))
        }
      })
      if (selectedPartitions.length === 0) {
        setSelectedPartitions(Array.from(allPartitions).sort((a, b) => a - b))
      }
    } catch (error) {
      console.error('Failed to load partition metrics', error)
    } finally {
      setTopicLoading(false)
    }
  }

  // 从 VictoriaMetrics 查询历史数据
  const queryVM = async (query: string, start: Dayjs, end: Dayjs, step: string): Promise<Array<[number, string]>> => {
    try {
      const res = await axios.get<VMQueryResponse>('/metrics/history', {
        params: {
          query,
          start: start.unix(),
          end: end.unix(),
          step
        }
      })
      if (res.data.status === 'success' && res.data.data.result.length > 0) {
        return res.data.data.result[0].values
      }
      return []
    } catch (error) {
      console.error('VM query failed:', query, error)
      return []
    }
  }

  // 查询即时值
  const queryVMInstant = async (query: string): Promise<number> => {
    try {
      const res = await axios.get<VMQueryResponse>('/metrics/history', {
        params: {
          query,
          start: dayjs().subtract(1, 'minute').unix(),
          end: dayjs().unix(),
          step: '60s'
        }
      })
      if (res.data.status === 'success' && res.data.data.result.length > 0) {
        const values = res.data.data.result[0].values
        if (values.length > 0) {
          return parseFloat(values[values.length - 1][1]) || 0
        }
      }
      return 0
    } catch (error) {
      console.error('VM instant query failed:', query, error)
      return 0
    }
  }

  // 获取时间范围
  const getTimeRange = (): { start: Dayjs; end: Dayjs; step: string } => {
    let end: Dayjs
    let start: Dayjs
    let step: string

    if (timeRange === 'custom' && customRange) {
      start = customRange[0]
      end = customRange[1]
      const durationMinutes = end.diff(start, 'minute')
      // step 与数据采集间隔（30s）匹配，确保能获取足够数据点
      if (durationMinutes <= 30) {
        step = '30s'
      } else if (durationMinutes <= 120) {
        step = '1m'
      } else if (durationMinutes <= 360) {
        step = '2m'
      } else if (durationMinutes <= 1440) {
        step = '5m'
      } else {
        step = '10m'
      }
    } else {
      end = dayjs()
      switch (quickRange) {
        case '5m':
          start = end.subtract(5, 'minute')
          step = '30s'
          break
        case '15m':
          start = end.subtract(15, 'minute')
          step = '30s'
          break
        case '30m':
          start = end.subtract(30, 'minute')
          step = '30s'
          break
        case '1h':
          start = end.subtract(1, 'hour')
          step = '1m'
          break
        case '3h':
          start = end.subtract(3, 'hour')
          step = '2m'
          break
        case '6h':
          start = end.subtract(6, 'hour')
          step = '2m'
          break
        case '12h':
          start = end.subtract(12, 'hour')
          step = '5m'
          break
        case '24h':
          start = end.subtract(24, 'hour')
          step = '5m'
          break
        case '2d':
          start = end.subtract(2, 'day')
          step = '10m'
          break
        case '7d':
          start = end.subtract(7, 'day')
          step = '30m'
          break
        case '30d':
          start = end.subtract(30, 'day')
          step = '1h'
          break
        default:
          start = end.subtract(1, 'hour')
          step = '1m'
      }
    }

    return { start, end, step }
  }

  const loadHistory = async () => {
    if (!selectedCluster) return
    
    try {
      const clusterId = selectedCluster.cluster_id.toString()
      const { start, end, step } = getTimeRange()
      
      // 1. 查询集群概览统计指标
      const [
        topicPartitionTotal,
        consumerGroupMemberCount,
        consumerGroupMemberTotal,
        isrTotal,
        nonPreferredLeaderCount,
        // 批次1新增指标
        activeBrokerCount,
        fencedBrokerCount,
        globalPartitionCount,
        globalTopicCount,
        preferredReplicaImbalance,
        // 新增：离线分区数、活跃 Controller 数量
        offlinePartitionsCount,
        activeControllerCount,
        // 新增：离线日志目录数、日志目录离线状态
        offlineLogDirectoryCount,
        logDirectoryOffline,
      ] = await Promise.all([
        queryVMInstant(`sum(kafka_topic_partitions{cluster_id="${clusterId}",topic!~"__.*"})`),
        queryVMInstant(`count(kafka_consumergroup_members{cluster_id="${clusterId}",consumergroup!~"__.*"})`),
        queryVMInstant(`sum(kafka_consumergroup_members{cluster_id="${clusterId}",consumergroup!~"__.*"})`),
        queryVMInstant(`sum(kafka_topic_partition_in_sync_replica{cluster_id="${clusterId}"})`),
        queryVMInstant(`count(kafka_topic_partition_leader_is_preferred{cluster_id="${clusterId}"}<1)`),
        // 批次1新增
        queryVMInstant(`max(kafka_broker_active_broker_count{cluster_id="${clusterId}"})`),
        queryVMInstant(`max(kafka_broker_fenced_broker_count{cluster_id="${clusterId}"})`),
        queryVMInstant(`max(kafka_broker_global_partition_count{cluster_id="${clusterId}"})`),
        queryVMInstant(`max(kafka_broker_global_topic_count{cluster_id="${clusterId}"})`),
        queryVMInstant(`max(kafka_broker_preferred_replica_imbalance{cluster_id="${clusterId}"})`),
        // 新增
        queryVMInstant(`max(kafka_broker_offline_partitions{cluster_id="${clusterId}"})`),
        queryVMInstant(`max(kafka_broker_active_controller{cluster_id="${clusterId}"})`),
        // 新增：离线日志目录数、日志目录离线状态
        queryVMInstant(`max(kafka_broker_offline_log_directory_count{cluster_id="${clusterId}"})`),
        queryVMInstant(`max(kafka_broker_log_directory_offline{cluster_id="${clusterId}"})`),
      ])

      setOverviewStats({
        topicPartitionTotal,
        consumerGroupMemberCount,
        consumerGroupMemberTotal,
        isrTotal,
        nonPreferredLeaderCount,
        activeBrokerCount,
        fencedBrokerCount,
        globalPartitionCount,
        globalTopicCount,
        preferredReplicaImbalance,
        offlinePartitionsCount,
        activeControllerCount,
        offlineLogDirectoryCount,
        logDirectoryOffline,
      })

      // 2. 查询趋势数据
      const [produceRateRes, consumeRateRes, lagRes, bytesInRes, bytesOutRes, bytesRejectedRes, failedProduceRes, failedFetchRes, invalidMagicRes, invalidCrcRes, invalidOffsetRes, messagesInRes] = await Promise.all([
        queryVM(`sum(rate(kafka_topic_partition_current_offset{cluster_id="${clusterId}",topic!~"__.*"}[30s]))`, start, end, step),
        queryVM(`sum(rate(kafka_consumergroup_current_offset{cluster_id="${clusterId}"}[30s]))`, start, end, step),
        queryVM(`sum(kafka_consumergroup_lag_sum{cluster_id="${clusterId}"})`, start, end, step),
        queryVM(`sum(rate(kafka_broker_bytes_in_total{cluster_id="${clusterId}"}[30s]))`, start, end, step),
        queryVM(`sum(rate(kafka_broker_bytes_out_total{cluster_id="${clusterId}"}[30s]))`, start, end, step),
        queryVM(`sum(rate(kafka_broker_bytes_rejected_total{cluster_id="${clusterId}"}[30s]))`, start, end, step),
        // 批次4新增
        queryVM(`sum(rate(kafka_broker_failed_produce_requests_total{cluster_id="${clusterId}"}[30s]))`, start, end, step),
        queryVM(`sum(rate(kafka_broker_failed_fetch_requests_total{cluster_id="${clusterId}"}[30s]))`, start, end, step),
        queryVMInstant(`sum(kafka_broker_invalid_magic_number_records_total{cluster_id="${clusterId}"})`),
        queryVMInstant(`sum(kafka_broker_invalid_message_crc_records_total{cluster_id="${clusterId}"})`),
        queryVMInstant(`sum(kafka_broker_invalid_offset_or_sequence_records_total{cluster_id="${clusterId}"})`),
        // 新增：消息流入速率
        queryVM(`sum(rate(kafka_broker_messages_in_total{cluster_id="${clusterId}"}[30s]))`, start, end, step),
      ])

      const times = produceRateRes.map(v => dayjs.unix(v[0]).format('HH:mm'))

      setProduceRateData({
        times,
        values: produceRateRes.map(v => parseFloat(v[1]) || 0),
      })

      setConsumeRateData({
        times,
        values: consumeRateRes.map(v => parseFloat(v[1]) || 0),
      })

      setLagTrendData({
        times,
        values: lagRes.map(v => parseFloat(v[1]) || 0),
      })

      setBytesInData({
        times,
        values: bytesInRes.map(v => parseFloat(v[1]) || 0),
      })

      setBytesOutData({
        times,
        values: bytesOutRes.map(v => parseFloat(v[1]) || 0),
      })

      setBytesRejectedData({
        times,
        values: bytesRejectedRes.map(v => parseFloat(v[1]) || 0),
      })

      // 批次4新增：错误数据
      setFailedProduceRateData({
        times,
        values: failedProduceRes.map(v => parseFloat(v[1]) || 0),
      })

      setFailedFetchRateData({
        times,
        values: failedFetchRes.map(v => parseFloat(v[1]) || 0),
      })

      setDataCorruptionStats({
        invalidMagicNumber: invalidMagicRes,
        invalidCrc: invalidCrcRes,
        invalidOffset: invalidOffsetRes,
      })

      // 新增：消息流入速率数据
      setMessagesInRateData({
        times,
        values: messagesInRes.map(v => parseFloat(v[1]) || 0),
      })

    } catch (error) {
      console.error('Failed to load history', error)
    }
  }

  // 时间选择器组件
  const renderTimeSelector = () => (
    <Space wrap>
      {timeRange === 'quick' ? (
        <>
          <Button.Group>
            <Button size="small" type={quickRange === '5m' ? 'primary' : 'default'} onClick={() => setQuickRange('5m')}>5分钟</Button>
            <Button size="small" type={quickRange === '15m' ? 'primary' : 'default'} onClick={() => setQuickRange('15m')}>15分钟</Button>
            <Button size="small" type={quickRange === '30m' ? 'primary' : 'default'} onClick={() => setQuickRange('30m')}>30分钟</Button>
            <Button size="small" type={quickRange === '1h' ? 'primary' : 'default'} onClick={() => setQuickRange('1h')}>1小时</Button>
            <Button size="small" type={quickRange === '3h' ? 'primary' : 'default'} onClick={() => setQuickRange('3h')}>3小时</Button>
            <Button size="small" type={quickRange === '6h' ? 'primary' : 'default'} onClick={() => setQuickRange('6h')}>6小时</Button>
            <Button size="small" type={quickRange === '12h' ? 'primary' : 'default'} onClick={() => setQuickRange('12h')}>12小时</Button>
            <Button size="small" type={quickRange === '24h' ? 'primary' : 'default'} onClick={() => setQuickRange('24h')}>24小时</Button>
            <Button size="small" type={quickRange === '2d' ? 'primary' : 'default'} onClick={() => setQuickRange('2d')}>2天</Button>
            <Button size="small" type={quickRange === '7d' ? 'primary' : 'default'} onClick={() => setQuickRange('7d')}>7天</Button>
            <Button size="small" type={quickRange === '30d' ? 'primary' : 'default'} onClick={() => setQuickRange('30d')}>30天</Button>
          </Button.Group>
          <Button size="small" icon={<CalendarOutlined />} onClick={() => setTimeRange('custom')}>
            自定义
          </Button>
        </>
      ) : (
        <>
          <DatePicker.RangePicker
            size="small"
            showTime
            value={customRange}
            onChange={(dates) => {
              if (dates && dates[0] && dates[1]) {
                setCustomRange([dates[0], dates[1]])
              }
            }}
            presets={[
              { label: '最近1小时', value: [dayjs().subtract(1, 'hour'), dayjs()] },
              { label: '最近6小时', value: [dayjs().subtract(6, 'hour'), dayjs()] },
              { label: '最近24小时', value: [dayjs().subtract(24, 'hour'), dayjs()] },
              { label: '最近7天', value: [dayjs().subtract(7, 'day'), dayjs()] },
              { label: '最近30天', value: [dayjs().subtract(30, 'day'), dayjs()] },
            ]}
            style={{ width: 360 }}
          />
          <Button size="small" onClick={() => { setTimeRange('quick'); setCustomRange(null) }}>
            快速选择
          </Button>
        </>
      )}
    </Space>
  )

  // 折线图配置 - 生产速率
  const getProduceRateChartOption = () => ({
    title: { text: '集群生产速率', left: 'center', textStyle: { fontSize: 14 } },
    tooltip: { trigger: 'axis', formatter: (params: any) => `${params[0].axisValue}<br/>${params[0].value.toFixed(2)} msg/s` },
    grid: { left: '3%', right: '4%', bottom: '10%', top: '15%', containLabel: true },
    xAxis: { type: 'category', boundaryGap: false, data: produceRateData.times },
    yAxis: { type: 'value', name: 'msg/s' },
    series: [{
      type: 'line',
      smooth: true,
      data: produceRateData.values,
      itemStyle: { color: '#1890ff' },
      areaStyle: { opacity: 0.1 }
    }]
  })

  // 折线图配置 - 消费速率
  const getConsumeRateChartOption = () => ({
    title: { text: '集群消费速率', left: 'center', textStyle: { fontSize: 14 } },
    tooltip: { trigger: 'axis', formatter: (params: any) => `${params[0].axisValue}<br/>${params[0].value.toFixed(2)} msg/s` },
    grid: { left: '3%', right: '4%', bottom: '10%', top: '15%', containLabel: true },
    xAxis: { type: 'category', boundaryGap: false, data: consumeRateData.times },
    yAxis: { type: 'value', name: 'msg/s' },
    series: [{
      type: 'line',
      smooth: true,
      data: consumeRateData.values,
      itemStyle: { color: '#52c41a' },
      areaStyle: { opacity: 0.1 }
    }]
  })

  // 折线图配置 - 消费延迟
  const getLagTrendChartOption = () => ({
    title: { text: '消费者组总 Lag', left: 'center', textStyle: { fontSize: 14 } },
    tooltip: { trigger: 'axis', formatter: (params: any) => `${params[0].axisValue}<br/>${params[0].value.toLocaleString()}` },
    grid: { left: '3%', right: '4%', bottom: '10%', top: '15%', containLabel: true },
    xAxis: { type: 'category', boundaryGap: false, data: lagTrendData.times },
    yAxis: { type: 'value', name: 'Lag' },
    series: [{
      type: 'line',
      smooth: true,
      data: lagTrendData.values,
      itemStyle: { color: '#f5222d' },
      areaStyle: { opacity: 0.1 }
    }]
  })

  // 折线图配置 - 字节流入
  const getBytesInChartOption = () => ({
    title: { text: '字节流入速率', left: 'center', textStyle: { fontSize: 14 } },
    tooltip: { 
      trigger: 'axis', 
      formatter: (params: any) => `${params[0].axisValue}<br/>${formatBytesForChart(params[0].value)}`
    },
    grid: { left: '3%', right: '4%', bottom: '10%', top: '15%', containLabel: true },
    xAxis: { type: 'category', boundaryGap: false, data: bytesInData.times },
    yAxis: { type: 'value', name: 'bytes/s', axisLabel: { formatter: (value: number) => formatBytesForChart(value) } },
    series: [{
      type: 'line',
      smooth: true,
      data: bytesInData.values,
      itemStyle: { color: '#52c41a' },
      areaStyle: { opacity: 0.1 }
    }]
  })

  // 折线图配置 - 字节流出
  const getBytesOutChartOption = () => ({
    title: { text: '字节流出速率', left: 'center', textStyle: { fontSize: 14 } },
    tooltip: { 
      trigger: 'axis', 
      formatter: (params: any) => `${params[0].axisValue}<br/>${formatBytesForChart(params[0].value)}`
    },
    grid: { left: '3%', right: '4%', bottom: '10%', top: '15%', containLabel: true },
    xAxis: { type: 'category', boundaryGap: false, data: bytesOutData.times },
    yAxis: { type: 'value', name: 'bytes/s', axisLabel: { formatter: (value: number) => formatBytesForChart(value) } },
    series: [{
      type: 'line',
      smooth: true,
      data: bytesOutData.values,
      itemStyle: { color: '#faad14' },
      areaStyle: { opacity: 0.1 }
    }]
  })

  // 批次1新增：拒绝字节速率图表
  const getBytesRejectedChartOption = () => ({
    title: { text: '拒绝字节速率', left: 'center', textStyle: { fontSize: 14 } },
    tooltip: { 
      trigger: 'axis', 
      formatter: (params: any) => `${params[0].axisValue}<br/>${formatBytesForChart(params[0].value)}`
    },
    grid: { left: '3%', right: '4%', bottom: '10%', top: '15%', containLabel: true },
    xAxis: { type: 'category', boundaryGap: false, data: bytesRejectedData.times },
    yAxis: { type: 'value', name: 'bytes/s', axisLabel: { formatter: (value: number) => formatBytesForChart(value) } },
    series: [{
      type: 'line',
      smooth: true,
      data: bytesRejectedData.values,
      itemStyle: { color: '#f5222d' },
      areaStyle: { opacity: 0.1 }
    }]
  })

  // 批次4新增：失败生产请求率图表
  const getFailedProduceRateChartOption = () => ({
    title: { text: '生产请求失败率', left: 'center', textStyle: { fontSize: 14 } },
    tooltip: { 
      trigger: 'axis', 
      formatter: (params: any) => `${params[0].axisValue}<br/>${(params[0].value ?? 0).toFixed(2)} 次/秒`
    },
    grid: { left: '3%', right: '4%', bottom: '10%', top: '15%', containLabel: true },
    xAxis: { type: 'category', boundaryGap: false, data: failedProduceRateData.times },
    yAxis: { type: 'value', name: '次/秒' },
    series: [{
      type: 'line',
      smooth: true,
      data: failedProduceRateData.values,
      itemStyle: { color: '#f5222d' },
      areaStyle: { opacity: 0.1 }
    }]
  })

  // 批次4新增：失败拉取请求率图表
  const getFailedFetchRateChartOption = () => ({
    title: { text: '拉取请求失败率', left: 'center', textStyle: { fontSize: 14 } },
    tooltip: { 
      trigger: 'axis', 
      formatter: (params: any) => `${params[0].axisValue}<br/>${(params[0].value ?? 0).toFixed(2)} 次/秒`
    },
    grid: { left: '3%', right: '4%', bottom: '10%', top: '15%', containLabel: true },
    xAxis: { type: 'category', boundaryGap: false, data: failedFetchRateData.times },
    yAxis: { type: 'value', name: '次/秒' },
    series: [{
      type: 'line',
      smooth: true,
      data: failedFetchRateData.values,
      itemStyle: { color: '#faad14' },
      areaStyle: { opacity: 0.1 }
    }]
  })

  // 新增：消息流入速率图表
  const getMessagesInRateChartOption = () => ({
    title: { text: '消息流入速率', left: 'center', textStyle: { fontSize: 14 } },
    tooltip: { 
      trigger: 'axis', 
      formatter: (params: any) => `${params[0].axisValue}<br/>${(params[0].value ?? 0).toFixed(2)} msg/s`
    },
    grid: { left: '3%', right: '4%', bottom: '10%', top: '15%', containLabel: true },
    xAxis: { type: 'category', boundaryGap: false, data: messagesInRateData.times },
    yAxis: { type: 'value', name: 'msg/s' },
    series: [{
      type: 'line',
      smooth: true,
      data: messagesInRateData.values,
      itemStyle: { color: '#722ed1' },
      areaStyle: { opacity: 0.1 }
    }]
  })

  // 格式化字节用于图表
  const formatBytesForChart = (bytes: number): string => {
    if (!bytes || bytes === 0) return '0 B'
    if (bytes >= 1024 * 1024 * 1024) return (bytes / (1024 * 1024 * 1024)).toFixed(2) + ' GB'
    if (bytes >= 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(2) + ' MB'
    if (bytes >= 1024) return (bytes / 1024).toFixed(2) + ' KB'
    return bytes.toFixed(0) + ' B'
  }

  // 构建多 series 折线图配置
  const buildMultiSeriesChartOption = (
    title: string,
    data: Record<string, { times: string[], values: number[] }> | { times: string[], values: number[] } | undefined,
    _color: string,
    yAxisName: string,
    tooltipFormatter?: (value: number) => string,
  ) => {
    const colors = ['#1890ff', '#52c41a', '#faad14', '#f5222d', '#722ed1', '#13c2c2', '#eb2f96', '#fa8c16']
    // 安全处理：data 可能是 undefined 或格式不对
    let safeData: Record<string, { times: string[], values: number[] }> = {}
    if (data && typeof data === 'object') {
      // 检查是否是 { times, values } 格式（单个 series）
      if ('times' in data && 'values' in data) {
        safeData = { '0': data as { times: string[], values: number[] } }
      } else {
        safeData = data as Record<string, { times: string[], values: number[] }>
      }
    }
    const entries = Object.entries(safeData).filter(([, d]) => d && d.times && d.values)
    if (entries.length === 0) {
      return {
        title: { text: title, left: 'center', textStyle: { fontSize: 14, color: '#999' } },
        graphic: { type: 'text', left: 'center', top: 'middle', style: { text: '暂无数据', fill: '#999', fontSize: 14 } },
        xAxis: { type: 'category', data: [] },
        yAxis: { type: 'value' },
        series: []
      }
    }

    const allTimes = new Set<string>()
    entries.forEach(([, d]) => d.times.forEach(t => allTimes.add(t)))
    const times = Array.from(allTimes).sort()

    return {
      title: { text: title, left: 'center', textStyle: { fontSize: 14 } },
      tooltip: {
        trigger: 'axis',
        formatter: (params: any[]) => {
          if (!params || params.length === 0) return ''
          let html = params[0].axisValue + '<br/>'
          params.forEach((p: any) => {
            const val = tooltipFormatter ? tooltipFormatter(p.value) : (p.value?.toFixed(2) ?? '0')
            html += `${p.marker} Broker ${p.seriesName}: ${val}<br/>`
          })
          return html
        }
      },
      legend: { data: entries.map(([id]) => `Broker ${id}`), top: 25, type: 'scroll' },
      grid: { left: '3%', right: '4%', bottom: '3%', top: 60, containLabel: true },
      xAxis: { type: 'category', boundaryGap: false, data: times },
      yAxis: { type: 'value', name: yAxisName },
      series: entries.map(([id, d], index) => ({
        name: `Broker ${id}`,
        type: 'line',
        smooth: true,
        data: times.map(t => {
          const idx = d.times.indexOf(t)
          return idx >= 0 ? d.values[idx] : null
        }),
        itemStyle: { color: colors[index % colors.length] },
        connectNulls: true,
      }))
    }
  }

  // Broker - 请求延迟图表（单个 request 类型）
  const getBrokerLatencyChartOption = (title: string, data: any) => {
    if (!data || (!data.single && Object.keys(data.brokers || {}).length === 0)) {
      return {
        title: { text: title, left: 'center', textStyle: { fontSize: 14, color: '#999' } },
        graphic: { type: 'text', left: 'center', top: 'middle', style: { text: '暂无数据', fill: '#999', fontSize: 14 } },
        xAxis: { type: 'category', data: [] },
        yAxis: { type: 'value' },
        series: []
      }
    }
    if (data.single) {
      return {
        title: { text: title, left: 'center', textStyle: { fontSize: 14 } },
        tooltip: { trigger: 'axis', formatter: (params: any) => `${params[0].axisValue}<br/>${params[0].value?.toFixed(2) ?? 0} ms` },
        grid: { left: '3%', right: '4%', bottom: '10%', top: '15%', containLabel: true },
        xAxis: { type: 'category', boundaryGap: false, data: data.single.times },
        yAxis: { type: 'value', name: 'ms' },
        series: [{ type: 'line', smooth: true, data: data.single.values, itemStyle: { color: '#1890ff' }, areaStyle: { opacity: 0.1 } }]
      }
    }
    return buildMultiSeriesChartOption(title, data.brokers, '#1890ff', 'ms')
  }

  // Broker - 副本 Lag 图表
  const getBrokerReplicaLagChartOption = () => {
    if (brokerReplicaLagData.times.length === 0) {
      return {
        title: { text: '副本同步 Lag', left: 'center', textStyle: { fontSize: 14, color: '#999' } },
        graphic: { type: 'text', left: 'center', top: 'middle', style: { text: '暂无数据', fill: '#999', fontSize: 14 } },
        xAxis: { type: 'category', data: [] },
        yAxis: { type: 'value' },
        series: []
      }
    }
    return {
      title: { text: '副本同步 Lag', left: 'center', textStyle: { fontSize: 14 } },
      tooltip: { trigger: 'axis', formatter: (params: any) => `${params[0].axisValue}<br/>${(params[0].value ?? 0).toLocaleString()}` },
      grid: { left: '3%', right: '4%', bottom: '10%', top: '15%', containLabel: true },
      xAxis: { type: 'category', boundaryGap: false, data: brokerReplicaLagData.times },
      yAxis: { type: 'value', name: 'Lag' },
      series: [{ type: 'line', smooth: true, data: brokerReplicaLagData.values, itemStyle: { color: '#f5222d' }, areaStyle: { opacity: 0.1 } }]
    }
  }

  // Topic 监控 - 生产速率折线图（按分区）
  const getTopicProduceRateChartOption = () => {
    const colors = ['#1890ff', '#52c41a', '#faad14', '#f5222d', '#722ed1', '#13c2c2', '#eb2f96', '#fa8c16']
    const filteredMetrics = partitionMetrics.produceRate.filter(p => selectedPartitions.includes(p.partition) && p.values.length > 0)
    
    if (filteredMetrics.length === 0) {
      return {}
    }

    // 使用第一个分区的时间作为基准
    const times = filteredMetrics[0]?.values.map(v => v.time) || []
    
    return {
      title: { text: 'Topic 生产速率（按分区）', left: 'center', textStyle: { fontSize: 14 } },
      tooltip: { 
        trigger: 'axis',
        formatter: (params: any[]) => {
          if (!params || params.length === 0) return ''
          let html = params[0].axisValue + '<br/>'
          params.filter(p => p.value !== undefined && p.value !== null).forEach(p => {
            html += `${p.marker} 分区${p.seriesName}: ${p.value.toFixed(2)} msg/s<br/>`
          })
          return html
        }
      },
      legend: { 
        data: filteredMetrics.map(p => `分区${p.partition}`),
        top: 25,
        type: 'scroll'
      },
      grid: { left: '3%', right: '4%', bottom: '3%', top: 60, containLabel: true },
      xAxis: { type: 'category', boundaryGap: false, data: times },
      yAxis: { type: 'value', name: 'msg/s' },
      series: filteredMetrics.map((p, index) => ({
        name: p.partition.toString(),
        type: 'line',
        smooth: true,
        data: p.values.map(v => v.value),
        itemStyle: { color: colors[index % colors.length] },
        emphasis: { focus: 'series' }
      }))
    }
  }

  // Topic 监控 - 消费速率折线图（按分区）
  const getTopicConsumeRateChartOption = () => {
    const colors = ['#52c41a', '#1890ff', '#faad14', '#f5222d', '#722ed1', '#13c2c2', '#eb2f96', '#fa8c16']
    const filteredMetrics = partitionMetrics.consumeRate.filter(p => selectedPartitions.includes(p.partition) && p.values.length > 0)
    
    if (filteredMetrics.length === 0 || !selectedConsumerGroup) {
      return {
        title: { text: '消费组消费速率（按分区）', left: 'center', textStyle: { fontSize: 14, color: '#999' } },
        graphic: {
          type: 'text',
          left: 'center',
          top: 'middle',
          style: { text: selectedConsumerGroup ? '暂无数据' : '请选择消费组', fill: '#999', fontSize: 14 }
        },
        xAxis: { type: 'category', data: [] },
        yAxis: { type: 'value' },
        series: []
      }
    }

    const times = filteredMetrics[0]?.values.map(v => v.time) || []
    
    return {
      title: { text: `消费速率（按分区）`, left: 'center', textStyle: { fontSize: 14 } },
      tooltip: { 
        trigger: 'axis',
        formatter: (params: any[]) => {
          if (!params || params.length === 0) return ''
          let html = params[0].axisValue + '<br/>'
          params.filter(p => p.value !== undefined && p.value !== null).forEach(p => {
            html += `${p.marker} 分区${p.seriesName}: ${p.value.toFixed(2)} msg/s<br/>`
          })
          return html
        }
      },
      legend: { 
        data: filteredMetrics.map(p => `分区${p.partition}`),
        top: 25,
        type: 'scroll'
      },
      grid: { left: '3%', right: '4%', bottom: '3%', top: 60, containLabel: true },
      xAxis: { type: 'category', boundaryGap: false, data: times },
      yAxis: { type: 'value', name: 'msg/s' },
      series: filteredMetrics.map((p, index) => ({
        name: p.partition.toString(),
        type: 'line',
        smooth: true,
        data: p.values.map(v => v.value),
        itemStyle: { color: colors[index % colors.length] },
        emphasis: { focus: 'series' }
      }))
    }
  }

  // Topic 监控 - Lag 折线图（按分区）
  const getTopicLagChartOption = () => {
    const colors = ['#f5222d', '#faad14', '#1890ff', '#52c41a', '#722ed1', '#13c2c2', '#eb2f96', '#fa8c16']
    const filteredMetrics = partitionMetrics.lag.filter(p => selectedPartitions.includes(p.partition) && p.values.length > 0)
    
    if (filteredMetrics.length === 0 || !selectedConsumerGroup) {
      return {
        title: { text: '消费组 Lag（按分区）', left: 'center', textStyle: { fontSize: 14, color: '#999' } },
        graphic: {
          type: 'text',
          left: 'center',
          top: 'middle',
          style: { text: selectedConsumerGroup ? '暂无数据' : '请选择消费组', fill: '#999', fontSize: 14 }
        },
        xAxis: { type: 'category', data: [] },
        yAxis: { type: 'value' },
        series: []
      }
    }

    const times = filteredMetrics[0]?.values.map(v => v.time) || []
    
    return {
      title: { text: `Lag（按分区）`, left: 'center', textStyle: { fontSize: 14 } },
      tooltip: { 
        trigger: 'axis',
        formatter: (params: any[]) => {
          if (!params || params.length === 0) return ''
          let html = params[0].axisValue + '<br/>'
          params.filter(p => p.value !== undefined && p.value !== null).forEach(p => {
            html += `${p.marker} 分区${p.seriesName}: ${p.value.toLocaleString()}<br/>`
          })
          return html
        }
      },
      legend: { 
        data: filteredMetrics.map(p => `分区${p.partition}`),
        top: 25,
        type: 'scroll'
      },
      grid: { left: '3%', right: '4%', bottom: '3%', top: 60, containLabel: true },
      xAxis: { type: 'category', boundaryGap: false, data: times },
      yAxis: { type: 'value', name: 'Lag' },
      series: filteredMetrics.map((p, index) => ({
        name: p.partition.toString(),
        type: 'line',
        smooth: true,
        data: p.values.map(v => v.value),
        itemStyle: { color: colors[index % colors.length] },
        emphasis: { focus: 'series' }
      }))
    }
  }

  // 批次8新增：Topic 日志大小图表
  const getTopicLogSizeChartOption = () => {
    const colors = ['#1890ff', '#52c41a', '#faad14', '#f5222d', '#722ed1', '#13c2c2', '#eb2f96', '#fa8c16']
    const filteredMetrics = topicLogSizeData.filter(p => selectedPartitions.includes(p.partition) && p.values.length > 0)
    
    if (filteredMetrics.length === 0) {
      return {
        title: { text: 'Topic 日志大小（按分区）', left: 'center', textStyle: { fontSize: 14, color: '#999' } },
        graphic: { type: 'text', left: 'center', top: 'middle', style: { text: '暂无数据', fill: '#999', fontSize: 14 } },
        xAxis: { type: 'category', data: [] },
        yAxis: { type: 'value' },
        series: []
      }
    }

    const times = filteredMetrics[0]?.values.map(v => v.time) || []
    
    return {
      title: { text: 'Topic 日志大小（按分区）', left: 'center', textStyle: { fontSize: 14 } },
      tooltip: { 
        trigger: 'axis',
        formatter: (params: any[]) => {
          if (!params || params.length === 0) return ''
          let html = params[0].axisValue + '<br/>'
          params.filter(p => p.value !== undefined && p.value !== null).forEach(p => {
            html += `${p.marker} 分区${p.seriesName}: ${formatBytesForChart(p.value)}<br/>`
          })
          return html
        }
      },
      legend: { data: filteredMetrics.map(p => `分区${p.partition}`), top: 25, type: 'scroll' },
      grid: { left: '3%', right: '4%', bottom: '3%', top: 60, containLabel: true },
      xAxis: { type: 'category', boundaryGap: false, data: times },
      yAxis: { type: 'value', name: 'bytes', axisLabel: { formatter: (value: number) => formatBytesForChart(value) } },
      series: filteredMetrics.map((p, index) => ({
        name: p.partition.toString(),
        type: 'line',
        smooth: true,
        data: p.values.map(v => v.value),
        itemStyle: { color: colors[index % colors.length] },
        emphasis: { focus: 'series' }
      }))
    }
  }

  // 批次8新增：Topic LogEndOffset 图表
  const getTopicLogEndOffsetChartOption = () => {
    const colors = ['#1890ff', '#52c41a', '#faad14', '#f5222d', '#722ed1', '#13c2c2', '#eb2f96', '#fa8c16']
    const filteredMetrics = topicLogEndOffsetData.filter(p => selectedPartitions.includes(p.partition) && p.values.length > 0)
    
    if (filteredMetrics.length === 0) {
      return {
        title: { text: 'Topic LogEndOffset（按分区）', left: 'center', textStyle: { fontSize: 14, color: '#999' } },
        graphic: { type: 'text', left: 'center', top: 'middle', style: { text: '暂无数据', fill: '#999', fontSize: 14 } },
        xAxis: { type: 'category', data: [] },
        yAxis: { type: 'value' },
        series: []
      }
    }

    const times = filteredMetrics[0]?.values.map(v => v.time) || []
    
    return {
      title: { text: 'Topic LogEndOffset（按分区）', left: 'center', textStyle: { fontSize: 14 } },
      tooltip: { trigger: 'axis' },
      legend: { data: filteredMetrics.map(p => `分区${p.partition}`), top: 25, type: 'scroll' },
      grid: { left: '3%', right: '4%', bottom: '3%', top: 60, containLabel: true },
      xAxis: { type: 'category', boundaryGap: false, data: times },
      yAxis: { type: 'value', name: 'Offset' },
      series: filteredMetrics.map((p, index) => ({
        name: p.partition.toString(),
        type: 'line',
        smooth: true,
        data: p.values.map(v => v.value),
        itemStyle: { color: colors[index % colors.length] },
        emphasis: { focus: 'series' }
      }))
    }
  }

  // 批次8新增：ISR vs 副本数对比图
  const getTopicIsrVsReplicaChartOption = () => {
    if (topicIsrVsReplicaData.isr.length === 0 && topicIsrVsReplicaData.replica.length === 0) {
      return {
        title: { text: '分区 ISR 数 vs 副本数', left: 'center', textStyle: { fontSize: 14, color: '#999' } },
        graphic: { type: 'text', left: 'center', top: 'middle', style: { text: '暂无数据', fill: '#999', fontSize: 14 } },
        xAxis: { type: 'category', data: [] },
        yAxis: { type: 'value' },
        series: []
      }
    }

    // 合并所有分区号
    const allPartitions = new Set<number>()
    topicIsrVsReplicaData.isr.forEach(p => allPartitions.add(p.partition))
    topicIsrVsReplicaData.replica.forEach(p => allPartitions.add(p.partition))
    const sortedPartitions = Array.from(allPartitions).sort((a, b) => a - b)

    // 获取最新值
    const isrValues = sortedPartitions.map(p => {
      const metric = topicIsrVsReplicaData.isr.find(m => m.partition === p)
      if (metric && metric.values.length > 0) {
        return metric.values[metric.values.length - 1].value
      }
      return 0
    })

    const replicaValues = sortedPartitions.map(p => {
      const metric = topicIsrVsReplicaData.replica.find(m => m.partition === p)
      if (metric && metric.values.length > 0) {
        return metric.values[metric.values.length - 1].value
      }
      return 0
    })

    return {
      title: { text: '分区 ISR 数 vs 副本数', left: 'center', textStyle: { fontSize: 14 } },
      tooltip: { trigger: 'axis' },
      legend: { data: ['ISR 数', '副本数'], top: 25 },
      grid: { left: '3%', right: '4%', bottom: '3%', top: 60, containLabel: true },
      xAxis: { type: 'category', data: sortedPartitions.map(p => `分区${p}`) },
      yAxis: { type: 'value', name: '个数' },
      series: [
        {
          name: 'ISR 数',
          type: 'bar',
          data: isrValues,
          itemStyle: { color: '#52c41a' },
        },
        {
          name: '副本数',
          type: 'bar',
          data: replicaValues,
          itemStyle: { color: '#1890ff' },
        }
      ]
    }
  }

  // Topic 监控 - 总生产速率折线图（汇总所有分区）
  const getTopicTotalProduceRateChartOption = () => {
    const allTimes = new Set<string>()
    partitionMetrics.produceRate.forEach(p => p.values.forEach(v => allTimes.add(v.time)))
    const times = Array.from(allTimes).sort()
    
    // 计算每个时间点的总速率
    const totalValues = times.map(t => {
      let sum = 0
      partitionMetrics.produceRate.forEach(p => {
        const found = p.values.find(v => v.time === t)
        if (found) sum += found.value
      })
      return sum
    })
    
    return {
      title: { text: 'Topic 生产速率', left: 'center', textStyle: { fontSize: 14 } },
      tooltip: { 
        trigger: 'axis',
        formatter: (params: any) => `${params[0].axisValue}<br/>${(params[0].value || 0).toFixed(2)} msg/s`
      },
      grid: { left: '3%', right: '4%', bottom: '10%', top: '15%', containLabel: true },
      xAxis: { type: 'category', boundaryGap: false, data: times },
      yAxis: { type: 'value', name: 'msg/s' },
      series: [{
        type: 'line',
        smooth: true,
        data: totalValues,
        itemStyle: { color: '#1890ff' },
        areaStyle: { opacity: 0.1 }
      }]
    }
  }

  // Topic 监控 - 总消费速率折线图（汇总所有分区）
  const getTopicTotalConsumeRateChartOption = () => {
    // 检查是否有选中的消费组和该消费组是否有数据
    const hasConsumeData = partitionMetrics.consumeRate.some(p => p.values.length > 0)
    
    if (!selectedConsumerGroup) {
      return {
        title: { text: '消费组消费速率', left: 'center', textStyle: { fontSize: 14, color: '#999' } },
        graphic: {
          type: 'text',
          left: 'center',
          top: 'middle',
          style: { text: '请选择消费组', fill: '#999', fontSize: 14 }
        },
        xAxis: { type: 'category', data: [] },
        yAxis: { type: 'value' },
        series: []
      }
    }

    if (!hasConsumeData) {
      return {
        title: { text: '消费组消费速率', left: 'center', textStyle: { fontSize: 14, color: '#999' } },
        graphic: {
          type: 'text',
          left: 'center',
          top: 'middle',
          style: { text: '该消费组未消费此 Topic', fill: '#999', fontSize: 14 }
        },
        xAxis: { type: 'category', data: [] },
        yAxis: { type: 'value' },
        series: []
      }
    }

    const allTimes = new Set<string>()
    partitionMetrics.consumeRate.forEach(p => p.values.forEach(v => allTimes.add(v.time)))
    const times = Array.from(allTimes).sort()
    
    // 计算每个时间点的总速率
    const totalValues = times.map(t => {
      let sum = 0
      partitionMetrics.consumeRate.forEach(p => {
        const found = p.values.find(v => v.time === t)
        if (found) sum += found.value
      })
      return sum
    })
    
    return {
      title: { text: `消费速率 (${selectedConsumerGroup})`, left: 'center', textStyle: { fontSize: 14 } },
      tooltip: {
        trigger: 'axis',
        formatter: (params: any) => `${params[0].axisValue}<br/>${(params[0].value || 0).toFixed(2)} msg/s`
      },
      graphic: [], // 清除 graphic
      grid: { left: '3%', right: '4%', bottom: '10%', top: '15%', containLabel: true },
      xAxis: { type: 'category', boundaryGap: false, data: times },
      yAxis: { type: 'value', name: 'msg/s' },
      series: [{
        type: 'line',
        smooth: true,
        data: totalValues,
        itemStyle: { color: '#52c41a' },
        areaStyle: { opacity: 0.1 }
      }]
    }
  }

  // Topic 监控 - 总 Lag 折线图（汇总所有分区）
  const getTopicTotalLagChartOption = () => {
    // 检查是否有选中的消费组和该消费组是否有数据
    const hasLagData = partitionMetrics.lag.some(p => p.values.length > 0)

    if (!selectedConsumerGroup) {
      return {
        title: { text: '消费组 Lag', left: 'center', textStyle: { fontSize: 14, color: '#999' } },
        graphic: {
          type: 'text',
          left: 'center',
          top: 'middle',
          style: { text: '请选择消费组', fill: '#999', fontSize: 14 }
        },
        xAxis: { type: 'category', data: [] },
        yAxis: { type: 'value' },
        series: []
      }
    }

    if (!hasLagData) {
      return {
        title: { text: '消费组 Lag', left: 'center', textStyle: { fontSize: 14, color: '#999' } },
        graphic: {
          type: 'text',
          left: 'center',
          top: 'middle',
          style: { text: '该消费组未消费此 Topic', fill: '#999', fontSize: 14 }
        },
        xAxis: { type: 'category', data: [] },
        yAxis: { type: 'value' },
        series: []
      }
    }

    const allTimes = new Set<string>()
    partitionMetrics.lag.forEach(p => p.values.forEach(v => allTimes.add(v.time)))
    const times = Array.from(allTimes).sort()
    
    // 计算每个时间点的总 Lag
    const totalValues = times.map(t => {
      let sum = 0
      partitionMetrics.lag.forEach(p => {
        const found = p.values.find(v => v.time === t)
        if (found) sum += found.value
      })
      return sum
    })
    
    return {
      title: { text: `Lag (${selectedConsumerGroup})`, left: 'center', textStyle: { fontSize: 14 } },
      tooltip: {
        trigger: 'axis',
        formatter: (params: any) => `${params[0].axisValue}<br/>${(params[0].value || 0).toLocaleString()}`
      },
      graphic: [], // 清除 graphic
      grid: { left: '3%', right: '4%', bottom: '10%', top: '15%', containLabel: true },
      xAxis: { type: 'category', boundaryGap: false, data: times },
      yAxis: { type: 'value', name: 'Lag' },
      series: [{
        type: 'line',
        smooth: true,
        data: totalValues,
        itemStyle: { color: '#f5222d' },
        areaStyle: { opacity: 0.1 }
      }]
    }
  }

  const tabItems = [
    {
      key: 'overview',
      label: '集群概览',
      children: (
        <>
          {/* 告警提示 */}
          {!metrics?.jmx_exporter_available && (
            <Alert
              message="JMX Exporter 未配置或不可用"
              description="请在集群配置中设置 JMX Exporter URL 以获取 Broker 指标"
              type="warning"
              showIcon
              style={{ marginBottom: 16 }}
            />
          )}
          {!metrics?.kafka_exporter_available && (
            <Alert
              message="Kafka 连接失败"
              description="无法连接到 Kafka 集群获取消费者组信息"
              type="error"
              showIcon
              style={{ marginBottom: 16 }}
            />
          )}

          <DashboardGrid
            storageKey="cluster-overview"
            cols={{ lg: 12, md: 12, sm: 6, xs: 4 }}
            rowHeight={50}
            items={[
              // 基础统计卡片
              { i: 'broker-count', x: 0, y: 0, w: 2, h: 2, component: <Card size="small"><Statistic title="Broker 数量" value={metrics?.broker_count || 0} valueStyle={{ color: '#1890ff', fontSize: 20 }} /></Card> },
              { i: 'topic-count', x: 2, y: 0, w: 2, h: 2, component: <Card size="small"><Statistic title="Topic 数量" value={metrics?.topic_count || 0} valueStyle={{ color: '#52c41a', fontSize: 20 }} /></Card> },
              { i: 'partition-total', x: 4, y: 0, w: 2, h: 2, component: <Card size="small"><Statistic title="分区总数" value={overviewStats.topicPartitionTotal} valueStyle={{ fontSize: 20 }} /></Card> },
              { i: 'cg-count', x: 6, y: 0, w: 2, h: 2, component: <Card size="small"><Statistic title="消费组数量" value={metrics?.consumer_groups?.length || 0} valueStyle={{ color: '#faad14', fontSize: 20 }} /></Card> },
              { i: 'cg-member', x: 8, y: 0, w: 2, h: 2, component: <Card size="small"><Statistic title="消费组成员" value={overviewStats.consumerGroupMemberTotal} valueStyle={{ fontSize: 20 }} /></Card> },
              { i: 'isr-total', x: 10, y: 0, w: 2, h: 2, component: <Card size="small"><Statistic title="ISR 总数" value={overviewStats.isrTotal} valueStyle={{ color: '#52c41a', fontSize: 20 }} /></Card> },
              
              // 副本状态卡片
              { i: 'non-preferred', x: 0, y: 2, w: 2, h: 2, component: <Card size="small"><Statistic title="非首选 Leader" value={overviewStats.nonPreferredLeaderCount} valueStyle={{ color: overviewStats.nonPreferredLeaderCount > 0 ? '#f5222d' : '#52c41a', fontSize: 20 }} /></Card> },
              { i: 'total-lag', x: 2, y: 2, w: 2, h: 2, component: <Card size="small"><Statistic title="总消费延迟" value={metrics?.consumer_groups?.reduce((sum, g) => sum + g.total_lag, 0) || 0} valueStyle={{ color: '#f5222d', fontSize: 20 }} /></Card> },
              { i: 'fenced-broker', x: 4, y: 2, w: 2, h: 2, component: <Card size="small"><Statistic title="不健康 Broker" value={overviewStats.fencedBrokerCount} valueStyle={{ color: overviewStats.fencedBrokerCount === 0 ? '#52c41a' : '#f5222d', fontSize: 20 }} /></Card> },
              { i: 'replica-imbalance', x: 6, y: 2, w: 2, h: 2, component: <Card size="small"><Statistic title="副本不均衡" value={overviewStats.preferredReplicaImbalance} valueStyle={{ color: overviewStats.preferredReplicaImbalance === 0 ? '#52c41a' : '#f5222d', fontSize: 20 }} /></Card> },
              { i: 'offline-partitions', x: 8, y: 2, w: 2, h: 2, component: <Card size="small"><Statistic title="离线分区数" value={overviewStats.offlinePartitionsCount} valueStyle={{ color: overviewStats.offlinePartitionsCount === 0 ? '#52c41a' : '#f5222d', fontSize: 20 }} /></Card> },
              { i: 'active-controller', x: 10, y: 2, w: 2, h: 2, component: <Card size="small"><Statistic title="活跃 Controller" value={overviewStats.activeControllerCount} valueStyle={{ color: overviewStats.activeControllerCount === 1 ? '#52c41a' : '#f5222d', fontSize: 20 }} /></Card> },
              
              // 日志状态卡片
              { i: 'offline-log-dir', x: 0, y: 4, w: 2, h: 2, component: <Card size="small"><Statistic title="离线日志目录" value={overviewStats.offlineLogDirectoryCount} valueStyle={{ color: overviewStats.offlineLogDirectoryCount === 0 ? '#52c41a' : '#f5222d', fontSize: 20 }} /></Card> },
              { i: 'log-dir-status', x: 2, y: 4, w: 2, h: 2, component: <Card size="small"><Statistic title="日志目录状态" value={overviewStats.logDirectoryOffline === 0 ? '正常' : '异常'} valueStyle={{ color: overviewStats.logDirectoryOffline === 0 ? '#52c41a' : '#f5222d', fontSize: 16 }} /></Card> },
              
              // 数据损坏卡片
              { i: 'invalid-magic', x: 4, y: 4, w: 2, h: 2, component: <Card size="small"><Statistic title="无效 Magic" value={dataCorruptionStats.invalidMagicNumber} valueStyle={{ color: dataCorruptionStats.invalidMagicNumber === 0 ? '#52c41a' : '#f5222d', fontSize: 20 }} /></Card> },
              { i: 'invalid-crc', x: 6, y: 4, w: 2, h: 2, component: <Card size="small"><Statistic title="无效 CRC" value={dataCorruptionStats.invalidCrc} valueStyle={{ color: dataCorruptionStats.invalidCrc === 0 ? '#52c41a' : '#f5222d', fontSize: 20 }} /></Card> },
              { i: 'invalid-offset', x: 8, y: 4, w: 2, h: 2, component: <Card size="small"><Statistic title="无效 Offset" value={dataCorruptionStats.invalidOffset} valueStyle={{ color: dataCorruptionStats.invalidOffset === 0 ? '#52c41a' : '#f5222d', fontSize: 20 }} /></Card> },
              
              // 趋势图表
              { i: 'lag-chart', x: 0, y: 6, w: 6, h: 5, component: <Card size="small"><ReactECharts option={getLagTrendChartOption()} style={{ height: 220 }} /></Card> },
              { i: 'produce-rate', x: 6, y: 6, w: 6, h: 5, component: <Card size="small"><ReactECharts option={getProduceRateChartOption()} style={{ height: 220 }} /></Card> },
              { i: 'consume-rate', x: 0, y: 11, w: 6, h: 5, component: <Card size="small"><ReactECharts option={getConsumeRateChartOption()} style={{ height: 220 }} /></Card> },
              { i: 'bytes-in', x: 6, y: 11, w: 6, h: 5, component: <Card size="small"><ReactECharts option={getBytesInChartOption()} style={{ height: 220 }} /></Card> },
              { i: 'bytes-out', x: 0, y: 16, w: 6, h: 5, component: <Card size="small"><ReactECharts option={getBytesOutChartOption()} style={{ height: 220 }} /></Card> },
              { i: 'bytes-rejected', x: 6, y: 16, w: 6, h: 5, component: <Card size="small"><ReactECharts option={getBytesRejectedChartOption()} style={{ height: 220 }} /></Card> },
              { i: 'failed-produce', x: 0, y: 21, w: 6, h: 5, component: <Card size="small"><ReactECharts option={getFailedProduceRateChartOption()} style={{ height: 220 }} /></Card> },
              { i: 'failed-fetch', x: 6, y: 21, w: 6, h: 5, component: <Card size="small"><ReactECharts option={getFailedFetchRateChartOption()} style={{ height: 220 }} /></Card> },
              { i: 'messages-in', x: 0, y: 26, w: 6, h: 5, component: <Card size="small"><ReactECharts option={getMessagesInRateChartOption()} style={{ height: 220 }} /></Card> },
            ]}
          />
        </>
      )
    },
    {
      key: 'broker',
      label: 'Broker 监控',
      children: (
        <Spin spinning={brokerOverviewLoading || brokerChartLoading}>
          {/* Broker 选择器 */}
          <Space style={{ marginBottom: 16 }} wrap>
            <Select
              placeholder="选择 Broker"
              value={selectedBroker}
              onChange={setSelectedBroker}
              style={{ width: 200 }}
              options={[
                { label: '全部 Broker', value: 'all' },
                ...brokerList.map(b => ({ label: `Broker ${b.id} (${b.host})`, value: b.id })),
              ]}
            />
          </Space>

          {/* Broker 总览表格 */}
          <Card size="small" title="Broker 总览" style={{ marginBottom: 16 }}>
            <Table
              size="small"
              dataSource={brokerOverviewData}
              loading={brokerOverviewLoading}
              rowKey="broker_id"
              pagination={false}
              columns={[
                { title: 'Broker ID', dataIndex: 'broker_id', key: 'broker_id', width: 100 },
                { title: 'Host', dataIndex: 'broker_host', key: 'broker_host' },
                { title: 'Leader Percent', dataIndex: 'leader_percent', key: 'leader_percent', width: 130, render: (val: number) => `${val?.toFixed(1) ?? 0}%`, sorter: (a: any, b: any) => a.leader_percent - b.leader_percent },
                { title: 'Leader 个数', dataIndex: 'leader_count', key: 'leader_count', width: 110, sorter: (a: any, b: any) => a.leader_count - b.leader_count },
                { title: 'Replicas 个数', dataIndex: 'replica_count', key: 'replica_count', width: 120, sorter: (a: any, b: any) => a.replica_count - b.replica_count },
                { title: '角色', dataIndex: 'is_controller', key: 'is_controller', width: 100, render: (isController: boolean) => <Tag color={isController ? 'red' : 'default'}>{isController ? 'Controller' : 'Follower'}</Tag> },
              ]}
            />
          </Card>

          <DashboardGrid
            storageKey="broker-monitor"
            cols={{ lg: 12, md: 12, sm: 6, xs: 4 }}
            rowHeight={45}
            items={[
              // 副本健康 Stat 卡片
              { i: 'isr-failed', x: 0, y: 0, w: 3, h: 2, component: <Card size="small"><Statistic title="ISR 更新失败" value={brokerIsrUpdatesFailed} valueStyle={{ color: brokerIsrUpdatesFailed === 0 ? '#52c41a' : '#f5222d', fontSize: 18 }} /></Card> },
              { i: 'unclean-leader', x: 3, y: 0, w: 3, h: 2, component: <Card size="small"><Statistic title="Unclean Leader 选举" value={brokerUncleanLeaderElections} valueStyle={{ color: brokerUncleanLeaderElections === 0 ? '#52c41a' : '#f5222d', fontSize: 18 }} /></Card> },
              { i: 'failed-partitions', x: 6, y: 0, w: 3, h: 2, component: <Card size="small"><Statistic title="Follower 失败分区" value={brokerFailedPartitionsCount} valueStyle={{ color: brokerFailedPartitionsCount === 0 ? '#52c41a' : '#f5222d', fontSize: 18 }} /></Card> },
              { i: 'dead-threads', x: 9, y: 0, w: 3, h: 2, component: <Card size="small"><Statistic title="Follower 死线程" value={brokerDeadThreadCount} valueStyle={{ color: brokerDeadThreadCount === 0 ? '#52c41a' : '#f5222d', fontSize: 18 }} /></Card> },
              
              // 请求延迟图表
              { i: 'latency-produce', x: 0, y: 2, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`lp-${selectedBroker}-${quickRange}`} option={getBrokerLatencyChartOption('生产请求延迟 P99', brokerRequestLatencyData.produce)} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'latency-fetch', x: 4, y: 2, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`lf-${selectedBroker}-${quickRange}`} option={getBrokerLatencyChartOption('消费请求延迟 P99', brokerRequestLatencyData.fetchConsumer)} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'latency-follower', x: 8, y: 2, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`lfo-${selectedBroker}-${quickRange}`} option={getBrokerLatencyChartOption('副本同步延迟 P99', brokerRequestLatencyData.fetchFollower)} style={{ height: 200 }} notMerge={true} /></Card> },
              
              // 副本 Lag + 字节速率
              { i: 'replica-lag', x: 0, y: 7, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`rl-${selectedBroker}-${quickRange}`} option={getBrokerReplicaLagChartOption()} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'bytes-in', x: 4, y: 7, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`bi-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('字节流入速率', brokerBytesInData.brokers || {}, '#52c41a', 'bytes/s', (v) => formatBytesForChart(v))} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'bytes-out', x: 8, y: 7, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`bo-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('字节流出速率', brokerBytesOutData.brokers || {}, '#faad14', 'bytes/s', (v) => formatBytesForChart(v))} style={{ height: 200 }} notMerge={true} /></Card> },
              
              // 请求延迟分解
              { i: 'queue-time', x: 0, y: 12, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`qt-${selectedBroker}-${quickRange}`} option={getBrokerLatencyChartOption('请求排队延迟 P99', brokerQueueTimeData)} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'local-time', x: 4, y: 12, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`lt-${selectedBroker}-${quickRange}`} option={getBrokerLatencyChartOption('本地处理延迟 P99', brokerLocalTimeData)} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'remote-time', x: 8, y: 12, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`rt-${selectedBroker}-${quickRange}`} option={getBrokerLatencyChartOption('远程等待延迟 P99', brokerRemoteTimeData)} style={{ height: 200 }} notMerge={true} /></Card> },
              
              // 限流 + 错误
              { i: 'throttle-time', x: 0, y: 17, w: 6, h: 5, component: <Card size="small"><ReactECharts key={`tt-${selectedBroker}-${quickRange}`} option={getBrokerLatencyChartOption('限流延迟 P99', brokerThrottleTimeData)} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'errors', x: 6, y: 17, w: 6, h: 5, component: <Card size="small"><ReactECharts key={`err-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('请求错误速率', brokerErrorsData, '#f5222d', 'errors/s')} style={{ height: 200 }} notMerge={true} /></Card> },
              
              // 副本同步 + 分区迁移
              { i: 'repl-in', x: 0, y: 22, w: 3, h: 5, component: <Card size="small"><ReactECharts key={`ri-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('副本同步流入', brokerReplicationInData.brokers || {}, '#1890ff', 'B/s', (v) => formatBytesForChart(v))} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'repl-out', x: 3, y: 22, w: 3, h: 5, component: <Card size="small"><ReactECharts key={`ro-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('副本同步流出', brokerReplicationOutData.brokers || {}, '#722ed1', 'B/s', (v) => formatBytesForChart(v))} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'reassign-in', x: 6, y: 22, w: 3, h: 5, component: <Card size="small"><ReactECharts key={`rai-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('分区迁移流入', brokerReassignmentInData.brokers || {}, '#13c2c2', 'B/s', (v) => formatBytesForChart(v))} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'reassign-out', x: 9, y: 22, w: 3, h: 5, component: <Card size="small"><ReactECharts key={`rao-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('分区迁移流出', brokerReassignmentOutData.brokers || {}, '#eb2f96', 'B/s', (v) => formatBytesForChart(v))} style={{ height: 200 }} notMerge={true} /></Card> },
              
              // ISR
              { i: 'isr-shrinks', x: 0, y: 27, w: 6, h: 5, component: <Card size="small"><ReactECharts key={`is-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('ISR 收缩速率', brokerIsrShrinksData.brokers || {}, '#f5222d', '次/秒')} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'isr-expands', x: 6, y: 27, w: 6, h: 5, component: <Card size="small"><ReactECharts key={`ie-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('ISR 扩展速率', brokerIsrExpandsData.brokers || {}, '#52c41a', '次/秒')} style={{ height: 200 }} notMerge={true} /></Card> },
              
              // 网络/线程
              { i: 'response-queue', x: 0, y: 32, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`rq-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('响应队列大小', brokerResponseQueueData, '#1890ff', '个')} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'handler-idle', x: 4, y: 32, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`hi-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('Handler 空闲率', brokerHandlerIdleData, '#52c41a', '%', (v) => (v * 100).toFixed(1) + '%')} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'network-idle', x: 8, y: 32, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`ni-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('网络 Processor 空闲率', brokerNetworkIdleData, '#722ed1', '%', (v) => (v * 100).toFixed(1) + '%')} style={{ height: 200 }} notMerge={true} /></Card> },
              
              // 磁盘
              { i: 'disk-read', x: 0, y: 37, w: 6, h: 5, component: <Card size="small"><ReactECharts key={`dr-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('磁盘读取速率', brokerDiskReadData, '#1890ff', 'B/s', (v) => formatBytesForChart(v))} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'disk-write', x: 6, y: 37, w: 6, h: 5, component: <Card size="small"><ReactECharts key={`dw-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('磁盘写入速率', brokerDiskWriteData, '#faad14', 'B/s', (v) => formatBytesForChart(v))} style={{ height: 200 }} notMerge={true} /></Card> },
              
              // 延迟操作
              { i: 'fetch-expires', x: 0, y: 42, w: 3, h: 2, component: <Card size="small"><Statistic title="Fetch 延迟过期" value={brokerDelayedFetchExpires} valueStyle={{ fontSize: 18 }} /></Card> },
              { i: 'controller-event', x: 3, y: 42, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`ce-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('Controller 事件排队耗时', brokerControllerEventQueueData, '#722ed1', 'ms')} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'delayed-ops', x: 7, y: 42, w: 5, h: 5, component: <Card size="small"><ReactECharts key={`do-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('延迟操作数', brokerDelayedOperationsData, '#f5222d', '个')} style={{ height: 200 }} notMerge={true} /></Card> },
              
              // Replica Fetcher + Log Flush
              { i: 'min-fetch', x: 0, y: 47, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`mf-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('Follower 最小拉取速率', brokerMinFetchRateData, '#1890ff', '条/秒')} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'log-flush', x: 4, y: 47, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`lf2-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('日志 Flush 耗时 P99', brokerLogFlushTimeData, '#faad14', 'ms')} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'purgatory', x: 8, y: 47, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`pg-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('Purgatory 大小', brokerPurgatorySizeData, '#faad14', '个')} style={{ height: 200 }} notMerge={true} /></Card> },
              
              // 系统资源 Stat 卡片
              { i: 'process-start', x: 0, y: 52, w: 4, h: 2, component: <Card size="small"><Statistic title="进程启动时间" value={brokerProcessStartTime ? new Date(brokerProcessStartTime * 1000).toLocaleString() : '-'} valueStyle={{ fontSize: 14 }} /></Card> },
              { i: 'max-fds', x: 4, y: 52, w: 4, h: 2, component: <Card size="small"><Statistic title="最大文件描述符" value={brokerProcessMaxFds} valueStyle={{ fontSize: 18 }} /></Card> },
              
              // 系统资源图表
              { i: 'cpu-usage', x: 0, y: 54, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`cpu-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('进程 CPU 使用率', brokerProcessCpuData, '#1890ff', '%', (v) => (v * 100).toFixed(2) + '%')} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'resident-mem', x: 4, y: 54, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`rm-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('进程驻留内存', brokerProcessResidentMemoryData, '#52c41a', 'B', (v) => formatBytesForChart(v))} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'virtual-mem', x: 8, y: 54, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`vm-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('进程虚拟内存', brokerProcessVirtualMemoryData, '#722ed1', 'B', (v) => formatBytesForChart(v))} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'open-fds', x: 0, y: 59, w: 6, h: 5, component: <Card size="small"><ReactECharts key={`of-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('已用文件描述符', brokerProcessOpenFdsData, '#faad14', '个')} style={{ height: 200 }} notMerge={true} /></Card> },
              
              // Log Cleaner Stat 卡片
              { i: 'uncleanable-parts', x: 6, y: 59, w: 3, h: 2, component: <Card size="small"><Statistic title="不可清理分区" value={brokerLogCleanerUncleanablePartitions} valueStyle={{ fontSize: 18 }} /></Card> },
              { i: 'cleaner-dead', x: 9, y: 59, w: 3, h: 2, component: <Card size="small"><Statistic title="Cleaner 死线程" value={brokerLogCleanerDeadThreads} valueStyle={{ color: brokerLogCleanerDeadThreads === 0 ? '#52c41a' : '#f5222d', fontSize: 18 }} /></Card> },
              
              // Log Cleaner 图表
              { i: 'max-dirty', x: 0, y: 64, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`md-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('最大脏比例', brokerLogCleanerMaxDirtyData, '#f5222d', '%')} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'time-since', x: 4, y: 64, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`ts-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('上次清理间隔', brokerLogCleanerTimeSinceLastRunData, '#1890ff', 'ms')} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'uncleanable-bytes', x: 8, y: 64, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`ub-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('不可清理字节数', brokerLogCleanerUncleanableBytesData, '#faad14', 'B', (v) => formatBytesForChart(v))} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'recopy', x: 0, y: 69, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`rc-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('Cleaner 重新复制比例', brokerLogCleanerRecopyData, '#722ed1', '%')} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'max-buffer', x: 4, y: 69, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`mb-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('Cleaner 最大缓冲利用率', brokerLogCleanerMaxBufferData, '#13c2c2', '%')} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'max-clean', x: 8, y: 69, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`mc-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('Cleaner 最大清理时间', brokerLogCleanerMaxCleanTimeData, '#eb2f96', '秒')} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'max-compact', x: 0, y: 74, w: 6, h: 5, component: <Card size="small"><ReactECharts key={`mc2-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('Cleaner 最大压缩延迟', brokerLogCleanerMaxCompactionDelayData, '#fa8c16', '秒')} style={{ height: 200 }} notMerge={true} /></Card> },
              
              // JVM Stat 卡片
              { i: 'deadlocked', x: 6, y: 74, w: 3, h: 2, component: <Card size="small"><Statistic title="死锁线程数" value={brokerJvmDeadlockedThreads} valueStyle={{ color: brokerJvmDeadlockedThreads === 0 ? '#52c41a' : '#f5222d', fontSize: 18 }} /></Card> },
              
              // JVM 图表
              { i: 'gc-sum', x: 0, y: 79, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`gs-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('GC 耗时', brokerJvmGcData, '#f5222d', '秒')} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'gc-count', x: 4, y: 79, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`gc-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('GC 次数', brokerJvmGcCountData, '#faad14', '次')} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'jvm-mem', x: 8, y: 79, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`jm-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('JVM 内存池已用', brokerJvmMemoryPoolData, '#1890ff', 'B', (v) => formatBytesForChart(v))} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'jvm-threads', x: 0, y: 84, w: 6, h: 5, component: <Card size="small"><ReactECharts key={`jt-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('JVM 线程数', brokerJvmThreadsData, '#52c41a', '个')} style={{ height: 200 }} notMerge={true} /></Card> },
              { i: 'jvm-buffer', x: 6, y: 84, w: 6, h: 5, component: <Card size="small"><ReactECharts key={`jb-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('JVM Buffer 池已用', brokerJvmBufferPoolData, '#722ed1', 'B', (v) => formatBytesForChart(v))} style={{ height: 200 }} notMerge={true} /></Card> },
            ]}
          />
        </Spin>
      )
    },
    {
      key: 'topic',
      label: 'Topic 监控',
      children: (
        <Spin spinning={topicLoading}>
          {/* 选择器 */}
          <Space style={{ marginBottom: 16 }} wrap>
            <Select
              placeholder="选择 Topic"
              value={selectedTopic}
              onChange={(value) => {
                setSelectedTopic(value)
                setSelectedPartitions([])
              }}
              style={{ width: 200 }}
              options={topics.map(t => ({ label: t.name, value: t.name }))}
              allowClear
              showSearch
              filterOption={(input, option) => 
                (option?.label ?? '').toLowerCase().includes(input.toLowerCase())
              }
            />
            <Select
              placeholder="选择消费组"
              value={selectedConsumerGroup}
              onChange={setSelectedConsumerGroup}
              style={{ width: 200 }}
              options={topicConsumerGroups.map(cg => ({ label: cg, value: cg }))}
              allowClear
              showSearch
              disabled={!selectedTopic}
              filterOption={(input, option) => 
                (option?.label ?? '').toLowerCase().includes(input.toLowerCase())
              }
            />
          </Space>

          {!selectedTopic ? (
            <Alert
              message="请选择 Topic"
              description="选择 Topic 后将显示该 Topic 的详细监控信息"
              type="info"
            />
          ) : (
            <>
              {/* Topic 概览 */}
              <Card size="small" title="Topic 概览" style={{ marginBottom: 16 }}>
                <Row gutter={24}>
                  <Col span={6}>
                    <Statistic title="Topic 名称" value={selectedTopic} valueStyle={{ fontSize: 16 }} />
                  </Col>
                  <Col span={6}>
                    <Statistic title="分区数" value={topics.find(t => t.name === selectedTopic)?.partitions || 0} />
                  </Col>
                  <Col span={6}>
                    <Statistic title="消费组数量" value={topicConsumerGroups.length} />
                  </Col>
                  <Col span={6}>
                    <Statistic 
                      title="总 Lag" 
                      value={metrics?.consumer_groups
                        ?.filter(cg => cg.topics.some(t => t.topic === selectedTopic))
                        .reduce((sum, cg) => sum + cg.topics
                          .filter(t => t.topic === selectedTopic)
                          .reduce((s, t) => s + t.lag, 0), 0) || 0}
                      valueStyle={{ color: '#f5222d' }}
                    />
                  </Col>
                </Row>
                
                {/* 消费组列表 */}
                {topicConsumerGroups.length === 0 ? (
                  <Alert message="该 Topic 暂无消费组" type="info" style={{ marginTop: 16 }} />
                ) : (
                  <div style={{ marginTop: 16 }}>
                    <h4 style={{ marginBottom: 8 }}>消费组列表（点击选中）</h4>
                    <Table
                      size="small"
                      dataSource={metrics?.consumer_groups
                        ?.filter(cg => cg.topics.some(t => t.topic === selectedTopic))
                        .map(cg => {
                          const topicData = cg.topics.filter(t => t.topic === selectedTopic)
                          return {
                            group_id: cg.group_id,
                            state: cg.state,
                            member_count: cg.member_count || 0,
                            topic_lag: topicData.reduce((s, t) => s + t.lag, 0),
                            topic: selectedTopic,
                            partitions: topicData.length
                          }
                        }) || []}
                      columns={[
                        { title: '消费组', dataIndex: 'group_id', key: 'group_id' },
                        { title: '状态', dataIndex: 'state', key: 'state', render: (state: string) => <Tag color={state === 'Stable' ? 'green' : state === 'Dead' ? 'red' : 'default'}>{state}</Tag> },
                        { title: '成员数', dataIndex: 'member_count', key: 'member_count' },
                        { title: '消费分区数', dataIndex: 'partitions', key: 'partitions' },
                        { title: 'Lag', dataIndex: 'topic_lag', key: 'topic_lag', render: (val: number) => val?.toLocaleString() || 0 },
                      ]}
                      rowKey="group_id"
                      pagination={false}
                      onRow={(record) => ({
                        onClick: () => setSelectedConsumerGroup(record.group_id),
                        style: { cursor: 'pointer', backgroundColor: selectedConsumerGroup === record.group_id ? '#e6f7ff' : undefined }
                      })}
                    />
                  </div>
                )}
              </Card>

              {/* 分区选择器 */}
              {partitionMetrics.produceRate.length > 0 && (
                <Card size="small" title="分区选择（点击筛选要查看的分区）" style={{ marginBottom: 16 }}>
                  <div style={{ maxHeight: 120, overflowY: 'auto' }}>
                    <Checkbox.Group
                      value={selectedPartitions}
                      onChange={(values) => setSelectedPartitions(values as number[])}
                    >
                      <Space wrap>
                        {partitionMetrics.produceRate
                          .map(p => p.partition)
                          .sort((a, b) => a - b)
                          .map(p => (
                            <Checkbox key={p} value={p}>分区 {p}</Checkbox>
                          ))}
                      </Space>
                    </Checkbox.Group>
                  </div>
                </Card>
              )}

              {/* 拖拽布局图表 */}
              <DashboardGrid
                storageKey="topic-monitor"
                cols={{ lg: 12, md: 12, sm: 6, xs: 4 }}
                rowHeight={45}
                items={[
                  // Topic 汇总图表
                  { i: 'total-produce', x: 0, y: 0, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`tp-${selectedTopic}`} option={getTopicTotalProduceRateChartOption()} style={{ height: 200 }} notMerge={true} /></Card> },
                  { i: 'total-consume', x: 4, y: 0, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`tc-${selectedTopic}-${selectedConsumerGroup}`} option={getTopicTotalConsumeRateChartOption()} style={{ height: 200 }} notMerge={true} /></Card> },
                  { i: 'total-lag', x: 8, y: 0, w: 4, h: 5, component: <Card size="small"><ReactECharts key={`tl-${selectedTopic}-${selectedConsumerGroup}`} option={getTopicTotalLagChartOption()} style={{ height: 200 }} notMerge={true} /></Card> },
                  
                  // Under Replicated 统计
                  { i: 'under-replicated', x: 0, y: 5, w: 3, h: 2, component: <Card size="small"><Statistic title="Under Replicated 分区" value={topicUnderReplicatedCount} valueStyle={{ color: topicUnderReplicatedCount === 0 ? '#52c41a' : '#f5222d', fontSize: 20 }} /></Card> },
                  
                  // 分区级别图表（选中分区后显示）
                  ...(selectedPartitions.length > 0 ? [
                    { i: 'partition-produce', x: 0, y: 7, w: 12, h: 7, component: <Card size="small"><ReactECharts key={`pp-${selectedTopic}-${selectedPartitions.join('-')}`} option={getTopicProduceRateChartOption()} style={{ height: 280 }} notMerge={true} /></Card> },
                    { i: 'partition-consume', x: 0, y: 14, w: 6, h: 7, component: <Card size="small"><ReactECharts key={`pc-${selectedTopic}-${selectedConsumerGroup}-${selectedPartitions.join('-')}`} option={getTopicConsumeRateChartOption()} style={{ height: 280 }} notMerge={true} /></Card> },
                    { i: 'partition-lag', x: 6, y: 14, w: 6, h: 7, component: <Card size="small"><ReactECharts key={`pl-${selectedTopic}-${selectedConsumerGroup}-${selectedPartitions.join('-')}`} option={getTopicLagChartOption()} style={{ height: 280 }} notMerge={true} /></Card> },
                  ] : []),
                  
                  // JMX 指标图表
                  { i: 'log-size', x: 0, y: 21, w: 12, h: 7, component: <Card size="small"><ReactECharts key={`ls-${selectedTopic}-${selectedPartitions.join('-')}`} option={getTopicLogSizeChartOption()} style={{ height: 280 }} notMerge={true} /></Card> },
                  { i: 'log-end-offset', x: 0, y: 28, w: 12, h: 7, component: <Card size="small"><ReactECharts key={`leo-${selectedTopic}-${selectedPartitions.join('-')}`} option={getTopicLogEndOffsetChartOption()} style={{ height: 280 }} notMerge={true} /></Card> },
                  { i: 'isr-vs-replica', x: 0, y: 35, w: 12, h: 7, component: <Card size="small"><ReactECharts key={`ivr-${selectedTopic}`} option={getTopicIsrVsReplicaChartOption()} style={{ height: 280 }} notMerge={true} /></Card> },
                ]}
              />
            </>
          )}
        </Spin>
      )
    }
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card 
        title="集群监控" 
        extra={
          <Space>
            <Select
              placeholder="选择集群"
              value={selectedCluster?.cluster_id}
              onChange={(value) => {
                const cluster = clusters.find(c => c.cluster_id === value)
                setSelectedCluster(cluster || null)
              }}
              style={{ width: 200 }}
              options={clusters.map(c => ({ label: c.cluster_name, value: c.cluster_id }))}
            />
          </Space>
        }
      >
        {/* 时间选择器放在 Tabs 上方 */}
        <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'flex-end' }}>
          {renderTimeSelector()}
        </div>
        <Spin spinning={loading}>
          <Tabs 
            activeKey={activeTab} 
            onChange={setActiveTab}
            items={tabItems}
          />
        </Spin>
      </Card>
    </div>
  )
}

// VictoriaMetrics API 响应类型
interface VMQueryResponse {
  status: string
  data: {
    result: Array<{
      metric: Record<string, string>
      values: Array<[number, string]>
    }>
  }
}

export default Monitor
