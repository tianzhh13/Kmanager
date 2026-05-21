import React, { useState, useEffect, useCallback } from 'react'
import { Card, Statistic, Alert } from 'antd'
import ReactECharts from 'echarts-for-react'
import dayjs, { Dayjs } from 'dayjs'
import DashboardGrid from '../../components/DashboardGrid'
import { ClusterMetricsResponse, metricsAPI, BatchQueryItem, extractInstantValue, extractRangeValues } from '../../services/metrics'
import { buildLineChartOption, formatBytesForChart } from '../../utils/chartOptions'

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
}

const ClusterOverview: React.FC<ClusterOverviewProps> = ({ cluster, timeRange, quickRange, customRange, metrics }) => {
  const [overviewStats, setOverviewStats] = useState({
    topicPartitionTotal: 0,
    consumerGroupMemberCount: 0,
    consumerGroupMemberTotal: 0,
    isrTotal: 0,
    nonPreferredLeaderCount: 0,
    activeBrokerCount: 0,
    fencedBrokerCount: 0,
    globalPartitionCount: 0,
    globalTopicCount: 0,
    preferredReplicaImbalance: 0,
    offlinePartitionsCount: 0,
    activeControllerCount: 0,
    offlineLogDirectoryCount: 0,
    logDirectoryOffline: 0,
  })

  const [produceRateData, setProduceRateData] = useState<{ times: string[]; values: number[] }>({ times: [], values: [] })
  const [consumeRateData, setConsumeRateData] = useState<{ times: string[]; values: number[] }>({ times: [], values: [] })
  const [lagTrendData, setLagTrendData] = useState<{ times: string[]; values: number[] }>({ times: [], values: [] })
  const [bytesInData, setBytesInData] = useState<{ times: string[]; values: number[] }>({ times: [], values: [] })
  const [bytesOutData, setBytesOutData] = useState<{ times: string[]; values: number[] }>({ times: [], values: [] })
  const [bytesRejectedData, setBytesRejectedData] = useState<{ times: string[]; values: number[] }>({ times: [], values: [] })
  const [messagesInRateData, setMessagesInRateData] = useState<{ times: string[]; values: number[] }>({ times: [], values: [] })
  const [failedProduceRateData, setFailedProduceRateData] = useState<{ times: string[]; values: number[] }>({ times: [], values: [] })
  const [failedFetchRateData, setFailedFetchRateData] = useState<{ times: string[]; values: number[] }>({ times: [], values: [] })
  const [dataCorruptionStats, setDataCorruptionStats] = useState({
    invalidMagicNumber: 0,
    invalidCrc: 0,
    invalidOffset: 0,
  })

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

      // 构建所有查询（14 个即时 + 12 个范围 = 26 个）
      const queries: BatchQueryItem[] = [
        // 即时查询（14 个）
        { id: 'topicPartitions', query: `sum(kafka_topic_partitions{cluster_id="${clusterId}",topic!~"__.*"})`, start: instantStart, end: instantEnd, step: '60s' },
        { id: 'cgMemberCount', query: `count(kafka_consumergroup_members{cluster_id="${clusterId}",consumergroup!~"__.*"})`, start: instantStart, end: instantEnd, step: '60s' },
        { id: 'cgMemberTotal', query: `sum(kafka_consumergroup_members{cluster_id="${clusterId}",consumergroup!~"__.*"})`, start: instantStart, end: instantEnd, step: '60s' },
        { id: 'isrTotal', query: `sum(kafka_topic_partition_in_sync_replica{cluster_id="${clusterId}"})`, start: instantStart, end: instantEnd, step: '60s' },
        { id: 'nonPreferred', query: `count(kafka_topic_partition_leader_is_preferred{cluster_id="${clusterId}"}<1)`, start: instantStart, end: instantEnd, step: '60s' },
        { id: 'activeBroker', query: `max(kafka_broker_active_broker_count{cluster_id="${clusterId}"})`, start: instantStart, end: instantEnd, step: '60s' },
        { id: 'fencedBroker', query: `max(kafka_broker_fenced_broker_count{cluster_id="${clusterId}"})`, start: instantStart, end: instantEnd, step: '60s' },
        { id: 'globalPartition', query: `max(kafka_broker_global_partition_count{cluster_id="${clusterId}"})`, start: instantStart, end: instantEnd, step: '60s' },
        { id: 'globalTopic', query: `max(kafka_broker_global_topic_count{cluster_id="${clusterId}"})`, start: instantStart, end: instantEnd, step: '60s' },
        { id: 'replicaImbalance', query: `max(kafka_broker_preferred_replica_imbalance{cluster_id="${clusterId}"})`, start: instantStart, end: instantEnd, step: '60s' },
        { id: 'offlinePartitions', query: `max(kafka_broker_offline_partitions{cluster_id="${clusterId}"})`, start: instantStart, end: instantEnd, step: '60s' },
        { id: 'activeController', query: `max(kafka_broker_active_controller{cluster_id="${clusterId}"})`, start: instantStart, end: instantEnd, step: '60s' },
        { id: 'offlineLogDirCount', query: `max(kafka_broker_offline_log_directory_count{cluster_id="${clusterId}"})`, start: instantStart, end: instantEnd, step: '60s' },
        { id: 'logDirOffline', query: `max(kafka_broker_log_directory_offline{cluster_id="${clusterId}"})`, start: instantStart, end: instantEnd, step: '60s' },
        // 即时查询：数据损坏（3 个）
        { id: 'invalidMagic', query: `sum(kafka_broker_invalid_magic_number_records_total{cluster_id="${clusterId}"})`, start: instantStart, end: instantEnd, step: '60s' },
        { id: 'invalidCrc', query: `sum(kafka_broker_invalid_message_crc_records_total{cluster_id="${clusterId}"})`, start: instantStart, end: instantEnd, step: '60s' },
        { id: 'invalidOffset', query: `sum(kafka_broker_invalid_offset_or_sequence_records_total{cluster_id="${clusterId}"})`, start: instantStart, end: instantEnd, step: '60s' },
        // 范围查询（7 个）
        { id: 'produceRate', query: `sum(rate(kafka_topic_partition_current_offset{cluster_id="${clusterId}",topic!~"__.*"}[30s]))`, start: start.unix(), end: end.unix(), step },
        { id: 'consumeRate', query: `sum(rate(kafka_consumergroup_current_offset{cluster_id="${clusterId}"}[30s]))`, start: start.unix(), end: end.unix(), step },
        { id: 'lagTrend', query: `sum(kafka_consumergroup_lag_sum{cluster_id="${clusterId}"})`, start: start.unix(), end: end.unix(), step },
        { id: 'bytesIn', query: `sum(rate(kafka_broker_bytes_in_total{cluster_id="${clusterId}"}[30s]))`, start: start.unix(), end: end.unix(), step },
        { id: 'bytesOut', query: `sum(rate(kafka_broker_bytes_out_total{cluster_id="${clusterId}"}[30s]))`, start: start.unix(), end: end.unix(), step },
        { id: 'bytesRejected', query: `sum(rate(kafka_broker_bytes_rejected_total{cluster_id="${clusterId}"}[30s]))`, start: start.unix(), end: end.unix(), step },
        { id: 'failedProduce', query: `sum(rate(kafka_broker_failed_produce_requests_total{cluster_id="${clusterId}"}[30s]))`, start: start.unix(), end: end.unix(), step },
        { id: 'failedFetch', query: `sum(rate(kafka_broker_failed_fetch_requests_total{cluster_id="${clusterId}"}[30s]))`, start: start.unix(), end: end.unix(), step },
        { id: 'messagesIn', query: `sum(rate(kafka_broker_messages_in_total{cluster_id="${clusterId}"}[30s]))`, start: start.unix(), end: end.unix(), step },
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

      // 提取范围值
      const toValues = (values: Array<[number, string]>, times: string[]) => {
        if (times.length === 0) return []
        if (values.length === times.length) return values.map(v => parseFloat(v[1]) || 0)
        return times.map(t => {
          const found = values.find(v => dayjs.unix(v[0]).format('HH:mm') === t)
          return found ? (parseFloat(found[1]) || 0) : 0
        })
      }

      const produceRes = extractRangeValues(r['produceRate'])
      const consumeRes = extractRangeValues(r['consumeRate'])
      const lagRes = extractRangeValues(r['lagTrend'])
      const bytesInRes = extractRangeValues(r['bytesIn'])
      const bytesOutRes = extractRangeValues(r['bytesOut'])
      const bytesRejectedRes = extractRangeValues(r['bytesRejected'])
      const failedProduceRes = extractRangeValues(r['failedProduce'])
      const failedFetchRes = extractRangeValues(r['failedFetch'])
      const messagesInRes = extractRangeValues(r['messagesIn'])

      // 合并时间轴
      const allResults = [produceRes, consumeRes, lagRes, bytesInRes, bytesOutRes, bytesRejectedRes, failedProduceRes, failedFetchRes, messagesInRes]
      let longestResult = allResults.reduce((a, b) => b.length > a.length ? b : a, [] as Array<[number, string]>)
      const times = longestResult.length > 0
        ? longestResult.map(v => dayjs.unix(v[0]).format('HH:mm'))
        : []

      setProduceRateData({ times, values: toValues(produceRes, times) })
      setConsumeRateData({ times, values: toValues(consumeRes, times) })
      setLagTrendData({ times, values: toValues(lagRes, times) })
      setBytesInData({ times, values: toValues(bytesInRes, times) })
      setBytesOutData({ times, values: toValues(bytesOutRes, times) })
      setBytesRejectedData({ times, values: toValues(bytesRejectedRes, times) })
      setFailedProduceRateData({ times, values: toValues(failedProduceRes, times) })
      setFailedFetchRateData({ times, values: toValues(failedFetchRes, times) })
      setMessagesInRateData({ times, values: toValues(messagesInRes, times) })

      setDataCorruptionStats({
        invalidMagicNumber: extractInstantValue(r['invalidMagic']),
        invalidCrc: extractInstantValue(r['invalidCrc']),
        invalidOffset: extractInstantValue(r['invalidOffset']),
      })
    } catch (error) {
      console.error('Failed to load history', error)
    }
  }, [cluster, getTimeRange])

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

      <DashboardGrid
        storageKey="cluster-overview"
        cols={{ lg: 12, md: 12, sm: 6, xs: 4 }}
        rowHeight={50}
        items={[
          { i: 'broker-count', x: 0, y: 0, w: 3, h: 2, component: <Card size="small"><Statistic title="Broker 数量" value={metrics?.broker_count || 0} valueStyle={{ color: '#1890ff', fontSize: 20 }} /></Card> },
          { i: 'topic-count', x: 3, y: 0, w: 3, h: 2, component: <Card size="small"><Statistic title="Topic 数量" value={metrics?.topic_count || 0} valueStyle={{ color: '#52c41a', fontSize: 20 }} /></Card> },
          { i: 'partition-total', x: 6, y: 0, w: 3, h: 2, component: <Card size="small"><Statistic title="分区总数" value={overviewStats.topicPartitionTotal} valueStyle={{ fontSize: 20 }} /></Card> },
          { i: 'cg-count', x: 9, y: 0, w: 3, h: 2, component: <Card size="small"><Statistic title="消费组数量" value={metrics?.consumer_groups?.length || 0} valueStyle={{ color: '#faad14', fontSize: 20 }} /></Card> },

          { i: 'cg-member', x: 0, y: 2, w: 3, h: 2, component: <Card size="small"><Statistic title="消费组成员" value={overviewStats.consumerGroupMemberTotal} valueStyle={{ fontSize: 20 }} /></Card> },
          { i: 'isr-total', x: 3, y: 2, w: 3, h: 2, component: <Card size="small"><Statistic title="ISR 总数" value={overviewStats.isrTotal} valueStyle={{ color: '#52c41a', fontSize: 20 }} /></Card> },
          { i: 'non-preferred', x: 6, y: 2, w: 3, h: 2, component: <Card size="small"><Statistic title="非首选 Leader" value={overviewStats.nonPreferredLeaderCount} valueStyle={{ color: overviewStats.nonPreferredLeaderCount > 0 ? '#f5222d' : '#52c41a', fontSize: 20 }} /></Card> },
          { i: 'total-lag', x: 9, y: 2, w: 3, h: 2, component: <Card size="small"><Statistic title="总消费延迟" value={metrics?.consumer_groups?.reduce((sum, g) => sum + g.total_lag, 0) || 0} valueStyle={{ color: '#f5222d', fontSize: 20 }} /></Card> },

          { i: 'active-broker', x: 0, y: 4, w: 3, h: 2, component: <Card size="small"><Statistic title="活跃 Broker" value={overviewStats.activeBrokerCount} valueStyle={{ fontSize: 20 }} /></Card> },
          { i: 'fenced-broker', x: 3, y: 4, w: 3, h: 2, component: <Card size="small"><Statistic title="不健康 Broker" value={overviewStats.fencedBrokerCount} valueStyle={{ color: overviewStats.fencedBrokerCount === 0 ? '#52c41a' : '#f5222d', fontSize: 20 }} /></Card> },
          { i: 'replica-imbalance', x: 6, y: 4, w: 3, h: 2, component: <Card size="small"><Statistic title="副本不均衡" value={overviewStats.preferredReplicaImbalance} valueStyle={{ color: overviewStats.preferredReplicaImbalance === 0 ? '#52c41a' : '#f5222d', fontSize: 20 }} /></Card> },
          { i: 'offline-partitions', x: 9, y: 4, w: 3, h: 2, component: <Card size="small"><Statistic title="离线分区数" value={overviewStats.offlinePartitionsCount} valueStyle={{ color: overviewStats.offlinePartitionsCount === 0 ? '#52c41a' : '#f5222d', fontSize: 20 }} /></Card> },

          { i: 'active-controller', x: 0, y: 6, w: 3, h: 2, component: <Card size="small"><Statistic title="活跃 Controller" value={overviewStats.activeControllerCount} valueStyle={{ color: overviewStats.activeControllerCount === 1 ? '#52c41a' : '#f5222d', fontSize: 20 }} /></Card> },
          { i: 'offline-log-dir', x: 3, y: 6, w: 3, h: 2, component: <Card size="small"><Statistic title="离线日志目录" value={overviewStats.offlineLogDirectoryCount} valueStyle={{ color: overviewStats.offlineLogDirectoryCount === 0 ? '#52c41a' : '#f5222d', fontSize: 20 }} /></Card> },
          { i: 'log-dir-status', x: 6, y: 6, w: 3, h: 2, component: <Card size="small"><Statistic title="日志目录状态" value={overviewStats.logDirectoryOffline === 0 ? '正常' : '异常'} valueStyle={{ color: overviewStats.logDirectoryOffline === 0 ? '#52c41a' : '#f5222d', fontSize: 16 }} /></Card> },
          { i: 'invalid-magic', x: 9, y: 6, w: 3, h: 2, component: <Card size="small"><Statistic title="无效 Magic" value={dataCorruptionStats.invalidMagicNumber} valueStyle={{ color: dataCorruptionStats.invalidMagicNumber === 0 ? '#52c41a' : '#f5222d', fontSize: 20 }} /></Card> },

          { i: 'invalid-crc', x: 0, y: 8, w: 3, h: 2, component: <Card size="small"><Statistic title="无效 CRC" value={dataCorruptionStats.invalidCrc} valueStyle={{ color: dataCorruptionStats.invalidCrc === 0 ? '#52c41a' : '#f5222d', fontSize: 20 }} /></Card> },
          { i: 'invalid-offset', x: 3, y: 8, w: 3, h: 2, component: <Card size="small"><Statistic title="无效 Offset" value={dataCorruptionStats.invalidOffset} valueStyle={{ color: dataCorruptionStats.invalidOffset === 0 ? '#52c41a' : '#f5222d', fontSize: 20 }} /></Card> },

          { i: 'lag-chart', x: 0, y: 10, w: 12, h: 6, component: <Card size="small"><ReactECharts option={buildLineChartOption('消费者组总 Lag', lagTrendData, '#f5222d')} style={{ height: 250 }} /></Card> },
          { i: 'produce-rate', x: 0, y: 16, w: 6, h: 6, component: <Card size="small"><ReactECharts option={buildLineChartOption('集群生产速率', produceRateData, '#1890ff', 'msg/s')} style={{ height: 250 }} /></Card> },
          { i: 'consume-rate', x: 6, y: 16, w: 6, h: 6, component: <Card size="small"><ReactECharts option={buildLineChartOption('集群消费速率', consumeRateData, '#52c41a', 'msg/s')} style={{ height: 250 }} /></Card> },
          { i: 'bytes-in', x: 0, y: 22, w: 6, h: 6, component: <Card size="small"><ReactECharts option={buildLineChartOption('字节流入速率', bytesInData, '#52c41a', 'bytes/s', formatBytesForChart)} style={{ height: 250 }} /></Card> },
          { i: 'bytes-out', x: 6, y: 22, w: 6, h: 6, component: <Card size="small"><ReactECharts option={buildLineChartOption('字节流出速率', bytesOutData, '#faad14', 'bytes/s', formatBytesForChart)} style={{ height: 250 }} /></Card> },
          { i: 'messages-in', x: 0, y: 28, w: 6, h: 6, component: <Card size="small"><ReactECharts option={buildLineChartOption('消息流入速率', messagesInRateData, '#722ed1', 'msg/s')} style={{ height: 250 }} /></Card> },
          { i: 'bytes-rejected', x: 6, y: 28, w: 6, h: 6, component: <Card size="small"><ReactECharts option={buildLineChartOption('拒绝字节速率', bytesRejectedData, '#f5222d', 'bytes/s', formatBytesForChart)} style={{ height: 250 }} /></Card> },
          { i: 'failed-produce', x: 0, y: 34, w: 6, h: 6, component: <Card size="small"><ReactECharts option={buildLineChartOption('生产请求失败率', failedProduceRateData, '#f5222d', '次/秒')} style={{ height: 250 }} /></Card> },
          { i: 'failed-fetch', x: 6, y: 34, w: 6, h: 6, component: <Card size="small"><ReactECharts option={buildLineChartOption('拉取请求失败率', failedFetchRateData, '#faad14', '次/秒')} style={{ height: 250 }} /></Card> },
        ]}
      />
    </>
  )
}

export default React.memo(ClusterOverview)
