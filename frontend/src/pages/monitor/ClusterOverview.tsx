import React, { useState, useEffect, useCallback } from 'react'
import { Alert } from 'antd'
import EChartsReact from 'echarts-for-react/lib/core'
import echarts from '../../utils/echarts'
import dayjs, { Dayjs } from 'dayjs'
import DashboardGrid from '../../components/DashboardGrid'
import { usePromqlOverrides, useDefaultPromqls, PromqlDebugger, PromqlDebugButton } from '../../components/PromqlDebugger'
import { ClusterMetricsResponse, metricsAPI, BatchQueryItem, extractInstantValue } from '../../services/metrics'
import { buildLineChartOption, formatBytesForChart } from '../../utils/chartOptions'
import { StatCard } from '../../components/bento'

interface ClusterOption {
  cluster_id: number
  cluster_name: string
}

interface ClusterOverviewProps {
  cluster: ClusterOption
  timeRange: 'quick' | 'custom'
  quickRange: string
  customRange: [Dayjs, Dayjs] | null
  metrics: ClusterMetricsResponse | null
  jmxAvailable?: boolean
}

const ClusterOverview: React.FC<ClusterOverviewProps> = ({ cluster, timeRange, quickRange, customRange, metrics, jmxAvailable }) => {
  const [overviewStats, setOverviewStats] = useState({
    topicPartitionTotal: null as number | null,
    consumerGroupMemberCount: null as number | null,
    consumerGroupMemberTotal: null as number | null,
    isrTotal: null as number | null,
    nonPreferredLeaderCount: null as number | null,
    activeBrokerCount: null as number | null,
    fencedBrokerCount: null as number | null,
    globalPartitionCount: null as number | null,
    globalTopicCount: null as number | null,
    preferredReplicaImbalance: null as number | null,
    offlinePartitionsCount: null as number | null,
    activeControllerCount: null as number | null,
    offlineLogDirectoryCount: null as number | null,
    logDirectoryOffline: null as number | null,
  })

  type TimeSeries = { times: string[]; values: (number | null)[] }
  const [produceRateData, setProduceRateData] = useState<TimeSeries>({ times: [], values: [] })
  const [consumeRateData, setConsumeRateData] = useState<TimeSeries>({ times: [], values: [] })
  const [lagTrendData, setLagTrendData] = useState<TimeSeries>({ times: [], values: [] })
  const [bytesInData, setBytesInData] = useState<TimeSeries>({ times: [], values: [] })
  const [bytesOutData, setBytesOutData] = useState<TimeSeries>({ times: [], values: [] })
  const [bytesRejectedData, setBytesRejectedData] = useState<TimeSeries>({ times: [], values: [] })
  const [messagesInRateData, setMessagesInRateData] = useState<TimeSeries>({ times: [], values: [] })
  const [failedProduceRateData, setFailedProduceRateData] = useState<TimeSeries>({ times: [], values: [] })
  const [failedFetchRateData, setFailedFetchRateData] = useState<TimeSeries>({ times: [], values: [] })
  const [dataCorruptionStats, setDataCorruptionStats] = useState({
    invalidMagicNumber: null as number | null,
    invalidCrc: null as number | null,
    invalidOffset: null as number | null,
  })
  const [debugOpen, setDebugOpen] = useState(false)
  const { overrides, getQ, setOverride, resetOverride, resetAll } = usePromqlOverrides('cluster_overview')
  const { q, defaultPromqls } = useDefaultPromqls(getQ)

  const queryLabels: Record<string, string> = {
    topicPartitions: '分区总数',
    cgMemberCount: '消费组数量',
    cgMemberTotal: '消费组成员',
    isrTotal: 'ISR 总数',
    nonPreferred: '非首选 Leader',
    activeBroker: '活跃 Broker',
    fencedBroker: '不健康 Broker',
    globalPartition: '全局分区数',
    globalTopic: '全局 Topic 数',
    replicaImbalance: '副本不均衡',
    offlinePartitions: '离线分区数',
    activeController: '活跃 Controller',
    offlineLogDirCount: '离线日志目录',
    logDirOffline: '日志目录状态',
    invalidMagic: '无效 Magic',
    invalidCrc: '无效 CRC',
    invalidOffset: '无效 Offset',
    produceRate: '集群生产速率',
    consumeRate: '集群消费速率',
    lagTrend: '消费者组总 Lag',
    bytesIn: '字节流入速率',
    bytesOut: '字节流出速率',
    bytesRejected: '拒绝字节速率',
    failedProduce: '生产请求失败率',
    failedFetch: '拉取请求失败率',
    messagesIn: '消息流入速率',
  }

  /** 获取时间范围 */
  const getTimeRange = useCallback((): { start: Dayjs; end: Dayjs; step: string } => {
    let end: Dayjs
    let start: Dayjs
    let step: string

    if (timeRange === 'custom' && customRange) {
      start = customRange[0]
      end = customRange[1]
      const durationMinutes = end.diff(start, 'minute')
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

  /** 加载历史数据（单次批量请求替代 N 次独立请求） */
  const loadHistory = useCallback(async () => {
    if (!cluster) return

    try {
      const clusterId = cluster.cluster_id.toString()
      const { start, end, step } = getTimeRange()
      const instantStart = dayjs().subtract(1, 'minute').unix()
      const instantEnd = dayjs().unix()

      // 构建所有查询（AdminClient 基础查询 + JMX 条件查询）
      const queries: BatchQueryItem[] = [
        // 即时查询：AdminClient 指标（始终查询）
        { id: 'topicPartitions', query: q('topicPartitions', `sum(kafka_topic_partitions{cluster_id="${clusterId}",topic!~"__.*"})`), start: instantStart, end: instantEnd, step: '60s' },
        { id: 'cgMemberCount', query: q('cgMemberCount', `count(kafka_consumergroup_members{cluster_id="${clusterId}",consumergroup!~"__.*"})`), start: instantStart, end: instantEnd, step: '60s' },
        { id: 'cgMemberTotal', query: q('cgMemberTotal', `sum(kafka_consumergroup_members{cluster_id="${clusterId}",consumergroup!~"__.*"})`), start: instantStart, end: instantEnd, step: '60s' },
        { id: 'isrTotal', query: q('isrTotal', `sum(kafka_topic_partition_in_sync_replica{cluster_id="${clusterId}"})`), start: instantStart, end: instantEnd, step: '60s' },
        { id: 'nonPreferred', query: q('nonPreferred', `count(kafka_topic_partition_leader_is_preferred{cluster_id="${clusterId}"}<1)`), start: instantStart, end: instantEnd, step: '60s' },
        // 即时查询：JMX 指标（仅 JMX Exporter 可用时才查）
        ...(jmxAvailable ? [
          { id: 'activeBroker', query: q('activeBroker', `max(kafka_controller_kafkacontroller_activebrokercount{cluster_id="${clusterId}"})`), start: instantStart, end: instantEnd, step: '60s' },
          { id: 'fencedBroker', query: q('fencedBroker', `max(kafka_controller_kafkacontroller_fencedbrokercount{cluster_id="${clusterId}"})`), start: instantStart, end: instantEnd, step: '60s' },
          { id: 'globalPartition', query: q('globalPartition', `max(kafka_controller_kafkacontroller_globalpartitioncount{cluster_id="${clusterId}"})`), start: instantStart, end: instantEnd, step: '60s' },
          { id: 'globalTopic', query: q('globalTopic', `max(kafka_controller_kafkacontroller_globaltopiccount{cluster_id="${clusterId}"})`), start: instantStart, end: instantEnd, step: '60s' },
          { id: 'replicaImbalance', query: q('replicaImbalance', `max(kafka_controller_kafkacontroller_preferredreplicaimbalancecount{cluster_id="${clusterId}"})`), start: instantStart, end: instantEnd, step: '60s' },
          { id: 'offlinePartitions', query: q('offlinePartitions', `max(kafka_controller_kafkacontroller_offlinepartitionscount{cluster_id="${clusterId}"})`), start: instantStart, end: instantEnd, step: '60s' },
          { id: 'activeController', query: q('activeController', `max(kafka_controller_kafkacontroller_activecontrollercount{cluster_id="${clusterId}"})`), start: instantStart, end: instantEnd, step: '60s' },
          { id: 'offlineLogDirCount', query: q('offlineLogDirCount', `max(kafka_log_logmanager_offlinelogdirectorycount{cluster_id="${clusterId}"})`), start: instantStart, end: instantEnd, step: '60s' },
          { id: 'logDirOffline', query: q('logDirOffline', `max(kafka_log_logmanager_logdirectoryoffline{cluster_id="${clusterId}"})`), start: instantStart, end: instantEnd, step: '60s' },
          { id: 'invalidMagic', query: q('invalidMagic', `sum(kafka_server_brokertopicmetrics_invalidmagicnumberrecords_total{cluster_id="${clusterId}"})`), start: instantStart, end: instantEnd, step: '60s' },
          { id: 'invalidCrc', query: q('invalidCrc', `sum(kafka_server_brokertopicmetrics_invalidmessagecrcrecords_total{cluster_id="${clusterId}"})`), start: instantStart, end: instantEnd, step: '60s' },
          { id: 'invalidOffset', query: q('invalidOffset', `sum(kafka_server_brokertopicmetrics_invalidoffsetorsequencerecords_total{cluster_id="${clusterId}"})`), start: instantStart, end: instantEnd, step: '60s' },
        ] : []),
        // 范围查询：AdminClient 指标（始终查询）
        { id: 'produceRate', query: q('produceRate', `sum(rate(kafka_topic_partition_current_offset{cluster_id="${clusterId}",topic!~"__.*"}[30s]))`), start: start.unix(), end: end.unix(), step },
        { id: 'consumeRate', query: q('consumeRate', `sum(rate(kafka_consumergroup_current_offset{cluster_id="${clusterId}"}[30s]))`), start: start.unix(), end: end.unix(), step },
        { id: 'lagTrend', query: q('lagTrend', `sum(kafka_consumergroup_lag_sum{cluster_id="${clusterId}"})`), start: start.unix(), end: end.unix(), step },
        // 范围查询：JMX 指标（仅 JMX Exporter 可用时才查）
        ...(jmxAvailable ? [
          { id: 'bytesIn', query: q('bytesIn', `sum(rate(kafka_server_brokertopicmetrics_bytesin_total{cluster_id="${clusterId}"}[30s]))`), start: start.unix(), end: end.unix(), step },
          { id: 'bytesOut', query: q('bytesOut', `sum(rate(kafka_server_brokertopicmetrics_bytesout_total{cluster_id="${clusterId}"}[30s]))`), start: start.unix(), end: end.unix(), step },
          { id: 'bytesRejected', query: q('bytesRejected', `sum(rate(kafka_server_brokertopicmetrics_bytesrejected_total{cluster_id="${clusterId}"}[30s]))`), start: start.unix(), end: end.unix(), step },
          { id: 'failedProduce', query: q('failedProduce', `sum(rate(kafka_server_brokertopicmetrics_failedproducerequests_total{cluster_id="${clusterId}"}[30s]))`), start: start.unix(), end: end.unix(), step },
          { id: 'failedFetch', query: q('failedFetch', `sum(rate(kafka_server_brokertopicmetrics_failedfetchrequests_total{cluster_id="${clusterId}"}[30s]))`), start: start.unix(), end: end.unix(), step },
          { id: 'messagesIn', query: q('messagesIn', `sum(rate(kafka_server_brokertopicmetrics_messagesin_total{cluster_id="${clusterId}"}[30s]))`), start: start.unix(), end: end.unix(), step },
        ] : []),
      ]

      // 单次批量请求
      const res = await metricsAPI.batchQuery(queries)
      const r = res.data.results

      // 提取即时值
      setOverviewStats({
        topicPartitionTotal: extractInstantValue(r['topicPartitions']),
        consumerGroupMemberCount: extractInstantValue(r['cgMemberCount']),
        consumerGroupMemberTotal: extractInstantValue(r['cgMemberTotal']),
        isrTotal: extractInstantValue(r['isrTotal']),
        nonPreferredLeaderCount: extractInstantValue(r['nonPreferred']),
        activeBrokerCount: extractInstantValue(r['activeBroker']),
        fencedBrokerCount: extractInstantValue(r['fencedBroker']),
        globalPartitionCount: extractInstantValue(r['globalPartition']),
        globalTopicCount: extractInstantValue(r['globalTopic']),
        preferredReplicaImbalance: extractInstantValue(r['replicaImbalance']),
        offlinePartitionsCount: extractInstantValue(r['offlinePartitions']),
        activeControllerCount: extractInstantValue(r['activeController']),
        offlineLogDirectoryCount: extractInstantValue(r['offlineLogDirCount']),
        logDirectoryOffline: extractInstantValue(r['logDirOffline']),
      })

      // 根据查询范围生成完整时间轴，缺失数据点填 null
      const fullTimes: string[] = []
      let cursor = start
      while (cursor.isBefore(end) || cursor.isSame(end, 'minute')) {
        fullTimes.push(cursor.format('HH:mm'))
        cursor = cursor.add(parseInt(step), 'second')
      }

      // 将 VM 返回的稀疏时间序列对齐到完整时间轴，缺失填 null
      const alignToTimes = (values: Array<[number, string]>): (number | null)[] => {
        const map = new Map(values.map(v => [dayjs.unix(v[0]).format('HH:mm'), parseFloat(v[1]) || 0] as [string, number]))
        return fullTimes.map(t => map.has(t) ? map.get(t)! : null)
      }

      // 从批量查询结果中提取时间序列 values
      const getValues = (key: string): Array<[number, string]> => {
        const result = r[key]?.data?.result
        return result && result.length > 0 ? result[0].values : []
      }

      setProduceRateData({ times: fullTimes, values: alignToTimes(getValues('produceRate')) })
      setConsumeRateData({ times: fullTimes, values: alignToTimes(getValues('consumeRate')) })
      setLagTrendData({ times: fullTimes, values: alignToTimes(getValues('lagTrend')) })
      setBytesInData({ times: fullTimes, values: alignToTimes(getValues('bytesIn')) })
      setBytesOutData({ times: fullTimes, values: alignToTimes(getValues('bytesOut')) })
      setBytesRejectedData({ times: fullTimes, values: alignToTimes(getValues('bytesRejected')) })
      setFailedProduceRateData({ times: fullTimes, values: alignToTimes(getValues('failedProduce')) })
      setFailedFetchRateData({ times: fullTimes, values: alignToTimes(getValues('failedFetch')) })
      setMessagesInRateData({ times: fullTimes, values: alignToTimes(getValues('messagesIn')) })

      setDataCorruptionStats({
        invalidMagicNumber: extractInstantValue(r['invalidMagic']),
        invalidCrc: extractInstantValue(r['invalidCrc']),
        invalidOffset: extractInstantValue(r['invalidOffset']),
      })
    } catch (error) {
      console.error('Failed to load history', error)
    }
  }, [cluster, getTimeRange, jmxAvailable, overrides])

  useEffect(() => {
    if (cluster) {
      loadHistory()
    }
  }, [cluster, quickRange, customRange, timeRange, loadHistory])

  return (
    <>
      {!metrics?.jmx_exporter_available && (
        <Alert message="JMX Exporter 未配置或不可用" description="请在集群配置中设置 JMX Exporter URL 以获取 Broker 指标" type="warning" showIcon style={{ marginBottom: 16 }} />
      )}
      {!metrics?.kafka_exporter_available && (
        <Alert message="Kafka 连接失败" description="无法连接到 Kafka 集群获取消费者组信息" type="error" showIcon style={{ marginBottom: 16 }} />
      )}

      <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 8 }}>
        <PromqlDebugButton onClick={() => setDebugOpen(true)} overrideCount={Object.keys(overrides).length} />
      </div>

      <DashboardGrid
        storageKey="cluster-overview"
        cols={{ lg: 12, md: 12, sm: 6, xs: 4 }}
        rowHeight={50}
        items={[
          // ===== 第一行：集群基础信息（始终显示） =====
          { i: 'broker-count', x: 0, y: 0, w: 3, h: 2, component: <StatCard label="BROKER COUNT" value={metrics?.broker_count ?? null} color="#3b82f6" /> },
          { i: 'topic-count', x: 3, y: 0, w: 3, h: 2, component: <StatCard label="TOPIC COUNT" value={metrics?.topic_count ?? null} color="#10b981" /> },
          { i: 'partition-total', x: 6, y: 0, w: 3, h: 2, component: <StatCard label="PARTITION TOTAL" value={overviewStats.topicPartitionTotal} /> },
          { i: 'cg-count', x: 9, y: 0, w: 3, h: 2, component: <StatCard label="CONSUMER GROUPS" value={metrics?.consumer_groups?.length ?? null} color="#f59e0b" /> },

          // ===== 第二行：消费组 + ISR 信息（AdminClient 指标，始终显示） =====
          { i: 'cg-member', x: 0, y: 2, w: 3, h: 2, component: <StatCard label="CG MEMBERS" value={overviewStats.consumerGroupMemberTotal} /> },
          { i: 'isr-total', x: 3, y: 2, w: 3, h: 2, component: <StatCard label="ISR TOTAL" value={overviewStats.isrTotal} color="#10b981" /> },
          { i: 'non-preferred', x: 6, y: 2, w: 3, h: 2, component: <StatCard label="NON-PREFERRED LEADER" value={overviewStats.nonPreferredLeaderCount} color={overviewStats.nonPreferredLeaderCount != null && overviewStats.nonPreferredLeaderCount > 0 ? '#ef4444' : '#10b981'} /> },
          { i: 'total-lag', x: 9, y: 2, w: 3, h: 2, component: <StatCard label="TOTAL LAG" value={metrics?.consumer_groups?.reduce((sum, g) => sum + g.total_lag, 0) ?? null} color="#ef4444" /> },

          // ===== JMX Stat 卡片（仅 JMX Exporter 可用时显示） =====
          ...(jmxAvailable ? [
          { i: 'active-broker', x: 0, y: 4, w: 3, h: 2, component: <StatCard label="ACTIVE BROKER" value={overviewStats.activeBrokerCount} /> },
          { i: 'fenced-broker', x: 3, y: 4, w: 3, h: 2, component: <StatCard label="UNHEALTHY BROKER" value={overviewStats.fencedBrokerCount} color={overviewStats.fencedBrokerCount === 0 ? '#10b981' : overviewStats.fencedBrokerCount == null ? undefined : '#ef4444'} /> },
          { i: 'replica-imbalance', x: 6, y: 4, w: 3, h: 2, component: <StatCard label="REPLICA IMBALANCE" value={overviewStats.preferredReplicaImbalance} color={overviewStats.preferredReplicaImbalance === 0 ? '#10b981' : overviewStats.preferredReplicaImbalance == null ? undefined : '#ef4444'} /> },
          { i: 'offline-partitions', x: 9, y: 4, w: 3, h: 2, component: <StatCard label="OFFLINE PARTITIONS" value={overviewStats.offlinePartitionsCount} color={overviewStats.offlinePartitionsCount === 0 ? '#10b981' : overviewStats.offlinePartitionsCount == null ? undefined : '#ef4444'} /> },
          { i: 'active-controller', x: 0, y: 6, w: 3, h: 2, component: <StatCard label="ACTIVE CONTROLLER" value={overviewStats.activeControllerCount} color={overviewStats.activeControllerCount === 1 ? '#10b981' : overviewStats.activeControllerCount == null ? undefined : '#ef4444'} /> },
          { i: 'offline-log-dir', x: 3, y: 6, w: 3, h: 2, component: <StatCard label="OFFLINE LOG DIR" value={overviewStats.offlineLogDirectoryCount} color={overviewStats.offlineLogDirectoryCount === 0 ? '#10b981' : overviewStats.offlineLogDirectoryCount == null ? undefined : '#ef4444'} /> },
          { i: 'log-dir-status', x: 6, y: 6, w: 3, h: 2, component: <StatCard label="LOG DIR STATUS" value={overviewStats.logDirectoryOffline === 0 ? '正常' : overviewStats.logDirectoryOffline == null ? null : '异常'} color={overviewStats.logDirectoryOffline === 0 ? '#10b981' : overviewStats.logDirectoryOffline == null ? undefined : '#ef4444'} /> },
          { i: 'invalid-magic', x: 9, y: 6, w: 3, h: 2, component: <StatCard label="INVALID MAGIC" value={dataCorruptionStats.invalidMagicNumber} color={dataCorruptionStats.invalidMagicNumber === 0 ? '#10b981' : dataCorruptionStats.invalidMagicNumber == null ? undefined : '#ef4444'} /> },
          { i: 'invalid-crc', x: 0, y: 8, w: 3, h: 2, component: <StatCard label="INVALID CRC" value={dataCorruptionStats.invalidCrc} color={dataCorruptionStats.invalidCrc === 0 ? '#10b981' : dataCorruptionStats.invalidCrc == null ? undefined : '#ef4444'} /> },
          { i: 'invalid-offset', x: 3, y: 8, w: 3, h: 2, component: <StatCard label="INVALID OFFSET" value={dataCorruptionStats.invalidOffset} color={dataCorruptionStats.invalidOffset === 0 ? '#10b981' : dataCorruptionStats.invalidOffset == null ? undefined : '#ef4444'} /> },
          ] : []),

          // ===== 趋势图 =====
          // AdminClient 趋势图（始终显示）
          { i: 'lag-chart', x: 0, y: 10, w: 12, h: 6, component: <div className="bento-card"><div className="bento-card-inner" style={{ padding: 16 }}><EChartsReact echarts={echarts} key={`lag-${cluster?.cluster_id}-${quickRange}`} option={buildLineChartOption('消费者组总 Lag', lagTrendData, '#ef4444')} style={{ height: 250 }} notMerge={true} /></div></div> },
          { i: 'produce-rate', x: 0, y: 16, w: 6, h: 6, component: <div className="bento-card"><div className="bento-card-inner" style={{ padding: 16 }}><EChartsReact echarts={echarts} key={`pr-${cluster?.cluster_id}-${quickRange}`} option={buildLineChartOption('集群生产速率', produceRateData, '#3b82f6', 'msg/s')} style={{ height: 250 }} notMerge={true} /></div></div> },
          { i: 'consume-rate', x: 6, y: 16, w: 6, h: 6, component: <div className="bento-card"><div className="bento-card-inner" style={{ padding: 16 }}><EChartsReact echarts={echarts} key={`cr-${cluster?.cluster_id}-${quickRange}`} option={buildLineChartOption('集群消费速率', consumeRateData, '#10b981', 'msg/s')} style={{ height: 250 }} notMerge={true} /></div></div> },
          // JMX 趋势图（仅 JMX Exporter 可用时显示）
          ...(jmxAvailable ? [
          { i: 'bytes-in', x: 0, y: 22, w: 6, h: 6, component: <div className="bento-card"><div className="bento-card-inner" style={{ padding: 16 }}><EChartsReact echarts={echarts} key={`bi-${cluster?.cluster_id}-${quickRange}`} option={buildLineChartOption('字节流入速率', bytesInData, '#10b981', 'bytes/s', formatBytesForChart)} style={{ height: 250 }} notMerge={true} /></div></div> },
          { i: 'bytes-out', x: 6, y: 22, w: 6, h: 6, component: <div className="bento-card"><div className="bento-card-inner" style={{ padding: 16 }}><EChartsReact echarts={echarts} key={`bo-${cluster?.cluster_id}-${quickRange}`} option={buildLineChartOption('字节流出速率', bytesOutData, '#f59e0b', 'bytes/s', formatBytesForChart)} style={{ height: 250 }} notMerge={true} /></div></div> },
          { i: 'messages-in', x: 0, y: 28, w: 6, h: 6, component: <div className="bento-card"><div className="bento-card-inner" style={{ padding: 16 }}><EChartsReact echarts={echarts} key={`mi-${cluster?.cluster_id}-${quickRange}`} option={buildLineChartOption('消息流入速率', messagesInRateData, '#8b5cf6', 'msg/s')} style={{ height: 250 }} notMerge={true} /></div></div> },
          { i: 'bytes-rejected', x: 6, y: 28, w: 6, h: 6, component: <div className="bento-card"><div className="bento-card-inner" style={{ padding: 16 }}><EChartsReact echarts={echarts} key={`br-${cluster?.cluster_id}-${quickRange}`} option={buildLineChartOption('拒绝字节速率', bytesRejectedData, '#ef4444', 'bytes/s', formatBytesForChart)} style={{ height: 250 }} notMerge={true} /></div></div> },
          { i: 'failed-produce', x: 0, y: 34, w: 6, h: 6, component: <div className="bento-card"><div className="bento-card-inner" style={{ padding: 16 }}><EChartsReact echarts={echarts} key={`fp-${cluster?.cluster_id}-${quickRange}`} option={buildLineChartOption('生产请求失败率', failedProduceRateData, '#ef4444', '次/秒')} style={{ height: 250 }} notMerge={true} /></div></div> },
          { i: 'failed-fetch', x: 6, y: 34, w: 6, h: 6, component: <div className="bento-card"><div className="bento-card-inner" style={{ padding: 16 }}><EChartsReact echarts={echarts} key={`ff-${cluster?.cluster_id}-${quickRange}`} option={buildLineChartOption('拉取请求失败率', failedFetchRateData, '#f59e0b', '次/秒')} style={{ height: 250 }} notMerge={true} /></div></div> },
          ] : []),
        ]}
      />
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

export default React.memo(ClusterOverview)
