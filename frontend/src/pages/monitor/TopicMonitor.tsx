import React, { useState, useEffect, useCallback } from 'react'
import { Card, Row, Col, Select, Spin, Statistic, Space, Tag, Alert, Checkbox, Table } from 'antd'
import ReactECharts from 'echarts-for-react'
import dayjs, { Dayjs } from 'dayjs'
import DashboardGrid from '../../components/DashboardGrid'
import { ClusterMetricsResponse, metricsAPI, BatchQueryItem, extractInstantValue } from '../../services/metrics'
import { buildLineChartOption, buildBarChartOption, buildPartitionChartOption, formatBytesForChart } from '../../utils/chartOptions'

interface ClusterOption {
  cluster_id: number
  cluster_name: string
}

interface TopicMonitorProps {
  cluster: ClusterOption
  timeRange: 'quick' | 'custom'
  quickRange: string
  customRange: [Dayjs, Dayjs] | null
  metrics: ClusterMetricsResponse | null
  activeTab: string
}

interface TopicInfo {
  name: string
  partitions: number
  replication_factor: number
  log_size_bytes?: number
}

interface PartitionMetric {
  partition: number
  values: { time: string; value: number }[]
}

const TopicMonitor: React.FC<TopicMonitorProps> = ({ cluster, timeRange, quickRange, customRange, metrics, activeTab }) => {
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
  const [topicLogSizeData, setTopicLogSizeData] = useState<PartitionMetric[]>([])
  const [topicLogEndOffsetData, setTopicLogEndOffsetData] = useState<PartitionMetric[]>([])
  const [topicIsrVsReplicaData, setTopicIsrVsReplicaData] = useState<{ isr: PartitionMetric[]; replica: PartitionMetric[] }>({ isr: [], replica: [] })
  const [topicUnderReplicatedCount, setTopicUnderReplicatedCount] = useState(0)

  /** 获取时间范围 */
  const getTimeRange = useCallback((): { start: Dayjs; end: Dayjs; step: string } => {
    let end: Dayjs, start: Dayjs, step: string
    if (timeRange === 'custom' && customRange) {
      start = customRange[0]; end = customRange[1]
      const dm = end.diff(start, 'minute')
      step = dm <= 30 ? '30s' : dm <= 120 ? '1m' : dm <= 360 ? '2m' : dm <= 1440 ? '5m' : '10m'
    } else {
      end = dayjs()
      switch (quickRange) {
        case '5m': start = end.subtract(5, 'minute'); step = '30s'; break
        case '15m': start = end.subtract(15, 'minute'); step = '30s'; break
        case '30m': start = end.subtract(30, 'minute'); step = '30s'; break
        case '1h': start = end.subtract(1, 'hour'); step = '1m'; break
        case '3h': start = end.subtract(3, 'hour'); step = '2m'; break
        case '6h': start = end.subtract(6, 'hour'); step = '2m'; break
        case '12h': start = end.subtract(12, 'hour'); step = '5m'; break
        case '24h': start = end.subtract(24, 'hour'); step = '5m'; break
        case '2d': start = end.subtract(2, 'day'); step = '10m'; break
        case '7d': start = end.subtract(7, 'day'); step = '30m'; break
        case '30d': start = end.subtract(30, 'day'); step = '1h'; break
        default: start = end.subtract(1, 'hour'); step = '1m'
      }
    }
    return { start, end, step }
  }, [timeRange, quickRange, customRange])

  /** 加载 Topic 列表 */
  const loadTopics = useCallback(async () => {
    if (!cluster) return
    setTopicLoading(true)
    try {
      const now = dayjs()
      const queries: BatchQueryItem[] = [{
        id: 'topic_list',
        query: `kafka_topic_partitions{cluster_id="${cluster.cluster_id}"}`,
        start: now.subtract(1, 'minute').unix(),
        end: now.unix(),
        step: '60s',
      }]
      const res = await metricsAPI.batchQuery(queries)
      const resp = res.data.results['topic_list']
      if (resp && resp.status === 'success') {
        const topicMap = new Map<string, TopicInfo>()
        resp.data.result.forEach(r => {
          const name = r.metric.topic
          if (name && !topicMap.has(name)) {
            topicMap.set(name, { name, partitions: parseInt(r.metric.partition_count || '0') || 1, replication_factor: 1 })
          }
        })
        setTopics(Array.from(topicMap.values()))
      }
    } catch (error) {
      console.error('Failed to load topics', error)
    } finally {
      setTopicLoading(false)
    }
  }, [cluster])

  /** 加载分区级别的指标 */
  const loadPartitionMetrics = useCallback(async () => {
    if (!cluster || !selectedTopic) return
    setTopicLoading(true)
    try {
      const { start, end, step } = getTimeRange()
      const clusterId = cluster.cluster_id
      const s = start.unix(), e = end.unix()

      const queries: BatchQueryItem[] = [
        {
          id: 'produce_rate',
          query: `rate(kafka_topic_partition_current_offset{cluster_id="${clusterId}",topic="${selectedTopic}"}[30s])`,
          start: s, end: e, step,
        },
        {
          id: 'log_size',
          query: `kafka_topic_log_size{cluster_id="${clusterId}",topic="${selectedTopic}"}`,
          start: s, end: e, step,
        },
        {
          id: 'log_end_offset',
          query: `kafka_topic_log_end_offset{cluster_id="${clusterId}",topic="${selectedTopic}"}`,
          start: s, end: e, step,
        },
        {
          id: 'isr_count',
          query: `kafka_topic_partition_isr_count{cluster_id="${clusterId}",topic="${selectedTopic}"}`,
          start: s, end: e, step,
        },
        {
          id: 'replica_count',
          query: `kafka_topic_partition_replica_count{cluster_id="${clusterId}",topic="${selectedTopic}"}`,
          start: s, end: e, step,
        },
        {
          id: 'under_replicated',
          query: `sum(kafka_topic_partition_under_replicated{cluster_id="${clusterId}",topic="${selectedTopic}"})`,
          start: s, end: e, step,
        },
      ]

      if (selectedConsumerGroup) {
        queries.push(
          {
            id: 'consume_rate',
            query: `rate(kafka_consumergroup_current_offset{cluster_id="${clusterId}",topic="${selectedTopic}",consumergroup="${selectedConsumerGroup}"}[30s])`,
            start: s, end: e, step,
          },
          {
            id: 'lag',
            query: `kafka_consumergroup_lag{cluster_id="${clusterId}",topic="${selectedTopic}",consumergroup="${selectedConsumerGroup}"}`,
            start: s, end: e, step,
          },
        )
      }

      const res = await metricsAPI.batchQuery(queries)
      const r = res.data.results

      const parsePartitionMetrics = (result: any[]): PartitionMetric[] => {
        const partitionMap = new Map<number, PartitionMetric>()
        result.forEach(item => {
          const partition = parseInt(item.metric.partition || '0')
          partitionMap.set(partition, {
            partition,
            values: item.values.map((v: [number, string]) => ({ time: dayjs.unix(v[0]).format('HH:mm'), value: parseFloat(v[1]) || 0 }))
          })
        })
        return Array.from(partitionMap.values())
      }

      setPartitionMetrics({
        produceRate: parsePartitionMetrics(r['produce_rate']?.data?.result || []),
        consumeRate: parsePartitionMetrics(r['consume_rate']?.data?.result || []),
        lag: parsePartitionMetrics(r['lag']?.data?.result || []),
      })

      setTopicLogSizeData(parsePartitionMetrics(r['log_size']?.data?.result || []))
      setTopicLogEndOffsetData(parsePartitionMetrics(r['log_end_offset']?.data?.result || []))
      setTopicIsrVsReplicaData({
        isr: parsePartitionMetrics(r['isr_count']?.data?.result || []),
        replica: parsePartitionMetrics(r['replica_count']?.data?.result || []),
      })

      setTopicUnderReplicatedCount(extractInstantValue(r['under_replicated']))

      const allPartitions = new Set<number>()
      ;(r['produce_rate']?.data?.result || []).forEach((item: any) => {
        if (item.metric.partition) allPartitions.add(parseInt(item.metric.partition))
      })
      if (selectedPartitions.length === 0) setSelectedPartitions(Array.from(allPartitions).sort((a, b) => a - b))
    } catch (error) {
      console.error('Failed to load partition metrics', error)
    } finally {
      setTopicLoading(false)
    }
  }, [cluster, selectedTopic, selectedConsumerGroup, getTimeRange])

  // 当切换到 Topic 监控 Tab 时加载 Topic 列表
  useEffect(() => {
    if (activeTab === 'topic' && cluster) {
      loadTopics()
    }
  }, [activeTab, cluster, loadTopics])

  // 当选择 Topic 时加载该 Topic 的消费组列表
  useEffect(() => {
    if (selectedTopic && metrics?.consumer_groups) {
      const cgs = metrics.consumer_groups
        .filter(cg => cg.topics.some(t => t.topic === selectedTopic))
        .map(cg => cg.group_id)
      setTopicConsumerGroups(cgs)
      if (selectedConsumerGroup && !cgs.includes(selectedConsumerGroup)) setSelectedConsumerGroup(null)
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

  // 当选择 Topic 时加载分区指标
  useEffect(() => {
    if (selectedTopic && cluster && activeTab === 'topic') {
      loadPartitionMetrics()
    }
  }, [selectedTopic, selectedConsumerGroup, cluster, quickRange, customRange, timeRange, activeTab, loadPartitionMetrics])

  // ISR vs Replica 图表
  const getIsrVsReplicaChartOption = () => {
    if (topicIsrVsReplicaData.isr.length === 0 && topicIsrVsReplicaData.replica.length === 0) {
      return { title: { text: '分区 ISR 数 vs 副本数', left: 'center', textStyle: { fontSize: 14, color: '#999' } }, graphic: { type: 'text', left: 'center', top: 'middle', style: { text: '暂无数据', fill: '#999', fontSize: 14 } }, xAxis: { type: 'category', data: [] }, yAxis: { type: 'value' }, series: [] }
    }
    const allPartitions = new Set<number>()
    topicIsrVsReplicaData.isr.forEach(p => allPartitions.add(p.partition))
    topicIsrVsReplicaData.replica.forEach(p => allPartitions.add(p.partition))
    const sortedPartitions = Array.from(allPartitions).sort((a, b) => a - b)
    const isrValues = sortedPartitions.map(p => { const m = topicIsrVsReplicaData.isr.find(m => m.partition === p); return m && m.values.length > 0 ? m.values[m.values.length - 1].value : 0 })
    const replicaValues = sortedPartitions.map(p => { const m = topicIsrVsReplicaData.replica.find(m => m.partition === p); return m && m.values.length > 0 ? m.values[m.values.length - 1].value : 0 })
    return buildBarChartOption('分区 ISR 数 vs 副本数', sortedPartitions.map(p => `分区${p}`), [
      { name: 'ISR 数', data: isrValues, color: '#52c41a' },
      { name: '副本数', data: replicaValues, color: '#1890ff' },
    ], '个数')
  }

  // Topic 总速率图表
  const getTotalRateOption = (title: string, data: PartitionMetric[], unit: string, color: string, cgLabel?: string) => {
    if (cgLabel && !selectedConsumerGroup) {
      return { title: { text: title, left: 'center', textStyle: { fontSize: 14, color: '#999' } }, graphic: { type: 'text', left: 'center', top: 'middle', style: { text: '请选择消费组', fill: '#999', fontSize: 14 } }, xAxis: { type: 'category', data: [] }, yAxis: { type: 'value' }, series: [] }
    }
    const hasData = data.some(p => p.values.length > 0)
    if (!hasData) {
      const emptyText = cgLabel ? '该消费组未消费此 Topic' : '暂无数据'
      return { title: { text: title, left: 'center', textStyle: { fontSize: 14, color: '#999' } }, graphic: { type: 'text', left: 'center', top: 'middle', style: { text: emptyText, fill: '#999', fontSize: 14 } }, xAxis: { type: 'category', data: [] }, yAxis: { type: 'value' }, series: [] }
    }
    const allTimes = new Set<string>()
    data.forEach(p => p.values.forEach(v => allTimes.add(v.time)))
    const times = Array.from(allTimes).sort()
    const totalValues = times.map(t => {
      let sum = 0
      data.forEach(p => { const found = p.values.find(v => v.time === t); if (found) sum += found.value })
      return sum
    })
    return buildLineChartOption(title, { times, values: totalValues }, color, unit)
  }

  return (
    <Spin spinning={topicLoading}>
      <Space style={{ marginBottom: 16 }} wrap>
        <Select
          placeholder="选择 Topic"
          value={selectedTopic}
          onChange={(value) => { setSelectedTopic(value); setSelectedPartitions([]) }}
          style={{ width: 200 }}
          options={topics.map(t => ({ label: t.name, value: t.name }))}
          allowClear showSearch
          filterOption={(input, option) => (option?.label ?? '').toLowerCase().includes(input.toLowerCase())}
        />
        <Select
          placeholder="选择消费组"
          value={selectedConsumerGroup}
          onChange={setSelectedConsumerGroup}
          style={{ width: 200 }}
          options={topicConsumerGroups.map(cg => ({ label: cg, value: cg }))}
          allowClear showSearch
          disabled={!selectedTopic}
          filterOption={(input, option) => (option?.label ?? '').toLowerCase().includes(input.toLowerCase())}
        />
      </Space>

      {!selectedTopic ? (
        <Alert message="请选择 Topic" description="选择 Topic 后将显示该 Topic 的详细监控信息" type="info" />
      ) : (
        <>
          <Card size="small" title="Topic 概览" style={{ marginBottom: 16 }}>
            <Row gutter={24}>
              <Col span={6}><Statistic title="Topic 名称" value={selectedTopic} valueStyle={{ fontSize: 16 }} /></Col>
              <Col span={6}><Statistic title="分区数" value={topics.find(t => t.name === selectedTopic)?.partitions || 0} /></Col>
              <Col span={6}><Statistic title="消费组数量" value={topicConsumerGroups.length} /></Col>
              <Col span={6}>
                <Statistic
                  title="总 Lag"
                  value={metrics?.consumer_groups?.filter(cg => cg.topics.some(t => t.topic === selectedTopic)).reduce((sum, cg) => sum + cg.topics.filter(t => t.topic === selectedTopic).reduce((s, t) => s + t.lag, 0), 0) || 0}
                  valueStyle={{ color: '#f5222d' }}
                />
              </Col>
            </Row>
            {topicConsumerGroups.length === 0 ? (
              <Alert message="该 Topic 暂无消费组" type="info" style={{ marginTop: 16 }} />
            ) : (
              <div style={{ marginTop: 16 }}>
                <h4 style={{ marginBottom: 8 }}>消费组列表（点击选中）</h4>
                <Table
                  size="small"
                  dataSource={metrics?.consumer_groups?.filter(cg => cg.topics.some(t => t.topic === selectedTopic)).map(cg => {
                    const topicData = cg.topics.filter(t => t.topic === selectedTopic)
                    return { group_id: cg.group_id, state: cg.state, member_count: cg.member_count || 0, topic_lag: topicData.reduce((s, t) => s + t.lag, 0), topic: selectedTopic, partitions: topicData.length }
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
                  onRow={(record: any) => ({
                    onClick: () => setSelectedConsumerGroup(record.group_id),
                    style: { cursor: 'pointer', backgroundColor: selectedConsumerGroup === record.group_id ? '#e6f7ff' : undefined }
                  })}
                />
              </div>
            )}
          </Card>

          {partitionMetrics.produceRate.length > 0 && (
            <Card size="small" title="分区选择（点击筛选要查看的分区）" style={{ marginBottom: 16 }}>
              <div style={{ maxHeight: 120, overflowY: 'auto' }}>
                <Checkbox.Group value={selectedPartitions} onChange={(values) => setSelectedPartitions(values as number[])}>
                  <Space wrap>
                    {partitionMetrics.produceRate.map(p => p.partition).sort((a, b) => a - b).map(p => (
                      <Checkbox key={p} value={p}>分区 {p}</Checkbox>
                    ))}
                  </Space>
                </Checkbox.Group>
              </div>
            </Card>
          )}

          <DashboardGrid
            storageKey="topic-monitor"
            cols={{ lg: 12, md: 12, sm: 6, xs: 4 }}
            rowHeight={45}
            items={[
              { i: 'total-produce', x: 0, y: 0, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`tp-${selectedTopic}`} option={getTotalRateOption('Topic 生产速率', partitionMetrics.produceRate, 'msg/s', '#1890ff')} style={{ height: 240 }} notMerge={true} /></Card> },
              { i: 'total-consume', x: 4, y: 0, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`tc-${selectedTopic}-${selectedConsumerGroup}`} option={getTotalRateOption('消费组消费速率', partitionMetrics.consumeRate, 'msg/s', '#52c41a', 'cg')} style={{ height: 240 }} notMerge={true} /></Card> },
              { i: 'total-lag', x: 8, y: 0, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`tl-${selectedTopic}-${selectedConsumerGroup}`} option={getTotalRateOption('消费组 Lag', partitionMetrics.lag, 'Lag', '#f5222d', 'cg')} style={{ height: 240 }} notMerge={true} /></Card> },
              { i: 'under-replicated', x: 0, y: 6, w: 3, h: 2, component: <Card size="small"><Statistic title="Under Replicated 分区" value={topicUnderReplicatedCount} valueStyle={{ color: topicUnderReplicatedCount === 0 ? '#52c41a' : '#f5222d', fontSize: 20 }} /></Card> },
              ...(selectedPartitions.length > 0 ? [
                { i: 'partition-produce', x: 0, y: 8, w: 12, h: 7, component: <Card size="small"><ReactECharts key={`pp-${selectedTopic}-${selectedPartitions.join('-')}`} option={buildPartitionChartOption('Topic 生产速率（按分区）', partitionMetrics.produceRate, selectedPartitions, 'msg/s')} style={{ height: 280 }} notMerge={true} /></Card> },
                { i: 'partition-consume', x: 0, y: 15, w: 6, h: 7, component: <Card size="small"><ReactECharts key={`pc-${selectedTopic}-${selectedConsumerGroup}-${selectedPartitions.join('-')}`} option={buildPartitionChartOption('消费组消费速率（按分区）', partitionMetrics.consumeRate, selectedPartitions, 'msg/s', undefined, selectedConsumerGroup ? '暂无数据' : '请选择消费组')} style={{ height: 280 }} notMerge={true} /></Card> },
                { i: 'partition-lag', x: 6, y: 15, w: 6, h: 7, component: <Card size="small"><ReactECharts key={`pl-${selectedTopic}-${selectedConsumerGroup}-${selectedPartitions.join('-')}`} option={buildPartitionChartOption('消费组 Lag（按分区）', partitionMetrics.lag, selectedPartitions, 'Lag', (v: number) => v.toLocaleString(), selectedConsumerGroup ? '暂无数据' : '请选择消费组')} style={{ height: 280 }} notMerge={true} /></Card> },
              ] : []),
              { i: 'log-size', x: 0, y: 22, w: 12, h: 7, component: <Card size="small"><ReactECharts key={`ls-${selectedTopic}-${selectedPartitions.join('-')}`} option={buildPartitionChartOption('Topic 日志大小（按分区）', topicLogSizeData, selectedPartitions, 'bytes', formatBytesForChart)} style={{ height: 280 }} notMerge={true} /></Card> },
              { i: 'log-end-offset', x: 0, y: 29, w: 12, h: 7, component: <Card size="small"><ReactECharts key={`leo-${selectedTopic}-${selectedPartitions.join('-')}`} option={buildPartitionChartOption('Topic LogEndOffset（按分区）', topicLogEndOffsetData, selectedPartitions, 'Offset')} style={{ height: 280 }} notMerge={true} /></Card> },
              { i: 'isr-vs-replica', x: 0, y: 36, w: 12, h: 7, component: <Card size="small"><ReactECharts key={`ivr-${selectedTopic}`} option={getIsrVsReplicaChartOption()} style={{ height: 280 }} notMerge={true} /></Card> },
            ]}
          />
        </>
      )}
    </Spin>
  )
}

export default React.memo(TopicMonitor)
