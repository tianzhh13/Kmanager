import { useState, useEffect } from 'react'
import { Card, Row, Col, Select, Spin, message, Statistic, Table, Tabs, Space, Tag, Alert, DatePicker, Button } from 'antd'
import { CalendarOutlined } from '@ant-design/icons'
import ReactECharts from 'echarts-for-react'
import dayjs, { Dayjs } from 'dayjs'
import { clusterAPI } from '../services/cluster'
import { metricsAPI, ClusterMetricsResponse, ConsumerGroupInfo, BrokerMetrics } from '../services/metrics'
import type { ColumnsType } from 'antd/es/table'
import axios from '../services/api'

interface ClusterOption {
  cluster_id: number
  cluster_name: string
}

const Monitor: React.FC = () => {
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

  // 折线图数据
  const [throughputData, setThroughputData] = useState<{ times: string[], messages: number[], bytesIn: number[], bytesOut: number[] }>({ times: [], messages: [], bytesIn: [], bytesOut: [] })
  const [lagData, setLagData] = useState<{ times: string[], totalLag: number[], consumerGroupCount: number[] }>({ times: [], totalLag: [], consumerGroupCount: [] })
  const [partitionData, setPartitionData] = useState<{ times: string[], underReplicated: number[], offline: number[] }>({ times: [], underReplicated: [], offline: [] })

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
    if (selectedCluster && activeTab === 'charts') {
      loadHistory()
    }
  }, [selectedCluster, quickRange, customRange, timeRange, activeTab])

  const loadClusters = async () => {
    try {
      const res = await clusterAPI.list()
      setClusters(res.data || [])
      if (res.data?.length > 0) {
        setSelectedCluster(res.data[0])
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

  // 获取时间范围
  const getTimeRange = (): { start: Dayjs; end: Dayjs; step: string } => {
    const end = dayjs()
    let start: Dayjs
    let step: string

    if (timeRange === 'custom' && customRange) {
      start = customRange[0]
      const durationMinutes = end.diff(start, 'minute')
      // 根据时间范围自动计算步长
      if (durationMinutes <= 60) {
        step = '30s'
      } else if (durationMinutes <= 360) {
        step = '1m'
      } else if (durationMinutes <= 1440) {
        step = '5m'
      } else {
        step = '15m'
      }
    } else {
      // 快速选择
      switch (quickRange) {
        case '5m':
          start = end.subtract(5, 'minute')
          step = '10s'
          break
        case '15m':
          start = end.subtract(15, 'minute')
          step = '15s'
          break
        case '30m':
          start = end.subtract(30, 'minute')
          step = '30s'
          break
        case '1h':
          start = end.subtract(1, 'hour')
          step = '30s'
          break
        case '3h':
          start = end.subtract(3, 'hour')
          step = '1m'
          break
        case '6h':
          start = end.subtract(6, 'hour')
          step = '1m'
          break
        case '12h':
          start = end.subtract(12, 'hour')
          step = '2m'
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
          step = '15m'
          break
        case '30d':
          start = end.subtract(30, 'day')
          step = '1h'
          break
        default:
          start = end.subtract(1, 'hour')
          step = '30s'
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
      
      // 并行查询所有指标
      const [messagesRes, bytesInRes, bytesOutRes, lagRes, cgCountRes, underRepRes, offlineRes] = await Promise.all([
        queryVM(`kafka_messages_in_per_sec{cluster_id="${clusterId}"}`, start, end, step),
        queryVM(`kafka_bytes_in_per_sec{cluster_id="${clusterId}"}`, start, end, step),
        queryVM(`kafka_bytes_out_per_sec{cluster_id="${clusterId}"}`, start, end, step),
        queryVM(`kafka_total_lag{cluster_id="${clusterId}"}`, start, end, step),
        queryVM(`kafka_consumer_group_count{cluster_id="${clusterId}"}`, start, end, step),
        queryVM(`kafka_under_replicated_partitions{cluster_id="${clusterId}"}`, start, end, step),
        queryVM(`kafka_offline_partitions_count{cluster_id="${clusterId}"}`, start, end, step),
      ])

      // 处理时间轴
      const times = messagesRes.map(v => dayjs.unix(v[0]).format('HH:mm:ss'))

      // 设置吞吐量数据
      setThroughputData({
        times,
        messages: messagesRes.map(v => parseFloat(v[1]) || 0),
        bytesIn: bytesInRes.map(v => parseFloat(v[1]) || 0),
        bytesOut: bytesOutRes.map(v => parseFloat(v[1]) || 0),
      })

      // 设置延迟数据
      setLagData({
        times,
        totalLag: lagRes.map(v => parseFloat(v[1]) || 0),
        consumerGroupCount: cgCountRes.map(v => parseFloat(v[1]) || 0),
      })

      // 设置分区数据
      setPartitionData({
        times,
        underReplicated: underRepRes.map(v => parseFloat(v[1]) || 0),
        offline: offlineRes.map(v => parseFloat(v[1]) || 0),
      })

    } catch (error) {
      console.error('Failed to load history', error)
    } finally {
      setHistoryLoading(false)
    }
  }

  const consumerColumns: ColumnsType<ConsumerGroupInfo> = [
    { 
      title: '消费组', 
      dataIndex: 'group_id', 
      key: 'group_id' 
    },
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
    { 
      title: '消费延迟', 
      dataIndex: 'total_lag', 
      key: 'total_lag',
      render: (val: number) => val?.toLocaleString() || 0,
      sorter: (a, b) => a.total_lag - b.total_lag,
    },
    { 
      title: '成员数', 
      dataIndex: 'member_count', 
      key: 'member_count' 
    },
    { 
      title: 'Topic 数', 
      dataIndex: 'topics', 
      key: 'topic_count',
      render: (topics: ConsumerGroupInfo['topics']) => topics?.length || 0
    },
  ]

  const expandableConfig = {
    expandedRowRender: (record: ConsumerGroupInfo) => (
      <Table
        size="small"
        dataSource={record.topics}
        rowKey={(r) => `${r.topic}-${r.partition}`}
        columns={[
          { title: 'Topic', dataIndex: 'topic', key: 'topic' },
          { title: '分区', dataIndex: 'partition', key: 'partition' },
          { 
            title: 'Lag', 
            dataIndex: 'lag', 
            key: 'lag',
            render: (val: number) => val?.toLocaleString() || 0,
          },
          { 
            title: 'Log End Offset', 
            dataIndex: 'log_end_offset', 
            key: 'log_end_offset',
            render: (val: number) => val?.toLocaleString() || 0,
          },
          { 
            title: 'Consumer Offset', 
            dataIndex: 'consumer_offset', 
            key: 'consumer_offset',
            render: (val: number) => val?.toLocaleString() || 0,
          },
        ]}
        pagination={false}
      />
    ),
    rowExpandable: (record: ConsumerGroupInfo) => record.topics && record.topics.length > 0,
  }

  const formatBytes = (bytes: number) => {
    if (!bytes) return '0 B'
    const units = ['B', 'KB', 'MB', 'GB', 'TB']
    let i = 0
    while (bytes >= 1024 && i < units.length - 1) {
      bytes /= 1024
      i++
    }
    return `${bytes.toFixed(2)} ${units[i]}`
  }

  const formatBytesForChart = (bytes: number): string => {
    if (!bytes) return '0'
    if (bytes >= 1024 * 1024 * 1024) return (bytes / (1024 * 1024 * 1024)).toFixed(2) + ' GB'
    if (bytes >= 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(2) + ' MB'
    if (bytes >= 1024) return (bytes / 1024).toFixed(2) + ' KB'
    return bytes.toFixed(2) + ' B'
  }

  // 折线图配置 - 吞吐量
  const getThroughputChartOption = () => ({
    title: { text: '吞吐量趋势', left: 'center', textStyle: { fontSize: 14 } },
    tooltip: { trigger: 'axis' },
    legend: { data: ['消息数/s', '字节流入', '字节流出'], bottom: 0 },
    grid: { left: '3%', right: '4%', bottom: '15%', top: '15%', containLabel: true },
    xAxis: { type: 'category', boundaryGap: false, data: throughputData.times },
    yAxis: [
      { type: 'value', name: '消息/s', position: 'left' },
      { type: 'value', name: '字节/s', position: 'right' }
    ],
    series: [
      {
        name: '消息数/s',
        type: 'line',
        smooth: true,
        data: throughputData.messages,
        itemStyle: { color: '#1890ff' },
        areaStyle: { opacity: 0.1 }
      },
      {
        name: '字节流入',
        type: 'line',
        smooth: true,
        yAxisIndex: 1,
        data: throughputData.bytesIn.map(formatBytesForChart),
        itemStyle: { color: '#52c41a' }
      },
      {
        name: '字节流出',
        type: 'line',
        smooth: true,
        yAxisIndex: 1,
        data: throughputData.bytesOut.map(formatBytesForChart),
        itemStyle: { color: '#faad14' }
      }
    ]
  })

  // 折线图配置 - 消费延迟
  const getLagChartOption = () => ({
    title: { text: '消费延迟趋势', left: 'center', textStyle: { fontSize: 14 } },
    tooltip: { trigger: 'axis' },
    legend: { data: ['总延迟', '消费组数'], bottom: 0 },
    grid: { left: '3%', right: '4%', bottom: '15%', top: '15%', containLabel: true },
    xAxis: { type: 'category', boundaryGap: false, data: lagData.times },
    yAxis: [
      { type: 'value', name: '延迟数' },
      { type: 'value', name: '消费组数', position: 'right' }
    ],
    series: [
      {
        name: '总延迟',
        type: 'line',
        smooth: true,
        data: lagData.totalLag,
        itemStyle: { color: '#f5222d' },
        areaStyle: { opacity: 0.1 }
      },
      {
        name: '消费组数',
        type: 'line',
        smooth: true,
        yAxisIndex: 1,
        data: lagData.consumerGroupCount,
        itemStyle: { color: '#722ed1' }
      }
    ]
  })

  // 折线图配置 - 分区状态
  const getPartitionChartOption = () => ({
    title: { text: '分区状态趋势', left: 'center', textStyle: { fontSize: 14 } },
    tooltip: { trigger: 'axis' },
    legend: { data: ['未同步分区', '离线分区'], bottom: 0 },
    grid: { left: '3%', right: '4%', bottom: '15%', top: '15%', containLabel: true },
    xAxis: { type: 'category', boundaryGap: false, data: partitionData.times },
    yAxis: { type: 'value', name: '数量' },
    series: [
      {
        name: '未同步分区',
        type: 'line',
        smooth: true,
        data: partitionData.underReplicated,
        itemStyle: { color: '#fa8c16' },
        areaStyle: { opacity: 0.1 }
      },
      {
        name: '离线分区',
        type: 'line',
        smooth: true,
        data: partitionData.offline,
        itemStyle: { color: '#f5222d' },
        areaStyle: { opacity: 0.1 }
      }
    ]
  })

  const renderBrokerOverview = (brokerMetrics: BrokerMetrics | null) => {
    if (!brokerMetrics) return null
    return (
      <Row gutter={16}>
        <Col span={6}>
          <Card>
            <Statistic 
              title="消息流入速率" 
              value={brokerMetrics.messages_in_per_sec || 0} 
              suffix="msg/s"
              valueStyle={{ color: '#1890ff' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic 
              title="字节流入速率" 
              value={formatBytes(brokerMetrics.bytes_in_per_sec)}
              valueStyle={{ color: '#52c41a' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic 
              title="字节流出速率" 
              value={formatBytes(brokerMetrics.bytes_out_per_sec)}
              valueStyle={{ color: '#faad14' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic 
              title="未同步分区" 
              value={brokerMetrics.under_replicated_partitions || 0}
              valueStyle={{ color: brokerMetrics.under_replicated_partitions > 0 ? '#f5222d' : '#52c41a' }}
            />
          </Card>
        </Col>
      </Row>
    )
  }

  const tabItems = [
    {
      key: 'overview',
      label: '集群概览',
      children: (
        <>
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
          <Row gutter={16}>
            <Col span={6}>
              <Card>
                <Statistic 
                  title="Broker 数量" 
                  value={metrics?.broker_count || 0} 
                  valueStyle={{ color: '#1890ff' }}
                />
              </Card>
            </Col>
            <Col span={6}>
              <Card>
                <Statistic 
                  title="Topic 数量" 
                  value={metrics?.topic_count || 0} 
                  valueStyle={{ color: '#52c41a' }}
                />
              </Card>
            </Col>
            <Col span={6}>
              <Card>
                <Statistic 
                  title="消费组数量" 
                  value={metrics?.consumer_groups?.length || 0} 
                  valueStyle={{ color: '#faad14' }}
                />
              </Card>
            </Col>
            <Col span={6}>
              <Card>
                <Statistic 
                  title="总消费延迟" 
                  value={metrics?.consumer_groups?.reduce((sum, g) => sum + g.total_lag, 0) || 0}
                  valueStyle={{ color: '#f5222d' }}
                />
              </Card>
            </Col>
          </Row>
          {metrics?.broker_metrics && (
            <div style={{ marginTop: 16 }}>
              <h4>Broker 指标</h4>
              {renderBrokerOverview(metrics.broker_metrics)}
            </div>
          )}
        </>
      )
    },
    {
      key: 'charts',
      label: '历史趋势',
      children: (
        <div>
          <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 8 }}>
            <span>集群: <strong>{selectedCluster?.cluster_name}</strong></span>
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
          </div>
          
          {historyLoading ? (
            <Spin tip="加载历史数据..." />
          ) : throughputData.times.length === 0 ? (
            <Alert
              message="暂无历史数据"
              description="系统会每 30 秒自动采集一次指标，请稍后再查看。确保 VictoriaMetrics 已正确配置且集群已设置 JMX Exporter URL。"
              type="info"
            />
          ) : (
            <Row gutter={[16, 16]}>
              <Col span={24}>
                <Card>
                  <ReactECharts option={getThroughputChartOption()} style={{ height: 300 }} />
                </Card>
              </Col>
              <Col span={24}>
                <Card>
                  <ReactECharts option={getLagChartOption()} style={{ height: 300 }} />
                </Card>
              </Col>
              <Col span={24}>
                <Card>
                  <ReactECharts option={getPartitionChartOption()} style={{ height: 300 }} />
                </Card>
              </Col>
            </Row>
          )}
        </div>
      )
    },
    {
      key: 'broker',
      label: 'Broker 监控',
      children: metrics?.broker_metrics ? (
        <>
          <Row gutter={16}>
            <Col span={6}>
              <Card>
                <Statistic 
                  title="消息流入速率" 
                  value={metrics.broker_metrics.messages_in_per_sec || 0} 
                  suffix="msg/s"
                />
              </Card>
            </Col>
            <Col span={6}>
              <Card>
                <Statistic 
                  title="字节流入速率" 
                  value={formatBytes(metrics.broker_metrics.bytes_in_per_sec)}
                />
              </Card>
            </Col>
            <Col span={6}>
              <Card>
                <Statistic 
                  title="字节流出速率" 
                  value={formatBytes(metrics.broker_metrics.bytes_out_per_sec)}
                />
              </Card>
            </Col>
            <Col span={6}>
              <Card>
                <Statistic 
                  title="未同步分区" 
                  value={metrics.broker_metrics.under_replicated_partitions || 0}
                  valueStyle={{ color: metrics.broker_metrics.under_replicated_partitions > 0 ? '#f5222d' : '#52c41a' }}
                />
              </Card>
            </Col>
          </Row>
          <Row gutter={16} style={{ marginTop: 16 }}>
            <Col span={6}>
              <Card>
                <Statistic 
                  title="离线分区" 
                  value={metrics.broker_metrics.offline_partitions_count || 0}
                  valueStyle={{ color: metrics.broker_metrics.offline_partitions_count > 0 ? '#f5222d' : '#52c41a' }}
                />
              </Card>
            </Col>
            <Col span={6}>
              <Card>
                <Statistic 
                  title="活跃 Controller" 
                  value={metrics.broker_metrics.active_controller_count || 0}
                />
              </Card>
            </Col>
          </Row>
        </>
      ) : (
        <Alert
          message="Broker 指标不可用"
          description="请配置 JMX Exporter URL"
          type="info"
        />
      )
    },
    {
      key: 'consumer',
      label: '消费组监控',
      children: (
        <Table 
          dataSource={metrics?.consumer_groups || []} 
          columns={consumerColumns} 
          rowKey="group_id"
          expandable={expandableConfig}
          pagination={{ pageSize: 20 }}
        />
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
