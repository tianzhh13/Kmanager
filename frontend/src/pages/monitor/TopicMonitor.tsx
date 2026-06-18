import React, { useState, useEffect, useCallback, useRef } from 'react'
import { Select, Spin, Alert } from 'antd'
import EChartsReact from 'echarts-for-react/lib/core'
import echarts from '../../utils/echarts'
import dayjs, { Dayjs } from 'dayjs'
import DashboardGrid from '../../components/DashboardGrid'
import { usePromqlOverrides, useDefaultPromqls, PromqlDebugger, PromqlDebugButton } from '../../components/PromqlDebugger'
import { ClusterMetricsResponse, metricsAPI, BatchQueryItem, extractInstantValue } from '../../services/metrics'
import {
  createAreaChartOption,
  createGroupedBarChartOption,
  buildPartitionChartOption,
  formatBytesForChart,
} from '../../utils/chartOptions'
import { StatCard, SectionTitle, LabelTag } from '../../components/bento'

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
  jmxAvailable?: boolean
  initialTopic?: string | null
  initialConsumerGroup?: string | null
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

const TopicMonitor: React.FC<TopicMonitorProps> = ({ cluster, timeRange, quickRange, customRange, metrics, activeTab, jmxAvailable, initialTopic, initialConsumerGroup }) => {
  const [topics, setTopics] = useState<TopicInfo[]>([])
  const [selectedTopic, setSelectedTopic] = useState<string | null>(null)
  const [selectedConsumerGroup, setSelectedConsumerGroup] = useState<string | null>(null)
  const [topicConsumerGroups, setTopicConsumerGroups] = useState<string[]>([])
  const [topicLoading, setTopicLoading] = useState(false)
  const [selectedPartitions, setSelectedPartitions] = useState<number[]>([])
  const [allPartitionsList, setAllPartitionsList] = useState<number[]>([])
  const [partitionMetrics, setPartitionMetrics] = useState<{
    produceRate: PartitionMetric[]
    consumeRate: PartitionMetric[]
    lag: PartitionMetric[]
  }>({ produceRate: [], consumeRate: [], lag: [] })
  const [topicLogSizeData, setTopicLogSizeData] = useState<PartitionMetric[]>([])
  const [topicLogEndOffsetData, setTopicLogEndOffsetData] = useState<PartitionMetric[]>([])
  const [topicIsrVsReplicaData, setTopicIsrVsReplicaData] = useState<{ isr: PartitionMetric[]; replica: PartitionMetric[] }>({ isr: [], replica: [] })
  const [topicUnderReplicatedCount, setTopicUnderReplicatedCount] = useState(0)
  const [debugOpen, setDebugOpen] = useState(false)
  const initialConsumerGroupRef = useRef<string | null>(initialConsumerGroup || null)
  const { overrides, getQ, setOverride, resetOverride, resetAll } = usePromqlOverrides('topic_monitor')
  const { q, defaultPromqls } = useDefaultPromqls(getQ)

  const queryLabels: Record<string, string> = {
    produce_rate: 'Topic 生产速率',
    consume_rate: '消费组消费速率',
    lag: '消费组 Lag',
    log_size: 'Topic 日志大小（按分区）',
    log_end_offset: 'Topic LogEndOffset（按分区）',
    isr_count: '分区 ISR 数 vs 副本数（ISR 列）',
    replica_count: '分区 ISR 数 vs 副本数（副本列）',
    under_replicated: 'Under Replicated 分区',
  }

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

  const loadTopics = useCallback(async () => {
    if (!cluster) return
    setTopicLoading(true)
    try {
      const now = dayjs()
      const queries: BatchQueryItem[] = [{
        id: 'topic_list',
        query: `kafka_topic_partitions{app="kmanager",cluster_id="${cluster.cluster_id}"}`,
        start: now.subtract(1, 'minute').unix(),
        end: now.unix(),
        step: '60s',
      }]
      const res = await metricsAPI.batchQuery(queries)
      const resp = res.data.results['topic_list']
      if (resp && resp.status === 'success') {
        const topicMap = new Map<string, TopicInfo>()
        resp.data.result.forEach((r: any) => {
          const name = r.metric.topic
          if (name && !topicMap.has(name)) {
            // kafka_topic_partitions 的值本身就是分区数
            const partitions = r.value ? parseInt(r.value[1]) || 1 : 1
            topicMap.set(name, { name, partitions, replication_factor: 1 })
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
          query: q('produce_rate', `rate(kafka_topic_partition_current_offset{app="kmanager",cluster_id="${clusterId}",topic="${selectedTopic}"}[30s])`),
          start: s, end: e, step,
        },
      ]

      if (jmxAvailable) {
        queries.push(
          { id: 'log_size', query: q('log_size', `max by (partition) (kafka_log_log_size{app="kmanager",cluster_id="${clusterId}",topic="${selectedTopic}"})`), start: s, end: e, step },
          { id: 'log_end_offset', query: q('log_end_offset', `max by (partition) (kafka_log_log_logendoffset{app="kmanager",cluster_id="${clusterId}",topic="${selectedTopic}"})`), start: s, end: e, step },
          { id: 'isr_count', query: q('isr_count', `max by (partition) (kafka_cluster_partition_insyncreplicascount{app="kmanager",cluster_id="${clusterId}",topic="${selectedTopic}"})`), start: s, end: e, step },
          { id: 'replica_count', query: q('replica_count', `max by (partition) (kafka_cluster_partition_replicascount{app="kmanager",cluster_id="${clusterId}",topic="${selectedTopic}"})`), start: s, end: e, step },
          { id: 'under_replicated', query: q('under_replicated', `sum(kafka_cluster_partition_underreplicated{app="kmanager",cluster_id="${clusterId}",topic="${selectedTopic}"})`), start: s, end: e, step },
        )
      }

      if (selectedConsumerGroup) {
        queries.push(
          {
            id: 'consume_rate',
            query: q('consume_rate', `rate(kafka_consumergroup_current_offset{app="kmanager",cluster_id="${clusterId}",topic="${selectedTopic}",consumergroup="${selectedConsumerGroup}"}[30s])`),
            start: s, end: e, step,
          },
          {
            id: 'lag',
            query: q('lag', `kafka_consumergroup_lag{app="kmanager",cluster_id="${clusterId}",topic="${selectedTopic}",consumergroup="${selectedConsumerGroup}"}`),
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

      setTopicUnderReplicatedCount(extractInstantValue(r['under_replicated']) ?? 0)

      const allParts = new Set<number>()
      // Collect partitions from all available metric results
      Object.values(r).forEach((result: any) => {
        if (result?.data?.result) {
          result.data.result.forEach((item: any) => {
            if (item.metric.partition) allParts.add(parseInt(item.metric.partition))
          })
        }
      })
      // Fallback: if no partition data from metrics, generate from topic info
      let partsList = Array.from(allParts).sort((a, b) => a - b)
      if (partsList.length === 0) {
        const topicInfo = topics.find(t => t.name === selectedTopic)
        if (topicInfo && topicInfo.partitions > 0) {
          partsList = Array.from({ length: topicInfo.partitions }, (_, i) => i)
        }
      }
      setAllPartitionsList(partsList)
      if (selectedPartitions.length === 0) setSelectedPartitions(partsList)
    } catch (error) {
      console.error('Failed to load partition metrics', error)
    } finally {
      setTopicLoading(false)
    }
  }, [cluster, selectedTopic, selectedConsumerGroup, getTimeRange, jmxAvailable, overrides])

  // Load topics on tab activation
  useEffect(() => {
    if (activeTab === 'topic' && cluster) {
      loadTopics().then(() => {
        // 如果有 initialTopic 参数，加载完成后自动选中
        if (initialTopic) {
          // 延迟一下等待 topics 状态更新
          setTimeout(() => {
            setSelectedTopic(initialTopic)
            // 从 ref 获取 initialConsumerGroup，避免被 useEffect 清除
            if (initialConsumerGroupRef.current) {
              setSelectedConsumerGroup(initialConsumerGroupRef.current)
              initialConsumerGroupRef.current = null // 用完清除
            }
          }, 100)
        }
      })
    }
  }, [activeTab, cluster, loadTopics, initialTopic])

  // Load consumer groups for selected topic
  useEffect(() => {
    if (selectedTopic && metrics?.consumer_groups) {
      const cgs = metrics.consumer_groups
        .filter(cg => cg.topics.some(t => t.topic === selectedTopic))
        .map(cg => cg.group_id)
      setTopicConsumerGroups(cgs)
      // 只有当 initialConsumerGroupRef 为空时才清除 selectedConsumerGroup
      if (selectedConsumerGroup && !cgs.includes(selectedConsumerGroup) && !initialConsumerGroupRef.current) {
        setSelectedConsumerGroup(null)
      }
    } else {
      setTopicConsumerGroups([])
      // 只有当 initialConsumerGroupRef 为空时才清除 selectedConsumerGroup
      if (!initialConsumerGroupRef.current) {
        setSelectedConsumerGroup(null)
      }
    }
  }, [selectedTopic, metrics?.consumer_groups])

  // Reset partition metrics on topic change
  useEffect(() => {
    setPartitionMetrics({ produceRate: [], consumeRate: [], lag: [] })
    setSelectedPartitions([])
    setAllPartitionsList([])
  }, [selectedTopic])

  // Reset consumer metrics on consumer group change
  useEffect(() => {
    setPartitionMetrics(prev => ({ ...prev, consumeRate: [], lag: [] }))
  }, [selectedConsumerGroup])

  // Load partition metrics on topic/consumer/range change
  useEffect(() => {
    if (selectedTopic && cluster && activeTab === 'topic') loadPartitionMetrics()
  }, [selectedTopic, selectedConsumerGroup, cluster, quickRange, customRange, timeRange, activeTab, loadPartitionMetrics])

  // ─── Chart builders ───

  const getTotalRateOption = (data: PartitionMetric[], unit: string, color: string, cgLabel?: string) => {
    if (cgLabel && !selectedConsumerGroup) return createAreaChartOption('', { times: [], values: [] })
    const hasData = data.some(p => p.values.length > 0)
    if (!hasData) return createAreaChartOption('', { times: [], values: [] })
    const allTimes = new Set<string>()
    data.forEach(p => p.values.forEach(v => allTimes.add(v.time)))
    const times = Array.from(allTimes).sort()
    const totalValues = times.map(t => {
      let sum = 0
      data.forEach(p => { const found = p.values.find(v => v.time === t); if (found) sum += found.value })
      return sum
    })
    return createAreaChartOption('', { times, values: totalValues }, color, unit)
  }

  const getIsrVsReplicaChartOption = () => {
    if (topicIsrVsReplicaData.isr.length === 0 && topicIsrVsReplicaData.replica.length === 0) {
      return createGroupedBarChartOption([], [], '')
    }
    const allParts = new Set<number>()
    topicIsrVsReplicaData.isr.forEach(p => allParts.add(p.partition))
    topicIsrVsReplicaData.replica.forEach(p => allParts.add(p.partition))
    const sorted = Array.from(allParts).sort((a, b) => a - b)
    const categories = sorted.map(p => `分区${p}`)
    const isrValues = sorted.map(p => {
      const m = topicIsrVsReplicaData.isr.find(m => m.partition === p)
      return m && m.values.length > 0 ? m.values[m.values.length - 1].value : 0
    })
    const replicaValues = sorted.map(p => {
      const m = topicIsrVsReplicaData.replica.find(m => m.partition === p)
      return m && m.values.length > 0 ? m.values[m.values.length - 1].value : 0
    })
    return createGroupedBarChartOption(categories, [
      { name: 'ISR 数', data: isrValues, color: '#10b981' },
      { name: '副本数', data: replicaValues, color: '#3b82f6' },
    ], '个数')
  }

  const chartKey = `${selectedTopic}-${selectedConsumerGroup}-${selectedPartitions.join('-')}`

  // Consumer group data for custom rows
  const consumerGroupRows = React.useMemo(() => {
    if (!selectedTopic || !metrics?.consumer_groups) return []
    return metrics.consumer_groups
      .filter(cg => cg.topics.some(t => t.topic === selectedTopic))
      .map(cg => {
        const topicData = cg.topics.find(t => t.topic === selectedTopic)
        return {
          group_id: cg.group_id,
          state: cg.state,
          member_count: cg.member_count || 0,
          partitions: topicData?.partitions?.length || 0,
          lag: topicData?.lag || 0,
        }
      })
  }, [selectedTopic, metrics?.consumer_groups])

  const cgStateColor = (state: string): 'green' | 'red' | 'orange' | 'blue' | 'neutral' => {
    switch (state) {
      case 'Stable': return 'green'
      case 'Dead': return 'red'
      case 'Empty': return 'orange'
      case 'PreparingRebalance':
      case 'CompletingRebalance': return 'orange'
      default: return 'blue'
    }
  }

  return (
    <>
    <Spin spinning={topicLoading}>
      {/* Topic + Consumer Group Selectors + PromQL Debug */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 16 }}>
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
        <PromqlDebugButton onClick={() => setDebugOpen(true)} overrideCount={Object.keys(overrides).length} />
      </div>

      {!selectedTopic ? (
        <Alert message="请选择 Topic" description="选择 Topic 后将显示该 Topic 的详细监控信息" type="info" />
      ) : (
        <>
          {/* Topic Overview Stat Cards */}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 12, marginBottom: 20 }}>
            <div className="bento-card"><div className="bento-card-inner" style={{ padding: '16px 20px' }}>
              <div className="stat-label">TOPIC</div>
              <div style={{ fontSize: 16, fontWeight: 700, marginTop: 6, fontFamily: "'JetBrains Mono', monospace" }}>{selectedTopic}</div>
            </div></div>
            <StatCard label="PARTITIONS" value={topics.find(t => t.name === selectedTopic)?.partitions || 0} />
            <StatCard label="CG COUNT" value={topicConsumerGroups.length} />
            <StatCard
              label="TOTAL LAG"
              value={metrics?.consumer_groups?.filter(cg => cg.topics.some(t => t.topic === selectedTopic)).reduce((sum, cg) => sum + cg.topics.filter(t => t.topic === selectedTopic).reduce((s, t) => s + t.lag, 0), 0) || 0}
              color="#ef4444"
            />
          </div>

          {/* Consumer Group Custom Rows */}
          {topicConsumerGroups.length > 0 && (
            <div style={{ marginBottom: 20 }}>
              <SectionTitle title="消费组列表" />
              <div style={{ overflowX: 'auto' }}>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 100px 80px 90px 100px', gap: 0, minWidth: 500 }}>
                  <div className="bento-grid-header">GROUP</div>
                  <div className="bento-grid-header">STATE</div>
                  <div className="bento-grid-header">MEMBERS</div>
                  <div className="bento-grid-header">PARTITIONS</div>
                  <div className="bento-grid-header">LAG</div>
                  {consumerGroupRows.map(cg => (
                    <React.Fragment key={cg.group_id}>
                      <div
                        className="bento-grid-cell mono"
                        style={{ cursor: 'pointer', backgroundColor: selectedConsumerGroup === cg.group_id ? 'rgba(249,115,22,0.08)' : undefined }}
                        onClick={() => setSelectedConsumerGroup(cg.group_id)}
                      >
                        {cg.group_id}
                      </div>
                      <div className="bento-grid-cell" onClick={() => setSelectedConsumerGroup(cg.group_id)} style={{ cursor: 'pointer', backgroundColor: selectedConsumerGroup === cg.group_id ? 'rgba(249,115,22,0.08)' : undefined }}>
                        <LabelTag text={cg.state} color={cgStateColor(cg.state)} />
                      </div>
                      <div className="bento-grid-cell mono" onClick={() => setSelectedConsumerGroup(cg.group_id)} style={{ cursor: 'pointer', backgroundColor: selectedConsumerGroup === cg.group_id ? 'rgba(249,115,22,0.08)' : undefined }}>{cg.member_count}</div>
                      <div className="bento-grid-cell mono" onClick={() => setSelectedConsumerGroup(cg.group_id)} style={{ cursor: 'pointer', backgroundColor: selectedConsumerGroup === cg.group_id ? 'rgba(249,115,22,0.08)' : undefined }}>{cg.partitions}</div>
                      <div className="bento-grid-cell mono" onClick={() => setSelectedConsumerGroup(cg.group_id)} style={{ cursor: 'pointer', backgroundColor: selectedConsumerGroup === cg.group_id ? 'rgba(249,115,22,0.08)' : undefined }}>{cg.lag?.toLocaleString() || 0}</div>
                    </React.Fragment>
                  ))}
                </div>
              </div>
            </div>
          )}

          {topicConsumerGroups.length === 0 && (
            <Alert message="该 Topic 暂无消费组" type="info" style={{ marginBottom: 16 }} />
          )}

          {/* Partition Multi-Select */}
          {allPartitionsList.length > 0 && (
            <div style={{ marginBottom: 16, display: 'flex', alignItems: 'center', gap: 12 }}>
              <span style={{ fontSize: 13, fontWeight: 600, color: '#57534e', whiteSpace: 'nowrap' }}>Partitions</span>
              <Select
                mode="multiple"
                placeholder="选择分区"
                value={selectedPartitions}
                onChange={(values) => setSelectedPartitions(values as number[])}
                style={{ minWidth: 240, maxWidth: 480 }}
                options={allPartitionsList.map(p => ({ label: `Partition ${p}`, value: p }))}
                allowClear
                maxTagCount={3}
                maxTagPlaceholder={(omitted) => `+${omitted.length}`}
                dropdownRender={(menu) => (
                  <div>
                    <div style={{ display: 'flex', justifyContent: 'space-between', padding: '6px 12px', borderBottom: '1px solid #ebe8e3' }}>
                      <a style={{ fontSize: 12, fontWeight: 600, color: '#f97316' }} onClick={() => setSelectedPartitions([...allPartitionsList])}>全选</a>
                      <a style={{ fontSize: 12, fontWeight: 600, color: '#f97316' }} onClick={() => setSelectedPartitions([])}>清空</a>
                    </div>
                    {menu}
                  </div>
                )}
              />
            </div>
          )}

          {/* Charts */}
          <DashboardGrid
            storageKey="topic-monitor"
            cols={{ lg: 12, md: 12, sm: 6, xs: 4 }}
            rowHeight={45}
            items={[
              // Topic-level rates
              { i: 'total-produce', x: 0, y: 0, w: 4, h: 6, component:
                <div className="bento-card"><div className="bento-card-inner">
                  <SectionTitle title="Topic 生产速率" />
                  <EChartsReact echarts={echarts} key={`tp-${chartKey}`} option={getTotalRateOption(partitionMetrics.produceRate, 'msg/s', '#f97316')} style={{ height: 240 }} notMerge={true} />
                </div></div>,
              },
              { i: 'total-consume', x: 4, y: 0, w: 4, h: 6, component:
                <div className="bento-card"><div className="bento-card-inner">
                  <SectionTitle title="消费组消费速率" />
                  <EChartsReact echarts={echarts} key={`tc-${chartKey}`} option={getTotalRateOption(partitionMetrics.consumeRate, 'msg/s', '#10b981', 'cg')} style={{ height: 240 }} notMerge={true} />
                </div></div>,
              },
              { i: 'total-lag', x: 8, y: 0, w: 4, h: 6, component:
                <div className="bento-card"><div className="bento-card-inner">
                  <SectionTitle title="消费组 Lag" />
                  <EChartsReact echarts={echarts} key={`tl-${chartKey}`} option={getTotalRateOption(partitionMetrics.lag, 'Lag', '#ef4444', 'cg')} style={{ height: 240 }} notMerge={true} />
                </div></div>,
              },
              // Partition-level charts
              ...(selectedPartitions.length > 0 ? [
                { i: 'partition-produce', x: 0, y: 6, w: 12, h: 7, component:
                  <div className="bento-card"><div className="bento-card-inner">
                    <SectionTitle title="分区生产速率" />
                    <EChartsReact echarts={echarts} key={`pp-${chartKey}`} option={buildPartitionChartOption('', partitionMetrics.produceRate, selectedPartitions, 'msg/s')} style={{ height: 260 }} notMerge={true} />
                  </div></div>,
                },
                { i: 'partition-consume', x: 0, y: 13, w: 6, h: 7, component:
                  <div className="bento-card"><div className="bento-card-inner">
                    <SectionTitle title="分区消费速率" />
                    <EChartsReact echarts={echarts} key={`pc-${chartKey}`} option={buildPartitionChartOption('', partitionMetrics.consumeRate, selectedPartitions, 'msg/s', undefined, selectedConsumerGroup ? undefined : '请选择消费组')} style={{ height: 260 }} notMerge={true} />
                  </div></div>,
                },
                { i: 'partition-lag', x: 6, y: 13, w: 6, h: 7, component:
                  <div className="bento-card"><div className="bento-card-inner">
                    <SectionTitle title="分区 Lag" />
                    <EChartsReact echarts={echarts} key={`pl-${chartKey}`} option={buildPartitionChartOption('', partitionMetrics.lag, selectedPartitions, 'Lag', (v: number) => v.toLocaleString(), selectedConsumerGroup ? undefined : '请选择消费组')} style={{ height: 260 }} notMerge={true} />
                  </div></div>,
                },
                // JMX charts
                ...(jmxAvailable ? [
                  { i: 'log-size', x: 0, y: 20, w: 12, h: 7, component:
                    <div className="bento-card"><div className="bento-card-inner">
                      <SectionTitle title="日志大小" />
                      <EChartsReact echarts={echarts} key={`ls-${chartKey}`} option={buildPartitionChartOption('', topicLogSizeData, selectedPartitions, 'bytes', formatBytesForChart)} style={{ height: 260 }} notMerge={true} />
                    </div></div>,
                  },
                  { i: 'log-end-offset', x: 0, y: 27, w: 12, h: 7, component:
                    <div className="bento-card"><div className="bento-card-inner">
                      <SectionTitle title="LogEndOffset" />
                      <EChartsReact echarts={echarts} key={`leo-${chartKey}`} option={buildPartitionChartOption('', topicLogEndOffsetData, selectedPartitions, 'Offset')} style={{ height: 260 }} notMerge={true} />
                    </div></div>,
                  },
                  { i: 'isr-vs-replica', x: 0, y: 34, w: 12, h: 7, component:
                    <div className="bento-card"><div className="bento-card-inner">
                      <SectionTitle title="ISR vs Replica" />
                      <EChartsReact echarts={echarts} key={`ivr-${selectedTopic}`} option={getIsrVsReplicaChartOption()} style={{ height: 240 }} notMerge={true} />
                    </div></div>,
                  },
                  { i: 'under-replicated', x: 0, y: 41, w: 12, h: 2, component:
                    <div className="bento-card"><div className="bento-card-inner" style={{ padding: '12px 20px', display: 'flex', alignItems: 'center', gap: 12 }}>
                      <div className="stat-label" style={{ margin: 0 }}>UNDER-REPLICATED PARTITIONS (JMX)</div>
                      <div className="stat-value" style={{ fontSize: 22, color: topicUnderReplicatedCount === 0 ? '#10b981' : '#ef4444' }}>{topicUnderReplicatedCount}</div>
                      <LabelTag text={topicUnderReplicatedCount === 0 ? 'NORMAL' : 'WARNING'} color={topicUnderReplicatedCount === 0 ? 'green' : 'red'} />
                    </div></div>,
                  },
                ] : []),
              ] : []),
            ]}
          />
        </>
      )}
    </Spin>
    <PromqlDebugger
      open={debugOpen}
      onClose={() => setDebugOpen(false)}
      defaultPromqls={defaultPromqls.current}
      overrides={overrides}
      onSetOverride={setOverride}
      onResetOverride={resetOverride}
      onResetAll={resetAll}
      labelMap={queryLabels}
    />
    </>
  )
}

export default React.memo(TopicMonitor)
