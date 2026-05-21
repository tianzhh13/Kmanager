import React, { useState, useEffect, useCallback } from 'react'
import { Card, Select, Spin, Statistic, Table, Space, Tag } from 'antd'
import ReactECharts from 'echarts-for-react'
import dayjs, { Dayjs } from 'dayjs'
import axios from '../../services/api'
import DashboardGrid from '../../components/DashboardGrid'
import { buildLineChartOption, buildMultiSeriesChartOption, formatBytesForChart } from '../../utils/chartOptions'
import { metricsAPI, BatchQueryItem, extractInstantValue, extractMultiSeries, extractErrorRate } from '../../services/metrics'

interface ClusterOption {
  cluster_id: number
  cluster_name: string
}

interface BrokerMonitorProps {
  cluster: ClusterOption
  timeRange: 'quick' | 'custom'
  quickRange: string
  customRange: [Dayjs, Dayjs] | null
  activeTab: string
}

const BrokerMonitor: React.FC<BrokerMonitorProps> = ({ cluster, timeRange, quickRange, customRange, activeTab }) => {
  const [brokerOverviewData, setBrokerOverviewData] = useState<any[]>([])
  const [brokerOverviewLoading, setBrokerOverviewLoading] = useState(false)
  const [selectedBroker, setSelectedBroker] = useState<string>('all')
  const [brokerList, setBrokerList] = useState<{ id: string; host: string }[]>([])
  const [brokerChartLoading, setBrokerChartLoading] = useState(false)

  const [brokerRequestLatencyData, setBrokerRequestLatencyData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerReplicaLagData, setBrokerReplicaLagData] = useState<{ times: string[]; values: number[] }>({ times: [], values: [] })
  const [brokerBytesInData, setBrokerBytesInData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerBytesOutData, setBrokerBytesOutData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerQueueTimeData, setBrokerQueueTimeData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerLocalTimeData, setBrokerLocalTimeData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerRemoteTimeData, setBrokerRemoteTimeData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerThrottleTimeData, setBrokerThrottleTimeData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerErrorsData, setBrokerErrorsData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerReplicationInData, setBrokerReplicationInData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerReplicationOutData, setBrokerReplicationOutData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerReassignmentInData, setBrokerReassignmentInData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerReassignmentOutData, setBrokerReassignmentOutData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerIsrShrinksData, setBrokerIsrShrinksData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerIsrExpandsData, setBrokerIsrExpandsData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerResponseQueueData, setBrokerResponseQueueData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerHandlerIdleData, setBrokerHandlerIdleData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerNetworkIdleData, setBrokerNetworkIdleData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerDiskReadData, setBrokerDiskReadData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerDiskWriteData, setBrokerDiskWriteData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerIsrUpdatesFailed, setBrokerIsrUpdatesFailed] = useState<number>(0)
  const [brokerControllerEventQueueData, setBrokerControllerEventQueueData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerUncleanLeaderElections, setBrokerUncleanLeaderElections] = useState<number>(0)
  const [brokerDelayedOperationsData, setBrokerDelayedOperationsData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerPurgatorySizeData, setBrokerPurgatorySizeData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerDelayedFetchExpires, setBrokerDelayedFetchExpires] = useState<number>(0)
  const [brokerMinFetchRateData, setBrokerMinFetchRateData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerFailedPartitionsCount, setBrokerFailedPartitionsCount] = useState<number>(0)
  const [brokerDeadThreadCount, setBrokerDeadThreadCount] = useState<number>(0)
  const [brokerLogFlushTimeData, setBrokerLogFlushTimeData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerProcessCpuData, setBrokerProcessCpuData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerProcessResidentMemoryData, setBrokerProcessResidentMemoryData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerProcessVirtualMemoryData, setBrokerProcessVirtualMemoryData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerProcessStartTime, setBrokerProcessStartTime] = useState<number>(0)
  const [brokerProcessMaxFds, setBrokerProcessMaxFds] = useState<number>(0)
  const [brokerProcessOpenFdsData, setBrokerProcessOpenFdsData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerLogCleanerMaxDirtyData, setBrokerLogCleanerMaxDirtyData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerLogCleanerTimeSinceLastRunData, setBrokerLogCleanerTimeSinceLastRunData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerLogCleanerUncleanableBytesData, setBrokerLogCleanerUncleanableBytesData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerLogCleanerUncleanablePartitions, setBrokerLogCleanerUncleanablePartitions] = useState<number>(0)
  const [brokerLogCleanerRecopyData, setBrokerLogCleanerRecopyData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerLogCleanerDeadThreads, setBrokerLogCleanerDeadThreads] = useState<number>(0)
  const [brokerLogCleanerMaxBufferData, setBrokerLogCleanerMaxBufferData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerLogCleanerMaxCleanTimeData, setBrokerLogCleanerMaxCleanTimeData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerLogCleanerMaxCompactionDelayData, setBrokerLogCleanerMaxCompactionDelayData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerJvmGcData, setBrokerJvmGcData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerJvmGcCountData, setBrokerJvmGcCountData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerJvmMemoryPoolData, setBrokerJvmMemoryPoolData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerJvmThreadsData, setBrokerJvmThreadsData] = useState<Record<string, { times: string[]; values: number[] }>>({})
  const [brokerJvmDeadlockedThreads, setBrokerJvmDeadlockedThreads] = useState<number>(0)
  const [brokerJvmBufferPoolData, setBrokerJvmBufferPoolData] = useState<Record<string, { times: string[]; values: number[] }>>({})

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

  const getBrokerLatencyChartOption = (title: string, data: any) => {
    if (!data || (!data.single && Object.keys(data.brokers || {}).length === 0)) {
      return { title: { text: title, left: 'center', textStyle: { fontSize: 14, color: '#999' } }, graphic: { type: 'text', left: 'center', top: 'middle', style: { text: '暂无数据', fill: '#999', fontSize: 14 } }, xAxis: { type: 'category', data: [] }, yAxis: { type: 'value' }, series: [] }
    }
    if (data.single) {
      return buildLineChartOption(title, data.single, '#1890ff', 'ms')
    }
    return buildMultiSeriesChartOption(title, data.brokers, '#1890ff', 'ms')
  }

  const loadBrokerOverview = useCallback(async () => {
    if (!cluster) return
    setBrokerOverviewLoading(true)
    try {
      const res = await axios.get(`/metrics/broker-overview/${cluster.cluster_id}`)
      const data = res.data?.data || []
      setBrokerOverviewData(data)
      setBrokerList(data.map((b: any) => ({ id: String(b.broker_id), host: b.broker_host })))
    } catch (error) {
      console.error('Failed to load broker overview', error)
    } finally {
      setBrokerOverviewLoading(false)
    }
  }, [cluster])

  const loadBrokerChartData = useCallback(async () => {
    if (!cluster) return
    setBrokerChartLoading(true)
    try {
      const { start, end, step } = getTimeRange()
      const clusterId = cluster.cluster_id
      const brokerFilter = selectedBroker === 'all' ? '' : `,broker_id="${selectedBroker}"`
      const instantStart = dayjs().subtract(1, 'minute')
      const instantEnd = dayjs()

      const rangeQuery = (id: string, query: string): BatchQueryItem => ({
        id, query, start: start.unix(), end: end.unix(), step
      })

      const instantQuery = (id: string, query: string): BatchQueryItem => ({
        id, query, start: instantStart.unix(), end: instantEnd.unix(), step: '60s'
      })

      const queries: BatchQueryItem[] = [
        rangeQuery('pro', `kafka_broker_request_latency_ms{cluster_id="${clusterId}",request="Produce"${brokerFilter}}`),
        rangeQuery('fetch', `kafka_broker_request_latency_ms{cluster_id="${clusterId}",request="FetchConsumer"${brokerFilter}}`),
        rangeQuery('follower', `kafka_broker_request_latency_ms{cluster_id="${clusterId}",request="FetchFollower"${brokerFilter}}`),
        rangeQuery('lag', `kafka_broker_replica_max_lag{cluster_id="${clusterId}"${brokerFilter}}`),
        rangeQuery('bytesIn', `rate(kafka_broker_bytes_in_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`),
        rangeQuery('bytesOut', `rate(kafka_broker_bytes_out_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`),
        rangeQuery('queueTime', `kafka_broker_request_queue_time_ms{cluster_id="${clusterId}",request="Produce"${brokerFilter}}`),
        rangeQuery('localTime', `kafka_broker_request_local_time_ms{cluster_id="${clusterId}",request="Produce"${brokerFilter}}`),
        rangeQuery('remoteTime', `kafka_broker_request_remote_time_ms{cluster_id="${clusterId}",request="FetchConsumer"${brokerFilter}}`),
        rangeQuery('throttleTime', `kafka_broker_throttle_time_ms{cluster_id="${clusterId}",request="Produce"${brokerFilter}}`),
        rangeQuery('errors', `sum by (request, error) (rate(kafka_broker_request_errors_total{cluster_id="${clusterId}",error!~"NONE"${brokerFilter}}[30s]))`),
        rangeQuery('replIn', `rate(kafka_broker_replication_bytes_in_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`),
        rangeQuery('replOut', `rate(kafka_broker_replication_bytes_out_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`),
        rangeQuery('reassignIn', `rate(kafka_broker_reassignment_bytes_in_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`),
        rangeQuery('reassignOut', `rate(kafka_broker_reassignment_bytes_out_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`),
        rangeQuery('isrShrinks', `rate(kafka_broker_isr_shrinks_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`),
        rangeQuery('isrExpands', `rate(kafka_broker_isr_expands_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`),
        rangeQuery('responseQueue', `kafka_broker_response_queue_size{cluster_id="${clusterId}"${brokerFilter}}`),
        rangeQuery('handlerIdle', `kafka_broker_request_handler_avg_idle_percent{cluster_id="${clusterId}"${brokerFilter}}`),
        rangeQuery('networkIdle', `kafka_broker_network_processor_avg_idle_percent{cluster_id="${clusterId}"${brokerFilter}}`),
        rangeQuery('diskRead', `rate(kafka_broker_disk_read_bytes{cluster_id="${clusterId}"${brokerFilter}}[30s])`),
        rangeQuery('diskWrite', `rate(kafka_broker_disk_write_bytes{cluster_id="${clusterId}"${brokerFilter}}[30s])`),
        instantQuery('isrFailed', `sum(kafka_broker_isr_updates_failed_total{cluster_id="${clusterId}"${brokerFilter}})`),
        rangeQuery('ctrlEventQueue', `kafka_broker_controller_event_queue_time_ms{cluster_id="${clusterId}"${brokerFilter}}`),
        instantQuery('uncleanLeader', `sum(kafka_broker_unclean_leader_elections_total{cluster_id="${clusterId}"${brokerFilter}})`),
        rangeQuery('delayedOps', `kafka_broker_delayed_operations{cluster_id="${clusterId}"${brokerFilter}}`),
        rangeQuery('purgatory', `kafka_broker_purgatory_size{cluster_id="${clusterId}"${brokerFilter}}`),
        instantQuery('fetchExpires', `sum(kafka_broker_delayed_fetch_expires_total{cluster_id="${clusterId}"${brokerFilter}})`),
        rangeQuery('minFetch', `kafka_broker_min_fetch_rate{cluster_id="${clusterId}"${brokerFilter}}`),
        instantQuery('failedParts', `max(kafka_broker_failed_partitions_count{cluster_id="${clusterId}"${brokerFilter}})`),
        instantQuery('deadThreads', `max(kafka_broker_dead_thread_count{cluster_id="${clusterId}"${brokerFilter}})`),
        rangeQuery('logFlush', `kafka_broker_log_flush_time_ms{cluster_id="${clusterId}"${brokerFilter}}`),
        rangeQuery('cpu', `rate(kafka_broker_process_cpu_seconds_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`),
        rangeQuery('resMem', `kafka_broker_process_resident_memory_bytes{cluster_id="${clusterId}"${brokerFilter}}`),
        rangeQuery('virtMem', `kafka_broker_process_virtual_memory_bytes{cluster_id="${clusterId}"${brokerFilter}}`),
        instantQuery('procStart', `max(kafka_broker_process_start_time_seconds{cluster_id="${clusterId}"${brokerFilter}})`),
        instantQuery('maxFds', `max(kafka_broker_process_max_fds{cluster_id="${clusterId}"${brokerFilter}})`),
        rangeQuery('openFds', `kafka_broker_process_open_fds{cluster_id="${clusterId}"${brokerFilter}}`),
        rangeQuery('cleanerDirty', `kafka_broker_log_cleaner_max_dirty_percent{cluster_id="${clusterId}"${brokerFilter}}`),
        rangeQuery('cleanerSince', `kafka_broker_log_cleaner_time_since_last_run_ms{cluster_id="${clusterId}"${brokerFilter}}`),
        rangeQuery('cleanerBytes', `kafka_broker_log_cleaner_uncleanable_bytes{cluster_id="${clusterId}"${brokerFilter}}`),
        instantQuery('cleanerParts', `max(kafka_broker_log_cleaner_uncleanable_partitions_count{cluster_id="${clusterId}"${brokerFilter}})`),
        rangeQuery('cleanerRecopy', `kafka_broker_log_cleaner_recopy_percent{cluster_id="${clusterId}"${brokerFilter}}`),
        instantQuery('cleanerDead', `max(kafka_broker_log_cleaner_dead_thread_count{cluster_id="${clusterId}"${brokerFilter}})`),
        rangeQuery('cleanerBuf', `kafka_broker_log_cleaner_max_buffer_utilization_percent{cluster_id="${clusterId}"${brokerFilter}}`),
        rangeQuery('cleanerClean', `kafka_broker_log_cleaner_max_clean_time_secs{cluster_id="${clusterId}"${brokerFilter}}`),
        rangeQuery('cleanerCompact', `kafka_broker_log_cleaner_max_compaction_delay_secs{cluster_id="${clusterId}"${brokerFilter}}`),
        rangeQuery('jvmGc', `kafka_broker_jvm_gc_seconds_sum{cluster_id="${clusterId}"${brokerFilter}}`),
        rangeQuery('jvmGcCount', `kafka_broker_jvm_gc_count{cluster_id="${clusterId}"${brokerFilter}}`),
        rangeQuery('jvmMem', `kafka_broker_jvm_memory_pool_used_bytes{cluster_id="${clusterId}"${brokerFilter}}`),
        rangeQuery('jvmThreads', `kafka_broker_jvm_threads_current{cluster_id="${clusterId}"${brokerFilter}}`),
        instantQuery('jvmDead', `max(kafka_broker_jvm_threads_deadlocked{cluster_id="${clusterId}"${brokerFilter}})`),
        rangeQuery('jvmBuf', `kafka_broker_jvm_buffer_pool_used_bytes{cluster_id="${clusterId}"${brokerFilter}}`),
      ]

      const { data: batchRes } = await metricsAPI.batchQuery(queries)
      const r = batchRes.results

      const getMulti = (id: string): any => extractMultiSeries(r[id])
      const getInstant = (id: string) => extractInstantValue(r[id])

      const proRes = getMulti('pro')
      const fetchRes = getMulti('fetch')
      const followerRes = getMulti('follower')
      const lagRes = getMulti('lag')
      const bytesInRes = getMulti('bytesIn')
      const bytesOutRes = getMulti('bytesOut')
      const queueTimeRes = getMulti('queueTime')
      const localTimeRes = getMulti('localTime')
      const remoteTimeRes = getMulti('remoteTime')
      const throttleTimeRes = getMulti('throttleTime')
      const errorsRes = extractErrorRate(r['errors'])
      const replicationInRes = getMulti('replIn')
      const replicationOutRes = getMulti('replOut')
      const reassignmentInRes = getMulti('reassignIn')
      const reassignmentOutRes = getMulti('reassignOut')
      const isrShrinksRes = getMulti('isrShrinks')
      const isrExpandsRes = getMulti('isrExpands')
      const responseQueueRes = getMulti('responseQueue')
      const handlerIdleRes = getMulti('handlerIdle')
      const networkIdleRes = getMulti('networkIdle')
      const diskReadRes = getMulti('diskRead')
      const diskWriteRes = getMulti('diskWrite')
      const isrUpdatesFailedRes = getInstant('isrFailed')
      const controllerEventQueueRes = getMulti('ctrlEventQueue')
      const uncleanLeaderElectionsRes = getInstant('uncleanLeader')
      const delayedOpsRes = getMulti('delayedOps')
      const purgatorySizeRes = getMulti('purgatory')
      const delayedFetchExpiresRes = getInstant('fetchExpires')
      const minFetchRateRes = getMulti('minFetch')
      const failedPartitionsRes = getInstant('failedParts')
      const deadThreadRes = getInstant('deadThreads')
      const logFlushTimeRes = getMulti('logFlush')
      const processCpuRes = getMulti('cpu')
      const processResMemRes = getMulti('resMem')
      const processVirtMemRes = getMulti('virtMem')
      const processStartRes = getInstant('procStart')
      const processMaxFdsRes = getInstant('maxFds')
      const processOpenFdsRes = getMulti('openFds')
      const cleanerMaxDirtyRes = getMulti('cleanerDirty')
      const cleanerTimeSinceRes = getMulti('cleanerSince')
      const cleanerUncleanableBytesRes = getMulti('cleanerBytes')
      const cleanerUncleanablePartRes = getInstant('cleanerParts')
      const cleanerRecopyRes = getMulti('cleanerRecopy')
      const cleanerDeadThreadsRes = getInstant('cleanerDead')
      const cleanerMaxBufRes = getMulti('cleanerBuf')
      const cleanerMaxCleanRes = getMulti('cleanerClean')
      const cleanerMaxCompactRes = getMulti('cleanerCompact')
      const jvmGcSumRes = getMulti('jvmGc')
      const jvmGcCountRes = getMulti('jvmGcCount')
      const jvmMemPoolRes = getMulti('jvmMem')
      const jvmThreadsRes = getMulti('jvmThreads')
      const jvmDeadlockedRes = getInstant('jvmDead')
      const jvmBufPoolRes = getMulti('jvmBuf')

      setBrokerRequestLatencyData({ produce: proRes, fetchConsumer: fetchRes, fetchFollower: followerRes })

      if (selectedBroker === 'all' && lagRes.brokers && Object.keys(lagRes.brokers).length > 0) {
        const allTimes = new Set<string>()
        const lagBrokers = lagRes.brokers as Record<string, { times: string[]; values: number[] }>
        Object.values(lagBrokers).forEach((b: { times: string[]; values: number[] }) => b.times.forEach(t => allTimes.add(t)))
        const times = Array.from(allTimes).sort()
        const values = times.map(t => {
          let maxVal = 0
          Object.values(lagBrokers).forEach((b: { times: string[]; values: number[] }) => {
            const idx = b.times.indexOf(t)
            if (idx >= 0 && b.values[idx] > maxVal) maxVal = b.values[idx]
          })
          return maxVal
        })
        setBrokerReplicaLagData({ times, values })
      } else {
        setBrokerReplicaLagData(lagRes.single || { times: [], values: [] })
      }

      setBrokerBytesInData(bytesInRes)
      setBrokerBytesOutData(bytesOutRes)
      setBrokerQueueTimeData(queueTimeRes)
      setBrokerLocalTimeData(localTimeRes)
      setBrokerRemoteTimeData(remoteTimeRes)
      setBrokerThrottleTimeData(throttleTimeRes)
      setBrokerErrorsData(errorsRes || {})
      setBrokerReplicationInData(replicationInRes)
      setBrokerReplicationOutData(replicationOutRes)
      setBrokerReassignmentInData(reassignmentInRes)
      setBrokerReassignmentOutData(reassignmentOutRes)
      setBrokerIsrShrinksData(isrShrinksRes)
      setBrokerIsrExpandsData(isrExpandsRes)
      setBrokerResponseQueueData(responseQueueRes)
      setBrokerHandlerIdleData(handlerIdleRes)
      setBrokerNetworkIdleData(networkIdleRes)
      setBrokerDiskReadData(diskReadRes)
      setBrokerDiskWriteData(diskWriteRes)
      setBrokerIsrUpdatesFailed(isrUpdatesFailedRes || 0)
      setBrokerControllerEventQueueData(controllerEventQueueRes)
      setBrokerUncleanLeaderElections(uncleanLeaderElectionsRes || 0)
      setBrokerDelayedOperationsData(delayedOpsRes)
      setBrokerPurgatorySizeData(purgatorySizeRes)
      setBrokerDelayedFetchExpires(delayedFetchExpiresRes || 0)
      setBrokerMinFetchRateData(minFetchRateRes)
      setBrokerFailedPartitionsCount(failedPartitionsRes || 0)
      setBrokerDeadThreadCount(deadThreadRes || 0)
      setBrokerLogFlushTimeData(logFlushTimeRes)
      setBrokerProcessCpuData(processCpuRes)
      setBrokerProcessResidentMemoryData(processResMemRes)
      setBrokerProcessVirtualMemoryData(processVirtMemRes)
      setBrokerProcessStartTime(processStartRes || 0)
      setBrokerProcessMaxFds(processMaxFdsRes || 0)
      setBrokerProcessOpenFdsData(processOpenFdsRes)
      setBrokerLogCleanerMaxDirtyData(cleanerMaxDirtyRes)
      setBrokerLogCleanerTimeSinceLastRunData(cleanerTimeSinceRes)
      setBrokerLogCleanerUncleanableBytesData(cleanerUncleanableBytesRes)
      setBrokerLogCleanerUncleanablePartitions(cleanerUncleanablePartRes || 0)
      setBrokerLogCleanerRecopyData(cleanerRecopyRes)
      setBrokerLogCleanerDeadThreads(cleanerDeadThreadsRes || 0)
      setBrokerLogCleanerMaxBufferData(cleanerMaxBufRes)
      setBrokerLogCleanerMaxCleanTimeData(cleanerMaxCleanRes)
      setBrokerLogCleanerMaxCompactionDelayData(cleanerMaxCompactRes)
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
  }, [cluster, selectedBroker, getTimeRange])

  useEffect(() => {
    if (activeTab === 'broker' && cluster) {
      loadBrokerOverview()
      loadBrokerChartData()
    }
  }, [activeTab, cluster, loadBrokerOverview, loadBrokerChartData])

  useEffect(() => {
    if (activeTab === 'broker' && cluster) {
      loadBrokerChartData()
    }
  }, [selectedBroker, quickRange, customRange, timeRange])

  return (
    <Spin spinning={brokerOverviewLoading || brokerChartLoading}>
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
          { i: 'isr-failed', x: 0, y: 0, w: 3, h: 2, component: <Card size="small"><Statistic title="ISR 更新失败" value={brokerIsrUpdatesFailed} valueStyle={{ color: brokerIsrUpdatesFailed === 0 ? '#52c41a' : '#f5222d', fontSize: 18 }} /></Card> },
          { i: 'unclean-leader', x: 3, y: 0, w: 3, h: 2, component: <Card size="small"><Statistic title="Unclean Leader 选举" value={brokerUncleanLeaderElections} valueStyle={{ color: brokerUncleanLeaderElections === 0 ? '#52c41a' : '#f5222d', fontSize: 18 }} /></Card> },
          { i: 'failed-partitions', x: 6, y: 0, w: 3, h: 2, component: <Card size="small"><Statistic title="Follower 失败分区" value={brokerFailedPartitionsCount} valueStyle={{ color: brokerFailedPartitionsCount === 0 ? '#52c41a' : '#f5222d', fontSize: 18 }} /></Card> },
          { i: 'dead-threads', x: 9, y: 0, w: 3, h: 2, component: <Card size="small"><Statistic title="Follower 死线程" value={brokerDeadThreadCount} valueStyle={{ color: brokerDeadThreadCount === 0 ? '#52c41a' : '#f5222d', fontSize: 18 }} /></Card> },
          { i: 'fetch-expires', x: 0, y: 2, w: 3, h: 2, component: <Card size="small"><Statistic title="Fetch 延迟过期" value={brokerDelayedFetchExpires} valueStyle={{ fontSize: 18 }} /></Card> },
          { i: 'process-start', x: 3, y: 2, w: 3, h: 2, component: <Card size="small"><Statistic title="进程启动时间" value={brokerProcessStartTime ? new Date(brokerProcessStartTime * 1000).toLocaleString() : '-'} valueStyle={{ fontSize: 14 }} /></Card> },
          { i: 'max-fds', x: 6, y: 2, w: 3, h: 2, component: <Card size="small"><Statistic title="最大文件描述符" value={brokerProcessMaxFds} valueStyle={{ fontSize: 18 }} /></Card> },
          { i: 'deadlocked', x: 9, y: 2, w: 3, h: 2, component: <Card size="small"><Statistic title="死锁线程数" value={brokerJvmDeadlockedThreads} valueStyle={{ color: brokerJvmDeadlockedThreads === 0 ? '#52c41a' : '#f5222d', fontSize: 18 }} /></Card> },
          { i: 'uncleanable-parts', x: 0, y: 4, w: 3, h: 2, component: <Card size="small"><Statistic title="不可清理分区" value={brokerLogCleanerUncleanablePartitions} valueStyle={{ fontSize: 18 }} /></Card> },
          { i: 'cleaner-dead', x: 3, y: 4, w: 3, h: 2, component: <Card size="small"><Statistic title="Cleaner 死线程" value={brokerLogCleanerDeadThreads} valueStyle={{ color: brokerLogCleanerDeadThreads === 0 ? '#52c41a' : '#f5222d', fontSize: 18 }} /></Card> },
          { i: 'latency-produce', x: 0, y: 6, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`lp-${selectedBroker}-${quickRange}`} option={getBrokerLatencyChartOption('生产请求延迟 P99', brokerRequestLatencyData.produce)} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'latency-fetch', x: 4, y: 6, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`lf-${selectedBroker}-${quickRange}`} option={getBrokerLatencyChartOption('消费请求延迟 P99', brokerRequestLatencyData.fetchConsumer)} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'latency-follower', x: 8, y: 6, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`lfo-${selectedBroker}-${quickRange}`} option={getBrokerLatencyChartOption('副本同步延迟 P99', brokerRequestLatencyData.fetchFollower)} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'replica-lag', x: 0, y: 12, w: 6, h: 6, component: <Card size="small"><ReactECharts key={`rl-${selectedBroker}-${quickRange}`} option={buildLineChartOption('副本同步 Lag', brokerReplicaLagData, '#f5222d', 'Lag')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'errors', x: 6, y: 12, w: 6, h: 6, component: <Card size="small"><ReactECharts key={`err-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('请求错误速率', brokerErrorsData, '#f5222d', 'errors/s')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'queue-time', x: 0, y: 18, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`qt-${selectedBroker}-${quickRange}`} option={getBrokerLatencyChartOption('请求排队延迟 P99', brokerQueueTimeData)} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'local-time', x: 4, y: 18, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`lt-${selectedBroker}-${quickRange}`} option={getBrokerLatencyChartOption('本地处理延迟 P99', brokerLocalTimeData)} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'remote-time', x: 8, y: 18, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`rt-${selectedBroker}-${quickRange}`} option={getBrokerLatencyChartOption('远程等待延迟 P99', brokerRemoteTimeData)} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'throttle-time', x: 0, y: 24, w: 6, h: 6, component: <Card size="small"><ReactECharts key={`tt-${selectedBroker}-${quickRange}`} option={getBrokerLatencyChartOption('限流延迟 P99', brokerThrottleTimeData)} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'bytes-in', x: 6, y: 24, w: 6, h: 6, component: <Card size="small"><ReactECharts key={`bi-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('字节流入速率', brokerBytesInData.brokers || {}, '#52c41a', 'bytes/s', (v) => formatBytesForChart(v))} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'bytes-out', x: 0, y: 30, w: 6, h: 6, component: <Card size="small"><ReactECharts key={`bo-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('字节流出速率', brokerBytesOutData.brokers || {}, '#faad14', 'bytes/s', (v) => formatBytesForChart(v))} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'repl-in', x: 6, y: 30, w: 6, h: 6, component: <Card size="small"><ReactECharts key={`ri-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('副本同步流入', brokerReplicationInData.brokers || {}, '#1890ff', 'B/s', (v) => formatBytesForChart(v))} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'repl-out', x: 0, y: 36, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`ro-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('副本同步流出', brokerReplicationOutData.brokers || {}, '#722ed1', 'B/s', (v) => formatBytesForChart(v))} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'reassign-in', x: 4, y: 36, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`rai-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('分区迁移流入', brokerReassignmentInData.brokers || {}, '#13c2c2', 'B/s', (v) => formatBytesForChart(v))} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'reassign-out', x: 8, y: 36, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`rao-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('分区迁移流出', brokerReassignmentOutData.brokers || {}, '#eb2f96', 'B/s', (v) => formatBytesForChart(v))} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'isr-shrinks', x: 0, y: 42, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`is-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('ISR 收缩速率', brokerIsrShrinksData.brokers || {}, '#f5222d', '次/秒')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'isr-expands', x: 4, y: 42, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`ie-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('ISR 扩展速率', brokerIsrExpandsData.brokers || {}, '#52c41a', '次/秒')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'response-queue', x: 8, y: 42, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`rq-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('响应队列大小', brokerResponseQueueData.brokers || {}, '#1890ff', '个')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'handler-idle', x: 0, y: 48, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`hi-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('Handler 空闲率', brokerHandlerIdleData.brokers || {}, '#52c41a', '%', (v) => (v * 100).toFixed(1) + '%')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'network-idle', x: 4, y: 48, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`ni-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('网络 Processor 空闲率', brokerNetworkIdleData.brokers || {}, '#722ed1', '%', (v) => (v * 100).toFixed(1) + '%')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'disk-read', x: 8, y: 48, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`dr-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('磁盘读取速率', brokerDiskReadData.brokers || {}, '#1890ff', 'B/s', (v) => formatBytesForChart(v))} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'disk-write', x: 0, y: 54, w: 6, h: 6, component: <Card size="small"><ReactECharts key={`dw-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('磁盘写入速率', brokerDiskWriteData.brokers || {}, '#faad14', 'B/s', (v) => formatBytesForChart(v))} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'controller-event', x: 6, y: 54, w: 6, h: 6, component: <Card size="small"><ReactECharts key={`ce-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('Controller 事件排队耗时', brokerControllerEventQueueData.brokers || {}, '#722ed1', 'ms')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'delayed-ops', x: 0, y: 60, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`do-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('延迟操作数', brokerDelayedOperationsData.brokers || {}, '#f5222d', '个')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'min-fetch', x: 4, y: 60, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`mf-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('Follower 最小拉取速率', brokerMinFetchRateData.brokers || {}, '#1890ff', '条/秒')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'log-flush', x: 8, y: 60, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`lf2-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('日志 Flush 耗时 P99', brokerLogFlushTimeData.brokers || {}, '#faad14', 'ms')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'purgatory', x: 0, y: 66, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`pg-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('Purgatory 大小', brokerPurgatorySizeData.brokers || {}, '#faad14', '个')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'cpu-usage', x: 4, y: 66, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`cpu-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('进程 CPU 使用率', brokerProcessCpuData.brokers || {}, '#1890ff', '%', (v) => (v * 100).toFixed(2) + '%')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'resident-mem', x: 8, y: 66, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`rm-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('进程驻留内存', brokerProcessResidentMemoryData.brokers || {}, '#52c41a', 'B', (v) => formatBytesForChart(v))} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'virtual-mem', x: 0, y: 72, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`vm-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('进程虚拟内存', brokerProcessVirtualMemoryData.brokers || {}, '#722ed1', 'B', (v) => formatBytesForChart(v))} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'open-fds', x: 4, y: 72, w: 8, h: 6, component: <Card size="small"><ReactECharts key={`of-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('已用文件描述符', brokerProcessOpenFdsData.brokers || {}, '#faad14', '个')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'max-dirty', x: 0, y: 78, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`md-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('最大脏比例', brokerLogCleanerMaxDirtyData.brokers || {}, '#f5222d', '%')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'time-since', x: 4, y: 78, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`ts-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('上次清理间隔', brokerLogCleanerTimeSinceLastRunData.brokers || {}, '#1890ff', 'ms')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'uncleanable-bytes', x: 8, y: 78, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`ub-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('不可清理字节数', brokerLogCleanerUncleanableBytesData.brokers || {}, '#faad14', 'B', (v) => formatBytesForChart(v))} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'recopy', x: 0, y: 84, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`rc-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('Cleaner 重新复制比例', brokerLogCleanerRecopyData.brokers || {}, '#722ed1', '%')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'max-buffer', x: 4, y: 84, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`mb-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('Cleaner 最大缓冲利用率', brokerLogCleanerMaxBufferData.brokers || {}, '#13c2c2', '%')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'max-clean', x: 8, y: 84, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`mc-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('Cleaner 最大清理时间', brokerLogCleanerMaxCleanTimeData.brokers || {}, '#eb2f96', '秒')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'max-compact', x: 0, y: 90, w: 6, h: 6, component: <Card size="small"><ReactECharts key={`mc2-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('Cleaner 最大压缩延迟', brokerLogCleanerMaxCompactionDelayData.brokers || {}, '#fa8c16', '秒')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'gc-sum', x: 6, y: 90, w: 6, h: 6, component: <Card size="small"><ReactECharts key={`gs-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('GC 耗时', brokerJvmGcData.brokers || {}, '#f5222d', '秒')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'gc-count', x: 0, y: 96, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`gc-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('GC 次数', brokerJvmGcCountData.brokers || {}, '#faad14', '次')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'jvm-mem', x: 4, y: 96, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`jm-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('JVM 内存池已用', brokerJvmMemoryPoolData.brokers || {}, '#1890ff', 'B', (v) => formatBytesForChart(v))} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'jvm-threads', x: 8, y: 96, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`jt-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('JVM 线程数', brokerJvmThreadsData.brokers || {}, '#52c41a', '个')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'jvm-buffer', x: 0, y: 102, w: 6, h: 6, component: <Card size="small"><ReactECharts key={`jb-${selectedBroker}-${quickRange}`} option={buildMultiSeriesChartOption('JVM Buffer 池已用', brokerJvmBufferPoolData.brokers || {}, '#722ed1', 'B', (v) => formatBytesForChart(v))} style={{ height: 250 }} notMerge={true} /></Card> },
        ]}
      />
    </Spin>
  )
}

export default React.memo(BrokerMonitor)
