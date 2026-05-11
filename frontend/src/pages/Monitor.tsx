import { useState, useEffect } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Card, Row, Col, Select, Spin, message, Statistic, Table, Tabs, Space, Tag, Alert, DatePicker, Button, Checkbox } from 'antd'
import { CalendarOutlined } from '@ant-design/icons'
import ReactECharts from 'echarts-for-react'
import dayjs, { Dayjs } from 'dayjs'
import { clusterAPI } from '../services/cluster'
import { metricsAPI, ClusterMetricsResponse } from '../services/metrics'
import axios from '../services/api'

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
  const [historyLoading, setHistoryLoading] = useState(false)
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

  // 集群概览 - 统计指标（从 VM 查询）
  const [overviewStats, setOverviewStats] = useState({
    topicPartitionTotal: 0,
    consumerGroupMemberCount: 0,
    consumerGroupMemberTotal: 0,
    isrTotal: 0,
    nonPreferredLeaderCount: 0,
  })

  // 集群概览 - 趋势数据
  const [produceRateData, setProduceRateData] = useState<{ times: string[], values: number[] }>({ times: [], values: [] })
  const [consumeRateData, setConsumeRateData] = useState<{ times: string[], values: number[] }>({ times: [], values: [] })
  const [lagTrendData, setLagTrendData] = useState<{ times: string[], values: number[] }>({ times: [], values: [] })
  const [bytesInData, setBytesInData] = useState<{ times: string[], values: number[] }>({ times: [], values: [] })
  const [bytesOutData, setBytesOutData] = useState<{ times: string[], values: number[] }>({ times: [], values: [] })

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

      // 并行查询所有图表数据
      const [proRes, fetchRes, followerRes, lagRes, bytesInRes, bytesOutRes] = await Promise.all([
        queryVMMulti(
          `kafka_broker_request_latency_ms{cluster_id="${clusterId}",request="Produce"${brokerFilter}}`,
          start, end, step
        ),
        queryVMMulti(
          `kafka_broker_request_latency_ms{cluster_id="${clusterId}",request="FetchConsumer"${brokerFilter}}`,
          start, end, step
        ),
        queryVMMulti(
          `kafka_broker_request_latency_ms{cluster_id="${clusterId}",request="FetchFollower"${brokerFilter}}`,
          start, end, step
        ),
        queryVMMulti(
          `kafka_broker_replica_max_lag{cluster_id="${clusterId}"${brokerFilter}}`,
          start, end, step
        ),
        queryVMMulti(
          `rate(kafka_broker_bytes_in_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`,
          start, end, step
        ),
        queryVMMulti(
          `rate(kafka_broker_bytes_out_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`,
          start, end, step
        ),
      ])

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
    
    setHistoryLoading(true)
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
      ] = await Promise.all([
        queryVMInstant(`sum(kafka_topic_partitions{cluster_id="${clusterId}",topic!~"__.*"})`),
        queryVMInstant(`count(kafka_consumergroup_members{cluster_id="${clusterId}",consumergroup!~"__.*"})`),
        queryVMInstant(`sum(kafka_consumergroup_members{cluster_id="${clusterId}",consumergroup!~"__.*"})`),
        queryVMInstant(`sum(kafka_topic_partition_in_sync_replica{cluster_id="${clusterId}"})`),
        queryVMInstant(`count(kafka_topic_partition_leader_is_preferred{cluster_id="${clusterId}"}<1)`),
      ])

      setOverviewStats({
        topicPartitionTotal,
        consumerGroupMemberCount,
        consumerGroupMemberTotal,
        isrTotal,
        nonPreferredLeaderCount,
      })

      // 2. 查询趋势数据
      const [produceRateRes, consumeRateRes, lagRes, bytesInRes, bytesOutRes] = await Promise.all([
        queryVM(`sum(rate(kafka_topic_partition_current_offset{cluster_id="${clusterId}",topic!~"__.*"}[30s]))`, start, end, step),
        queryVM(`sum(rate(kafka_consumergroup_current_offset{cluster_id="${clusterId}"}[30s]))`, start, end, step),
        queryVM(`sum(kafka_consumergroup_lag_sum{cluster_id="${clusterId}"})`, start, end, step),
        queryVM(`sum(rate(kafka_broker_bytes_in_total{cluster_id="${clusterId}"}[30s]))`, start, end, step),
        queryVM(`sum(rate(kafka_broker_bytes_out_total{cluster_id="${clusterId}"}[30s]))`, start, end, step),
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

    } catch (error) {
      console.error('Failed to load history', error)
    } finally {
      setHistoryLoading(false)
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
    data: Record<string, { times: string[], values: number[] }>,
    _color: string,
    yAxisName: string,
    tooltipFormatter?: (value: number) => string,
  ) => {
    const colors = ['#1890ff', '#52c41a', '#faad14', '#f5222d', '#722ed1', '#13c2c2', '#eb2f96', '#fa8c16']
    const entries = Object.entries(data)
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

          {/* 第一行：基础统计 */}
          <Row gutter={16}>
            <Col span={4}>
              <Card size="small">
                <Statistic 
                  title="Broker 数量" 
                  value={metrics?.broker_count || 0} 
                  valueStyle={{ color: '#1890ff', fontSize: 24 }}
                />
              </Card>
            </Col>
            <Col span={4}>
              <Card size="small">
                <Statistic 
                  title="Topic 数量" 
                  value={metrics?.topic_count || 0} 
                  valueStyle={{ color: '#52c41a', fontSize: 24 }}
                />
              </Card>
            </Col>
            <Col span={4}>
              <Card size="small">
                <Statistic 
                  title="分区总数" 
                  value={overviewStats.topicPartitionTotal}
                  valueStyle={{ fontSize: 24 }}
                />
              </Card>
            </Col>
            <Col span={4}>
              <Card size="small">
                <Statistic 
                  title="消费组数量" 
                  value={metrics?.consumer_groups?.length || 0}
                  valueStyle={{ color: '#faad14', fontSize: 24 }}
                />
              </Card>
            </Col>
            <Col span={4}>
              <Card size="small">
                <Statistic 
                  title="消费组成员总数" 
                  value={overviewStats.consumerGroupMemberTotal}
                  valueStyle={{ fontSize: 24 }}
                />
              </Card>
            </Col>
            <Col span={4}>
              <Card size="small">
                <Statistic 
                  title="ISR 总数" 
                  value={overviewStats.isrTotal}
                  valueStyle={{ color: '#52c41a', fontSize: 24 }}
                />
              </Card>
            </Col>
          </Row>

          {/* 第二行：副本状态 */}
          <Row gutter={16} style={{ marginTop: 16 }}>
            <Col span={4}>
              <Card size="small">
                <Statistic 
                  title="非首选 Leader" 
                  value={overviewStats.nonPreferredLeaderCount}
                  valueStyle={{ 
                    color: overviewStats.nonPreferredLeaderCount > 0 ? '#f5222d' : '#52c41a', 
                    fontSize: 24 
                  }}
                />
              </Card>
            </Col>
            <Col span={4}>
              <Card size="small">
                <Statistic 
                  title="总消费延迟" 
                  value={metrics?.consumer_groups?.reduce((sum, g) => sum + g.total_lag, 0) || 0}
                  valueStyle={{ color: '#f5222d', fontSize: 24 }}
                />
              </Card>
            </Col>
          </Row>

          {/* 趋势图 */}
          <div style={{ marginTop: 24 }}>
            <h4>历史趋势</h4>
            {historyLoading ? (
              <Spin tip="加载历史数据..." />
            ) : produceRateData.times.length === 0 ? (
              <Alert
                message="暂无历史数据"
                description="系统会每 30 秒自动采集一次指标，请稍后再查看。确保 VictoriaMetrics 已正确配置。"
                type="info"
              />
            ) : (
              <Row gutter={[16, 16]}>
                <Col span={12}>
                  <Card size="small">
                    <ReactECharts option={getLagTrendChartOption()} style={{ height: 250 }} />
                  </Card>
                </Col>
                <Col span={12}>
                  <Card size="small">
                    <ReactECharts option={getProduceRateChartOption()} style={{ height: 250 }} />
                  </Card>
                </Col>
                <Col span={12}>
                  <Card size="small">
                    <ReactECharts option={getConsumeRateChartOption()} style={{ height: 250 }} />
                  </Card>
                </Col>
                <Col span={12}>
                  <Card size="small">
                    <ReactECharts option={getBytesInChartOption()} style={{ height: 250 }} />
                  </Card>
                </Col>
                <Col span={12}>
                  <Card size="small">
                    <ReactECharts option={getBytesOutChartOption()} style={{ height: 250 }} />
                  </Card>
                </Col>
              </Row>
            )}
          </div>
        </>
      )
    },
    {
      key: 'broker',
      label: 'Broker 监控',
      children: (
        <Spin spinning={brokerOverviewLoading || brokerChartLoading}>
          {/* 1. Broker 选择器 */}
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

          {/* 2. Broker 总览表格 */}
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
                {
                  title: 'Leader Percent',
                  dataIndex: 'leader_percent',
                  key: 'leader_percent',
                  width: 130,
                  render: (val: number) => `${val?.toFixed(1) ?? 0}%`,
                  sorter: (a: any, b: any) => a.leader_percent - b.leader_percent,
                },
                {
                  title: 'Leader 个数',
                  dataIndex: 'leader_count',
                  key: 'leader_count',
                  width: 110,
                  sorter: (a: any, b: any) => a.leader_count - b.leader_count,
                },
                {
                  title: 'Replicas 个数',
                  dataIndex: 'replica_count',
                  key: 'replica_count',
                  width: 120,
                  sorter: (a: any, b: any) => a.replica_count - b.replica_count,
                },
                {
                  title: '角色',
                  dataIndex: 'is_controller',
                  key: 'is_controller',
                  width: 100,
                  render: (isController: boolean) => (
                    <Tag color={isController ? 'red' : 'default'}>
                      {isController ? 'Controller' : 'Follower'}
                    </Tag>
                  ),
                },
              ]}
            />
          </Card>

          {/* 3. 请求延迟图表 */}
          <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
            <Col span={8}>
              <Card size="small">
                <ReactECharts
                  key={`broker-latency-produce-${selectedBroker}-${quickRange}`}
                  option={getBrokerLatencyChartOption('生产请求延迟 P99', brokerRequestLatencyData.produce)}
                  style={{ height: 280 }}
                  notMerge={true}
                />
              </Card>
            </Col>
            <Col span={8}>
              <Card size="small">
                <ReactECharts
                  key={`broker-latency-fetch-${selectedBroker}-${quickRange}`}
                  option={getBrokerLatencyChartOption('消费请求延迟 P99', brokerRequestLatencyData.fetchConsumer)}
                  style={{ height: 280 }}
                  notMerge={true}
                />
              </Card>
            </Col>
            <Col span={8}>
              <Card size="small">
                <ReactECharts
                  key={`broker-latency-follower-${selectedBroker}-${quickRange}`}
                  option={getBrokerLatencyChartOption('副本同步延迟 P99', brokerRequestLatencyData.fetchFollower)}
                  style={{ height: 280 }}
                  notMerge={true}
                />
              </Card>
            </Col>
          </Row>

          {/* 4. 副本 Lag + 字节速率 */}
          <Row gutter={[16, 16]}>
            <Col span={12}>
              <Card size="small">
                <ReactECharts
                  key={`broker-replica-lag-${selectedBroker}-${quickRange}`}
                  option={getBrokerReplicaLagChartOption()}
                  style={{ height: 280 }}
                  notMerge={true}
                />
              </Card>
            </Col>
            <Col span={12}>
              <Card size="small">
                <ReactECharts
                  key={`broker-bytes-in-${selectedBroker}-${quickRange}`}
                  option={buildMultiSeriesChartOption(
                    '字节流入速率',
                    (Object.keys(brokerBytesInData.brokers || {}).length > 0
                      ? brokerBytesInData.brokers
                      : (brokerBytesInData.single ? { [selectedBroker === 'all' ? '0' : selectedBroker]: brokerBytesInData.single } : {})) as Record<string, { times: string[], values: number[] }>,
                    '#52c41a',
                    'bytes/s',
                    (v) => formatBytesForChart(v)
                  )}
                  style={{ height: 280 }}
                  notMerge={true}
                />
              </Card>
            </Col>
            <Col span={12}>
              <Card size="small">
                <ReactECharts
                  key={`broker-bytes-out-${selectedBroker}-${quickRange}`}
                  option={buildMultiSeriesChartOption(
                    '字节流出速率',
                    (Object.keys(brokerBytesOutData.brokers || {}).length > 0
                      ? brokerBytesOutData.brokers
                      : (brokerBytesOutData.single ? { [selectedBroker === 'all' ? '0' : selectedBroker]: brokerBytesOutData.single } : {})) as Record<string, { times: string[], values: number[] }>,
                    '#faad14',
                    'bytes/s',
                    (v) => formatBytesForChart(v)
                  )}
                  style={{ height: 280 }}
                  notMerge={true}
                />
              </Card>
            </Col>
          </Row>
        </Spin>
      )
    },
    {
      key: 'topic',
      label: 'Topic 监控',
      children: (
        <Spin spinning={topicLoading}>
          {/* 1. 选择器 */}
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
              {/* 2. Topic 维度信息表格 */}
              <Card size="small" title="Topic 概览" style={{ marginBottom: 16 }}>
                <Row gutter={24}>
                  <Col span={6}>
                    <Statistic 
                      title="Topic 名称" 
                      value={selectedTopic}
                      valueStyle={{ fontSize: 16 }}
                    />
                  </Col>
                  <Col span={6}>
                    <Statistic 
                      title="分区数" 
                      value={topics.find(t => t.name === selectedTopic)?.partitions || 0}
                    />
                  </Col>
                  <Col span={6}>
                    <Statistic 
                      title="消费组数量" 
                      value={topicConsumerGroups.length}
                    />
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
                  <Alert
                    message="该 Topic 暂无消费组"
                    description="当前选择的 Topic 没有活跃的消费组，请选择其他 Topic 或等待消费组启动"
                    type="info"
                    style={{ marginTop: 16 }}
                  />
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
                        { 
                          title: '状态', 
                          dataIndex: 'state', 
                          key: 'state',
                          render: (state: string) => {
                            const colorMap: Record<string, string> = {
                              'Stable': 'green',
                              'Empty': 'default',
                              'Rebalancing': 'orange',
                              'Dead': 'red',
                            }
                            return <Tag color={colorMap[state] || 'default'}>{state}</Tag>
                          }
                        },
                        { title: '成员数', dataIndex: 'member_count', key: 'member_count' },
                        { title: '消费分区数', dataIndex: 'partitions', key: 'partitions' },
                        { 
                          title: 'Lag', 
                          dataIndex: 'topic_lag', 
                          key: 'topic_lag',
                          render: (val: number) => val?.toLocaleString() || 0
                        },
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

              {/* 3. Topic 汇总指标 */}
              <Row gutter={16} style={{ marginBottom: 16 }}>
                <Col span={8}>
                  <Card size="small">
                    <ReactECharts
                      key={`total-produce-${selectedTopic}`}
                      option={getTopicTotalProduceRateChartOption()}
                      style={{ height: 200 }}
                      notMerge={true}
                    />
                  </Card>
                </Col>
                <Col span={8}>
                  <Card size="small">
                    <ReactECharts
                      key={`total-consume-${selectedTopic}-${selectedConsumerGroup}`}
                      option={getTopicTotalConsumeRateChartOption()}
                      style={{ height: 200 }}
                      notMerge={true}
                    />
                  </Card>
                </Col>
                <Col span={8}>
                  <Card size="small">
                    <ReactECharts
                      key={`total-lag-${selectedTopic}-${selectedConsumerGroup}`}
                      option={getTopicTotalLagChartOption()}
                      style={{ height: 200 }}
                      notMerge={true}
                    />
                  </Card>
                </Col>
              </Row>

              {/* 4. 分区选择器 */}
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

              {/* 5. 分区级别折线图 */}
              {partitionMetrics.produceRate.length > 0 && selectedPartitions.length > 0 && (
                <Row gutter={[16, 16]}>
                  <Col span={24}>
                    <Card size="small">
                      <ReactECharts
                        key={`produce-${selectedTopic}-${selectedPartitions.join('-')}`}
                        option={getTopicProduceRateChartOption()}
                        style={{ height: 300 }}
                        notMerge={true}
                      />
                    </Card>
                  </Col>
                  <Col span={12}>
                    <Card size="small">
                      <ReactECharts
                        key={`consume-${selectedTopic}-${selectedConsumerGroup}-${selectedPartitions.join('-')}`}
                        option={getTopicConsumeRateChartOption()}
                        style={{ height: 300 } }
                        notMerge={true}
                      />
                    </Card>
                  </Col>
                  <Col span={12}>
                    <Card size="small">
                      <ReactECharts
                        key={`lag-${selectedTopic}-${selectedConsumerGroup}-${selectedPartitions.join('-')}`}
                        option={getTopicLagChartOption()}
                        style={{ height: 300 }}
                        notMerge={true}
                      />
                    </Card>
                  </Col>
                </Row>
              )}
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
