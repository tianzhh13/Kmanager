import React, { useState, useEffect, useCallback } from 'react'
import { Select, Spin, Alert } from 'antd'
import EChartsReact from 'echarts-for-react/lib/core'
import echarts from '../../utils/echarts'
import dayjs, { Dayjs } from 'dayjs'
import axios from '../../services/api'
import DashboardGrid from '../../components/DashboardGrid'
import { usePromqlOverrides, useDefaultPromqls, PromqlDebugger, PromqlDebugButton } from '../../components/PromqlDebugger'
import {
  createAreaChartOption,
  createMultiLineChartOption,
  buildMultiSeriesChartOption,
  formatBytesForChart,
} from '../../utils/chartOptions'
import type { MultiLineSeries } from '../../utils/chartOptions'
import { metricsAPI, BatchQueryItem, extractInstantValue, extractMultiSeries, extractErrorRate } from '../../services/metrics'
import { StatCard, SectionTitle, LabelTag } from '../../components/bento'

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
  jmxAvailable?: boolean
}

// ─── Helper: aggregate broker-keyed data or extract single broker ───

type BrokerRecord = Record<string, { times: string[]; values: number[] }>

function aggregateBrokerSeries(data: BrokerRecord): { times: string[]; values: number[] } {
  const entries = Object.entries(data).filter(([, d]) => d && d.times && d.times.length > 0)
  if (entries.length === 0) return { times: [], values: [] }
  if (entries.length === 1) return entries[0][1]
  const allTimes = new Set<string>()
  entries.forEach(([, d]) => d.times.forEach(t => allTimes.add(t)))
  const times = Array.from(allTimes).sort()
  const values = times.map(t => {
    let sum = 0
    entries.forEach(([, d]) => {
      const idx = d.times.indexOf(t)
      if (idx >= 0) sum += d.values[idx]
    })
    return sum
  })
  return { times, values }
}

function getBrokerOrAggregate(
  data: any,
  selectedBroker: string,
): { times: string[]; values: number[] } {
  if (!data) return { times: [], values: [] }
  const brokers: BrokerRecord = data.brokers || {}
  const single = data.single
  if (selectedBroker !== 'all') {
    if (single) return single
    if (brokers[selectedBroker]) return brokers[selectedBroker]
    // Try matching by key
    const keys = Object.keys(brokers)
    if (keys.length === 1) return brokers[keys[0]]
  }
  return aggregateBrokerSeries(brokers)
}

// ─── Component ───

const BrokerMonitor: React.FC<BrokerMonitorProps> = ({ cluster, timeRange, quickRange, customRange, activeTab, jmxAvailable }) => {
  const [brokerOverviewData, setBrokerOverviewData] = useState<any[]>([])
  const [brokerOverviewLoading, setBrokerOverviewLoading] = useState(false)
  const [selectedBroker, setSelectedBroker] = useState<string>('all')
  const [fullTimes, setFullTimes] = useState<string[]>([])
  const [brokerList, setBrokerList] = useState<{ id: string; host: string }[]>([])
  const [brokerChartLoading, setBrokerChartLoading] = useState(false)

  const [brokerRequestLatencyData, setBrokerRequestLatencyData] = useState<any>({})
  const [brokerReplicaLagData, setBrokerReplicaLagData] = useState<{ times: string[]; values: number[] }>({ times: [], values: [] })
  const [brokerBytesInData, setBrokerBytesInData] = useState<any>({})
  const [brokerBytesOutData, setBrokerBytesOutData] = useState<any>({})
  const [brokerQueueTimeData, setBrokerQueueTimeData] = useState<any>({})
  const [brokerLocalTimeData, setBrokerLocalTimeData] = useState<any>({})
  const [brokerRemoteTimeData, setBrokerRemoteTimeData] = useState<any>({})
  const [brokerThrottleTimeData, setBrokerThrottleTimeData] = useState<any>({})
  const [brokerErrorsData, setBrokerErrorsData] = useState<BrokerRecord>({})
  const [brokerReplicationInData, setBrokerReplicationInData] = useState<any>({})
  const [brokerReplicationOutData, setBrokerReplicationOutData] = useState<any>({})
  const [brokerReassignmentInData, setBrokerReassignmentInData] = useState<any>({})
  const [brokerReassignmentOutData, setBrokerReassignmentOutData] = useState<any>({})
  const [brokerIsrShrinksData, setBrokerIsrShrinksData] = useState<any>({})
  const [brokerIsrExpandsData, setBrokerIsrExpandsData] = useState<any>({})
  const [brokerResponseQueueData, setBrokerResponseQueueData] = useState<any>({})
  const [brokerHandlerIdleData, setBrokerHandlerIdleData] = useState<any>({})
  const [brokerNetworkIdleData, setBrokerNetworkIdleData] = useState<any>({})
  const [brokerDiskReadData, setBrokerDiskReadData] = useState<any>({})
  const [brokerDiskWriteData, setBrokerDiskWriteData] = useState<any>({})
  const [brokerIsrUpdatesFailed, setBrokerIsrUpdatesFailed] = useState<number>(0)
  const [brokerControllerEventQueueData, setBrokerControllerEventQueueData] = useState<any>({})
  const [brokerUncleanLeaderElections, setBrokerUncleanLeaderElections] = useState<number>(0)
  const [brokerDelayedOperationsData, setBrokerDelayedOperationsData] = useState<any>({})
  const [brokerPurgatorySizeData, setBrokerPurgatorySizeData] = useState<any>({})
  const [brokerDelayedFetchExpires, setBrokerDelayedFetchExpires] = useState<number>(0)
  const [brokerMinFetchRateData, setBrokerMinFetchRateData] = useState<any>({})
  const [brokerFailedPartitionsCount, setBrokerFailedPartitionsCount] = useState<number>(0)
  const [brokerDeadThreadCount, setBrokerDeadThreadCount] = useState<number>(0)
  const [brokerLogFlushTimeData, setBrokerLogFlushTimeData] = useState<any>({})
  const [brokerProcessCpuData, setBrokerProcessCpuData] = useState<any>({})
  const [brokerProcessResidentMemoryData, setBrokerProcessResidentMemoryData] = useState<any>({})
  const [brokerProcessVirtualMemoryData, setBrokerProcessVirtualMemoryData] = useState<any>({})
  const [brokerProcessStartTime, setBrokerProcessStartTime] = useState<number>(0)
  const [brokerProcessMaxFds, setBrokerProcessMaxFds] = useState<number>(0)
  const [brokerProcessOpenFdsData, setBrokerProcessOpenFdsData] = useState<any>({})
  const [brokerLogCleanerMaxDirtyData, setBrokerLogCleanerMaxDirtyData] = useState<any>({})
  const [brokerLogCleanerTimeSinceLastRunData, setBrokerLogCleanerTimeSinceLastRunData] = useState<any>({})
  const [brokerLogCleanerUncleanableBytesData, setBrokerLogCleanerUncleanableBytesData] = useState<any>({})
  const [brokerLogCleanerUncleanablePartitions, setBrokerLogCleanerUncleanablePartitions] = useState<number>(0)
  const [brokerLogCleanerRecopyData, setBrokerLogCleanerRecopyData] = useState<any>({})
  const [brokerLogCleanerDeadThreads, setBrokerLogCleanerDeadThreads] = useState<number>(0)
  const [brokerLogCleanerMaxBufferData, setBrokerLogCleanerMaxBufferData] = useState<any>({})
  const [brokerLogCleanerMaxCleanTimeData, setBrokerLogCleanerMaxCleanTimeData] = useState<any>({})
  const [brokerLogCleanerMaxCompactionDelayData, setBrokerLogCleanerMaxCompactionDelayData] = useState<any>({})
  const [brokerJvmGcData, setBrokerJvmGcData] = useState<any>({})
  const [brokerJvmGcCountData, setBrokerJvmGcCountData] = useState<any>({})
  const [brokerJvmMemoryPoolData, setBrokerJvmMemoryPoolData] = useState<any>({})
  const [brokerJvmThreadsData, setBrokerJvmThreadsData] = useState<any>({})
  const [brokerJvmDeadlockedThreads, setBrokerJvmDeadlockedThreads] = useState<number>(0)
  const [brokerJvmBufferPoolData, setBrokerJvmBufferPoolData] = useState<any>({})
  const [debugOpen, setDebugOpen] = useState(false)
  const { overrides, getQ, setOverride, resetOverride, resetAll } = usePromqlOverrides('broker_monitor')
  const { q, defaultPromqls } = useDefaultPromqls(getQ)

  const queryLabels: Record<string, string> = {
    pro: '生产请求延迟 P99',
    fetch: '消费请求延迟 P99',
    follower: '副本同步延迟 P99',
    lag: '副本同步 Lag',
    bytesIn: '字节流入速率',
    bytesOut: '字节流出速率',
    queueTime: '请求排队延迟 P99',
    localTime: '本地处理延迟 P99',
    remoteTime: '远程等待延迟 P99',
    throttleTime: '限流延迟 P99',
    errors: '请求错误速率',
    replIn: '副本同步流入',
    replOut: '副本同步流出',
    reassignIn: '分区迁移流入',
    reassignOut: '分区迁移流出',
    isrShrinks: 'ISR 收缩速率',
    isrExpands: 'ISR 扩展速率',
    responseQueue: '响应队列大小',
    handlerIdle: 'Handler 空闲率',
    networkIdle: '网络 Processor 空闲率',
    diskRead: '磁盘读取速率',
    diskWrite: '磁盘写入速率',
    isrFailed: 'ISR 更新失败',
    ctrlEventQueue: 'Controller 事件排队耗时',
    uncleanLeader: 'Unclean Leader 选举',
    delayedOps: '延迟操作数',
    purgatory: 'Purgatory 大小',
    fetchExpires: 'Fetch 延迟过期',
    minFetch: 'Follower 最小拉取速率',
    failedParts: 'Follower 失败分区',
    deadThreads: 'Follower 死线程',
    logFlush: '日志 Flush 耗时 P99',
    cpu: '进程 CPU 使用率',
    resMem: '进程驻留内存',
    virtMem: '进程虚拟内存',
    procStart: '进程启动时间',
    maxFds: '最大文件描述符',
    openFds: '已用文件描述符',
    cleanerDirty: '最大脏比例',
    cleanerSince: '上次清理间隔',
    cleanerBytes: '不可清理字节数',
    cleanerParts: '不可清理分区',
    cleanerRecopy: 'Cleaner 重新复制比例',
    cleanerDead: 'Cleaner 死线程',
    cleanerBuf: 'Cleaner 最大缓冲利用率',
    cleanerClean: 'Cleaner 最大清理时间',
    cleanerCompact: 'Cleaner 最大压缩延迟',
    jvmGc: 'GC 耗时',
    jvmGcCount: 'GC 次数',
    jvmMem: 'JVM 内存池已用',
    jvmThreads: 'JVM 线程数',
    jvmDead: '死锁线程数',
    jvmBuf: 'JVM Buffer 池已用',
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

  // Legacy compat: wrap buildMultiSeriesChartOption
  const buildChart = (title: string, data: any, color: string, unit: string, fmt?: (v: number) => string) => {
    return buildMultiSeriesChartOption(title, data, color, unit, fmt, fullTimes)
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
    if (!cluster || !jmxAvailable) return
    setBrokerChartLoading(true)
    try {
      const { start, end, step } = getTimeRange()
      const clusterId = cluster.cluster_id
      const brokerFilter = selectedBroker === 'all' ? '' : `,broker_id="${selectedBroker}"`
      const instantStart = dayjs().subtract(1, 'minute')
      const instantEnd = dayjs()

      const rangeQuery = (id: string, query: string): BatchQueryItem => ({
        id, query, start: start.unix(), end: end.unix(), step,
      })

      const instantQuery = (id: string, query: string): BatchQueryItem => ({
        id, query, start: instantStart.unix(), end: instantEnd.unix(), step: '60s',
      })

      const queries: BatchQueryItem[] = [
        rangeQuery('pro', q('pro', `kafka_network_requestmetrics_totaltimems{app="kmanager",cluster_id="${clusterId}",request="Produce"${brokerFilter}}`)),
        rangeQuery('fetch', q('fetch', `kafka_network_requestmetrics_totaltimems{app="kmanager",cluster_id="${clusterId}",request="FetchConsumer"${brokerFilter}}`)),
        rangeQuery('follower', q('follower', `kafka_network_requestmetrics_totaltimems{app="kmanager",cluster_id="${clusterId}",request="FetchFollower"${brokerFilter}}`)),
        rangeQuery('lag', q('lag', `kafka_server_replicafetchermanager_maxlag{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('bytesIn', q('bytesIn', `rate(kafka_server_brokertopicmetrics_bytesin_total{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}[30s])`)),
        rangeQuery('bytesOut', q('bytesOut', `rate(kafka_server_brokertopicmetrics_bytesout_total{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}[30s])`)),
        rangeQuery('queueTime', q('queueTime', `kafka_network_requestmetrics_requestqueuetimems{app="kmanager",cluster_id="${clusterId}",request="Produce"${brokerFilter}}`)),
        rangeQuery('localTime', q('localTime', `kafka_network_requestmetrics_localtimems{app="kmanager",cluster_id="${clusterId}",request="Produce"${brokerFilter}}`)),
        rangeQuery('remoteTime', q('remoteTime', `kafka_network_requestmetrics_remotetimems{app="kmanager",cluster_id="${clusterId}",request="FetchConsumer"${brokerFilter}}`)),
        rangeQuery('throttleTime', q('throttleTime', `kafka_network_requestmetrics_throttletimems{app="kmanager",cluster_id="${clusterId}",request="Produce"${brokerFilter}}`)),
        rangeQuery('errors', q('errors', `sum by (request, error) (rate(kafka_network_requestmetrics_errors_total{app="kmanager",cluster_id="${clusterId}",error!~"NONE"${brokerFilter}}[30s]))`)),
        rangeQuery('replIn', q('replIn', `rate(kafka_server_brokertopicmetrics_replicationbytesin_total{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}[30s])`)),
        rangeQuery('replOut', q('replOut', `rate(kafka_server_brokertopicmetrics_replicationbytesout_total{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}[30s])`)),
        rangeQuery('reassignIn', q('reassignIn', `rate(kafka_server_brokertopicmetrics_reassignmentbytesin_total{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}[30s])`)),
        rangeQuery('reassignOut', q('reassignOut', `rate(kafka_server_brokertopicmetrics_reassignmentbytesout_total{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}[30s])`)),
        rangeQuery('isrShrinks', q('isrShrinks', `rate(kafka_server_replicamanager_isrshrinks_total{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}[30s])`)),
        rangeQuery('isrExpands', q('isrExpands', `rate(kafka_server_replicamanager_isrexpands_total{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}[30s])`)),
        rangeQuery('responseQueue', q('responseQueue', `kafka_network_requestchannel_responsequeuesize{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('handlerIdle', q('handlerIdle', `kafka_server_kafkarequesthandlerpool_requesthandleravgidle_percent{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('networkIdle', q('networkIdle', `kafka_network_socketserver_networkprocessoravgidlepercent{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('diskRead', q('diskRead', `rate(kafka_server_kafkaserver_linux_disk_read_bytes{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}[30s])`)),
        rangeQuery('diskWrite', q('diskWrite', `rate(kafka_server_kafkaserver_linux_disk_write_bytes{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}[30s])`)),
        instantQuery('isrFailed', q('isrFailed', `sum(kafka_server_replicamanager_failedisrupdates_total{app="kmanager",cluster_id="${clusterId}"${brokerFilter}})`)),
        rangeQuery('ctrlEventQueue', q('ctrlEventQueue', `kafka_controller_controllereventmanager_eventqueuetimems{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}`)),
        instantQuery('uncleanLeader', q('uncleanLeader', `sum(kafka_controller_controllerstats_uncleanleaderelections_total{app="kmanager",cluster_id="${clusterId}"${brokerFilter}})`)),
        rangeQuery('delayedOps', q('delayedOps', `kafka_server_delayedoperationpurgatory_numdelayedoperations{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('purgatory', q('purgatory', `kafka_server_delayedoperationpurgatory_purgatorysize{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}`)),
        instantQuery('fetchExpires', q('fetchExpires', `sum(kafka_server_delayedfetchmetrics_expires_total{app="kmanager",cluster_id="${clusterId}"${brokerFilter}})`)),
        rangeQuery('minFetch', q('minFetch', `kafka_server_replicafetchermanager_minfetchrate{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}`)),
        instantQuery('failedParts', q('failedParts', `max(kafka_server_replicafetchermanager_failedpartitionscount{app="kmanager",cluster_id="${clusterId}"${brokerFilter}})`)),
        instantQuery('deadThreads', q('deadThreads', `max(kafka_server_replicafetchermanager_deadthreadcount{app="kmanager",cluster_id="${clusterId}"${brokerFilter}})`)),
        rangeQuery('logFlush', q('logFlush', `kafka_log_logflushstats_logflushrateandtimems{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('cpu', q('cpu', `rate(process_cpu_seconds_total{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}[30s])`)),
        rangeQuery('resMem', q('resMem', `process_resident_memory_bytes{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('virtMem', q('virtMem', `process_virtual_memory_bytes{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}`)),
        instantQuery('procStart', q('procStart', `max(process_start_time_seconds{app="kmanager",cluster_id="${clusterId}"${brokerFilter}})`)),
        instantQuery('maxFds', q('maxFds', `max(process_max_fds{app="kmanager",cluster_id="${clusterId}"${brokerFilter}})`)),
        rangeQuery('openFds', q('openFds', `process_open_fds{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('cleanerDirty', q('cleanerDirty', `kafka_log_logcleanermanager_max_dirty_percent{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('cleanerSince', q('cleanerSince', `kafka_log_logcleanermanager_time_since_last_run_ms{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('cleanerBytes', q('cleanerBytes', `kafka_log_logcleanermanager_uncleanable_bytes{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}`)),
        instantQuery('cleanerParts', q('cleanerParts', `max(kafka_log_logcleanermanager_uncleanable_partitions_count{app="kmanager",cluster_id="${clusterId}"${brokerFilter}})`)),
        rangeQuery('cleanerRecopy', q('cleanerRecopy', `kafka_log_logcleaner_cleaner_recopy_percent{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}`)),
        instantQuery('cleanerDead', q('cleanerDead', `max(kafka_log_logcleaner_deadthreadcount{app="kmanager",cluster_id="${clusterId}"${brokerFilter}})`)),
        rangeQuery('cleanerBuf', q('cleanerBuf', `kafka_log_logcleaner_max_buffer_utilization_percent{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('cleanerClean', q('cleanerClean', `kafka_log_logcleaner_max_clean_time_secs{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('cleanerCompact', q('cleanerCompact', `kafka_log_logcleaner_max_compaction_delay_secs{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('jvmGc', q('jvmGc', `jvm_gc_collection_seconds_sum{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('jvmGcCount', q('jvmGcCount', `jvm_gc_collection_seconds_count{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('jvmMem', q('jvmMem', `jvm_memory_pool_collection_used_bytes{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('jvmThreads', q('jvmThreads', `jvm_threads_current{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}`)),
        instantQuery('jvmDead', q('jvmDead', `max(jvm_threads_deadlocked{app="kmanager",cluster_id="${clusterId}"${brokerFilter}})`)),
        rangeQuery('jvmBuf', q('jvmBuf', `jvm_buffer_pool_used_bytes{app="kmanager",cluster_id="${clusterId}"${brokerFilter}}`)),
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
        const lagBrokers = lagRes.brokers as BrokerRecord
        Object.values(lagBrokers).forEach((b) => b.times.forEach(t => allTimes.add(t)))
        const times = Array.from(allTimes).sort()
        const values = times.map(t => {
          let maxVal = 0
          Object.values(lagBrokers).forEach((b) => {
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

      // 当时间范围达到 24 小时时，使用包含日期的格式，避免不同日期的相同时间点重叠
      const { start: startTs, end: endTs, step: stepStr } = getTimeRange()
      const durationHours = endTs.diff(startTs, 'hour', true)
      const timeFormat = durationHours >= 24 ? 'MM-DD HH:mm' : 'HH:mm'
      const times: string[] = []
      let cursor = startTs
      while (cursor.isBefore(endTs) || cursor.isSame(endTs, 'minute')) {
        times.push(cursor.format(timeFormat))
        cursor = cursor.add(parseInt(stepStr), 'second')
      }
      setFullTimes(times)
    } catch (error) {
      console.error('Failed to load broker chart data', error)
    } finally {
      setBrokerChartLoading(false)
    }
  }, [cluster, selectedBroker, getTimeRange, jmxAvailable, overrides])

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

  // ─── Chart data builders (merged 43→23) ───

  /** Build a single-series area chart from metric state data */
  const mkArea = (
    data: { times: string[]; values: number[] },
    color: string,
    unit: string,
    fmt?: (v: number) => string,
  ) => createAreaChartOption('', data, color, unit, fmt)

  /** Build a multi-line chart combining multiple metrics into one chart */
  const mkMulti = (seriesList: MultiLineSeries[], yAxisName: string, fmt?: (v: number) => string) =>
    createMultiLineChartOption('', seriesList, yAxisName, fmt)

  /** Extract series from metric state data for multi-line charts */
  const ms = (
    data: any,
    name: string,
  ): MultiLineSeries => {
    const d = getBrokerOrAggregate(data, selectedBroker)
    return { name, times: d.times, values: d.values }
  }

  /** Build cross-broker multi-line chart (per-broker breakdown for single metric) */
  const mkBrokerChart = (
    data: any,
    unit: string,
    fmt?: (v: number) => string,
  ) => {
    const brokers: BrokerRecord = data?.brokers || {}
    if (selectedBroker !== 'all' && data?.single) {
      return createAreaChartOption('', data.single, '#f97316', unit, fmt)
    }
    if (selectedBroker !== 'all') {
      const b = brokers[selectedBroker]
      if (b) return createAreaChartOption('', b, '#f97316', unit, fmt)
      if (Object.keys(brokers).length === 1) return createAreaChartOption('', Object.values(brokers)[0], '#f97316', unit, fmt)
    }
    return buildChart('', brokers, '#f97316', unit, fmt)
  }

  const chartKey = `${selectedBroker}-${quickRange}`

  // 23 merged chart definitions
  const chartItems = jmxAvailable ? [
    // ─── 延迟 ───
    { i: 'bk-replica-lag', x: 0, y: 0, w: 6, h: 6, group: '延迟', component:
      <div className="bento-card"><div className="bento-card-inner">
        <SectionTitle title="副本同步 Lag" />
        <EChartsReact echarts={echarts} key={`rl-${chartKey}`} option={mkArea(brokerReplicaLagData, '#ef4444', 'Lag')} style={{ height: 220 }} notMerge={true} />
      </div></div>,
    },
    { i: 'bk-req-latency', x: 6, y: 0, w: 6, h: 6, group: '延迟', component:
      <div className="bento-card"><div className="bento-card-inner">
        <SectionTitle title="请求延迟 P99" />
        <EChartsReact echarts={echarts} key={`rlat-${chartKey}`} option={mkMulti([
          ms(brokerRequestLatencyData.produce, 'Produce'),
          ms(brokerRequestLatencyData.fetchConsumer, 'Fetch'),
          ms(brokerRequestLatencyData.fetchFollower, 'Follower'),
        ], 'ms')} style={{ height: 220 }} notMerge={true} />
      </div></div>,
    },
    { i: 'bk-proc-latency', x: 0, y: 6, w: 12, h: 6, group: '延迟', component:
      <div className="bento-card"><div className="bento-card-inner">
        <SectionTitle title="处理延迟 P99" />
        <EChartsReact echarts={echarts} key={`plat-${chartKey}`} option={mkMulti([
          ms(brokerQueueTimeData, 'Queue'),
          ms(brokerLocalTimeData, 'Local'),
          ms(brokerRemoteTimeData, 'Remote'),
          ms(brokerThrottleTimeData, 'Throttle'),
        ], 'ms')} style={{ height: 220 }} notMerge={true} />
      </div></div>,
    },
    // ─── 吞吐 ───
    { i: 'bk-bytes-io', x: 0, y: 12, w: 6, h: 6, group: '吞吐', component:
      <div className="bento-card"><div className="bento-card-inner">
        <SectionTitle title="字节流入 / 流出" />
        <EChartsReact echarts={echarts} key={`bio-${chartKey}`} option={mkMulti([
          ms(brokerBytesInData, 'In'),
          ms(brokerBytesOutData, 'Out'),
        ], 'B/s', (v) => formatBytesForChart(v))} style={{ height: 220 }} notMerge={true} />
      </div></div>,
    },
    { i: 'bk-repl-io', x: 6, y: 12, w: 6, h: 6, group: '吞吐', component:
      <div className="bento-card"><div className="bento-card-inner">
        <SectionTitle title="副本同步流入 / 流出" />
        <EChartsReact echarts={echarts} key={`rio-${chartKey}`} option={mkMulti([
          ms(brokerReplicationInData, 'In'),
          ms(brokerReplicationOutData, 'Out'),
        ], 'B/s', (v) => formatBytesForChart(v))} style={{ height: 220 }} notMerge={true} />
      </div></div>,
    },
    { i: 'bk-reassign-io', x: 0, y: 18, w: 6, h: 6, group: '吞吐', component:
      <div className="bento-card"><div className="bento-card-inner">
        <SectionTitle title="分区迁移流入 / 流出" />
        <EChartsReact echarts={echarts} key={`raio-${chartKey}`} option={mkMulti([
          ms(brokerReassignmentInData, 'In'),
          ms(brokerReassignmentOutData, 'Out'),
        ], 'B/s', (v) => formatBytesForChart(v))} style={{ height: 220 }} notMerge={true} />
      </div></div>,
    },
    // ─── 错误与ISR ───
    { i: 'bk-errors', x: 6, y: 18, w: 6, h: 6, group: '错误与ISR', component:
      <div className="bento-card"><div className="bento-card-inner">
        <SectionTitle title="请求错误速率" />
        <EChartsReact echarts={echarts} key={`err-${chartKey}`} option={buildChart('请求错误速率', brokerErrorsData, '#ef4444', 'errors/s')} style={{ height: 220 }} notMerge={true} />
      </div></div>,
    },
    { i: 'bk-isr', x: 0, y: 24, w: 6, h: 6, group: '错误与ISR', component:
      <div className="bento-card"><div className="bento-card-inner">
        <SectionTitle title="ISR 收缩 / 扩展" />
        <EChartsReact echarts={echarts} key={`isr-${chartKey}`} option={mkMulti([
          ms(brokerIsrShrinksData, 'Shrink'),
          ms(brokerIsrExpandsData, 'Expand'),
        ], '次/秒')} style={{ height: 220 }} notMerge={true} />
      </div></div>,
    },
    // ─── 队列与处理 ───
    { i: 'bk-resp-queue', x: 6, y: 24, w: 6, h: 6, group: '队列与处理', component:
      <div className="bento-card"><div className="bento-card-inner">
        <SectionTitle title="响应队列大小" />
        <EChartsReact echarts={echarts} key={`rq-${chartKey}`} option={mkBrokerChart(brokerResponseQueueData, '个')} style={{ height: 220 }} notMerge={true} />
      </div></div>,
    },
    { i: 'bk-idle', x: 0, y: 30, w: 6, h: 6, group: '队列与处理', component:
      <div className="bento-card"><div className="bento-card-inner">
        <SectionTitle title="Handler / 网络 Processor 空闲率" />
        <EChartsReact echarts={echarts} key={`idle-${chartKey}`} option={mkMulti([
          ms(brokerHandlerIdleData, 'Handler'),
          ms(brokerNetworkIdleData, 'Network'),
        ], '%', (v) => (v * 100).toFixed(1) + '%')} style={{ height: 220 }} notMerge={true} />
      </div></div>,
    },
    // ─── 磁盘 ───
    { i: 'bk-disk-io', x: 6, y: 30, w: 6, h: 6, group: '磁盘', component:
      <div className="bento-card"><div className="bento-card-inner">
        <SectionTitle title="磁盘读 / 写速率" />
        <EChartsReact echarts={echarts} key={`disk-${chartKey}`} option={mkMulti([
          ms(brokerDiskReadData, 'Read'),
          ms(brokerDiskWriteData, 'Write'),
        ], 'B/s', (v) => formatBytesForChart(v))} style={{ height: 220 }} notMerge={true} />
      </div></div>,
    },
    // ─── Controller与延迟 ───
    { i: 'bk-ctrl-event', x: 0, y: 36, w: 6, h: 6, group: 'Controller与延迟', component:
      <div className="bento-card"><div className="bento-card-inner">
        <SectionTitle title="Controller 事件排队耗时" />
        <EChartsReact echarts={echarts} key={`ctrl-${chartKey}`} option={mkBrokerChart(brokerControllerEventQueueData, 'ms')} style={{ height: 220 }} notMerge={true} />
      </div></div>,
    },
    { i: 'bk-delay-purg', x: 6, y: 36, w: 6, h: 6, group: 'Controller与延迟', component:
      <div className="bento-card"><div className="bento-card-inner">
        <SectionTitle title="延迟操作 / Purgatory" />
        <EChartsReact echarts={echarts} key={`dpurg-${chartKey}`} option={mkMulti([
          ms(brokerDelayedOperationsData, 'Delayed Ops'),
          ms(brokerPurgatorySizeData, 'Purgatory'),
        ], '个')} style={{ height: 220 }} notMerge={true} />
      </div></div>,
    },
    { i: 'bk-follower-io', x: 0, y: 42, w: 6, h: 6, group: 'Controller与延迟', component:
      <div className="bento-card"><div className="bento-card-inner">
        <SectionTitle title="Follower 最小拉取 / Flush P99" />
        <EChartsReact echarts={echarts} key={`fio-${chartKey}`} option={mkMulti([
          ms(brokerMinFetchRateData, 'Min Fetch'),
          ms(brokerLogFlushTimeData, 'Flush'),
        ], '')} style={{ height: 220 }} notMerge={true} />
      </div></div>,
    },
    // ─── 资源 ───
    { i: 'bk-cpu', x: 6, y: 42, w: 6, h: 6, group: '资源', component:
      <div className="bento-card"><div className="bento-card-inner">
        <SectionTitle title="进程 CPU 使用率" />
        <EChartsReact echarts={echarts} key={`cpu-${chartKey}`} option={mkBrokerChart(brokerProcessCpuData, '%', (v) => (v * 100).toFixed(2) + '%')} style={{ height: 220 }} notMerge={true} />
      </div></div>,
    },
    { i: 'bk-mem', x: 0, y: 48, w: 6, h: 6, group: '资源', component:
      <div className="bento-card"><div className="bento-card-inner">
        <SectionTitle title="进程驻留 / 虚拟内存" />
        <EChartsReact echarts={echarts} key={`mem-${chartKey}`} option={mkMulti([
          ms(brokerProcessResidentMemoryData, 'Resident'),
          ms(brokerProcessVirtualMemoryData, 'Virtual'),
        ], 'B', (v) => formatBytesForChart(v))} style={{ height: 220 }} notMerge={true} />
      </div></div>,
    },
    { i: 'bk-fds', x: 6, y: 48, w: 6, h: 6, group: '资源', component:
      <div className="bento-card"><div className="bento-card-inner">
        <SectionTitle title="已用文件描述符" />
        <EChartsReact echarts={echarts} key={`fds-${chartKey}`} option={mkBrokerChart(brokerProcessOpenFdsData, '个')} style={{ height: 220 }} notMerge={true} />
      </div></div>,
    },
    { i: 'bk-dirty', x: 0, y: 54, w: 6, h: 6, group: '资源', component:
      <div className="bento-card"><div className="bento-card-inner">
        <SectionTitle title="最大脏比例 / 不可清理字节" />
        <EChartsReact echarts={echarts} key={`dirty-${chartKey}`} option={mkMulti([
          ms(brokerLogCleanerMaxDirtyData, 'Dirty %'),
          ms(brokerLogCleanerUncleanableBytesData, 'Uncleanable'),
        ], '')} style={{ height: 220 }} notMerge={true} />
      </div></div>,
    },
    // ─── LogCleaner ───
    { i: 'bk-cleaner-status', x: 6, y: 54, w: 6, h: 6, group: 'LogCleaner', component:
      <div className="bento-card"><div className="bento-card-inner">
        <SectionTitle title="Cleaner 运行状态" />
        <EChartsReact echarts={echarts} key={`cstat-${chartKey}`} option={mkMulti([
          ms(brokerLogCleanerTimeSinceLastRunData, '上次清理间隔'),
          ms(brokerLogCleanerRecopyData, '重复制比例'),
          ms(brokerLogCleanerMaxBufferData, '缓冲利用率'),
        ], '')} style={{ height: 220 }} notMerge={true} />
      </div></div>,
    },
    { i: 'bk-cleaner-latency', x: 0, y: 60, w: 6, h: 6, group: 'LogCleaner', component:
      <div className="bento-card"><div className="bento-card-inner">
        <SectionTitle title="Cleaner 延迟" />
        <EChartsReact echarts={echarts} key={`clat-${chartKey}`} option={mkMulti([
          ms(brokerLogCleanerMaxCleanTimeData, '清理时间'),
          ms(brokerLogCleanerMaxCompactionDelayData, '压缩延迟'),
        ], '秒')} style={{ height: 220 }} notMerge={true} />
      </div></div>,
    },
    // ─── GC与JVM ───
    { i: 'bk-gc', x: 6, y: 60, w: 6, h: 6, group: 'GC与JVM', component:
      <div className="bento-card"><div className="bento-card-inner">
        <SectionTitle title="GC 耗时 / 次数" />
        <EChartsReact echarts={echarts} key={`gc-${chartKey}`} option={mkMulti([
          ms(brokerJvmGcData, 'Time'),
          ms(brokerJvmGcCountData, 'Count'),
        ], '')} style={{ height: 220 }} notMerge={true} />
      </div></div>,
    },
    { i: 'bk-jvm-mem', x: 0, y: 66, w: 6, h: 6, group: 'GC与JVM', component:
      <div className="bento-card"><div className="bento-card-inner">
        <SectionTitle title="JVM 内存池 / Buffer" />
        <EChartsReact echarts={echarts} key={`jvmem-${chartKey}`} option={mkMulti([
          ms(brokerJvmMemoryPoolData, 'Mem Pool'),
          ms(brokerJvmBufferPoolData, 'Buffer Pool'),
        ], 'B', (v) => formatBytesForChart(v))} style={{ height: 220 }} notMerge={true} />
      </div></div>,
    },
    { i: 'bk-jvm-threads', x: 6, y: 66, w: 6, h: 6, group: 'GC与JVM', component:
      <div className="bento-card"><div className="bento-card-inner">
        <SectionTitle title="JVM 线程数" />
        <EChartsReact echarts={echarts} key={`jt-${chartKey}`} option={mkBrokerChart(brokerJvmThreadsData, '个')} style={{ height: 220 }} notMerge={true} />
      </div></div>,
    },
  ] : []

  // ─── Render ───

  return (
    <Spin spinning={brokerOverviewLoading || brokerChartLoading}>
      {/* Broker Selector + PromQL Debug */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 16 }}>
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
        <PromqlDebugButton onClick={() => setDebugOpen(true)} overrideCount={Object.keys(overrides).length} />
      </div>

      {/* Instant Stat Cards (10) */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(5, 1fr)', gap: 12, marginBottom: 20 }}>
        <StatCard label="ISR 更新失败" value={brokerIsrUpdatesFailed} color={brokerIsrUpdatesFailed === 0 ? '#10b981' : '#ef4444'} />
        <StatCard label="Unclean Leader 选举" value={brokerUncleanLeaderElections} color={brokerUncleanLeaderElections === 0 ? '#10b981' : '#ef4444'} />
        <StatCard label="Follower 失败分区" value={brokerFailedPartitionsCount} color={brokerFailedPartitionsCount === 0 ? '#10b981' : '#ef4444'} />
        <StatCard label="Follower 死线程" value={brokerDeadThreadCount} color={brokerDeadThreadCount === 0 ? '#10b981' : '#ef4444'} />
        <StatCard label="Fetch 延迟过期" value={brokerDelayedFetchExpires} />
        <StatCard label="进程启动时间" value={brokerProcessStartTime ? new Date(brokerProcessStartTime * 1000).toLocaleString() : '-'} />
        <StatCard label="最大文件描述符" value={brokerProcessMaxFds} />
        <StatCard label="死锁线程数" value={brokerJvmDeadlockedThreads} color={brokerJvmDeadlockedThreads === 0 ? '#10b981' : '#ef4444'} />
        <StatCard label="不可清理分区" value={brokerLogCleanerUncleanablePartitions} />
        <StatCard label="Cleaner 死线程" value={brokerLogCleanerDeadThreads} color={brokerLogCleanerDeadThreads === 0 ? '#10b981' : '#ef4444'} />
      </div>

      {/* Broker Overview Rows */}
      {brokerOverviewData.length > 0 && (
        <div style={{ marginBottom: 20 }}>
          <SectionTitle title="Broker 总览" />
          <div style={{ overflowX: 'auto' }}>
            <div style={{ display: 'grid', gridTemplateColumns: '80px 1fr 120px 100px 100px 100px', gap: 0, minWidth: 700 }}>
              {/* Header */}
              <div className="bento-grid-header">Broker ID</div>
              <div className="bento-grid-header">Host</div>
              <div className="bento-grid-header">Leader Percent</div>
              <div className="bento-grid-header">Leader</div>
              <div className="bento-grid-header">Replicas</div>
              <div className="bento-grid-header">角色</div>
              {/* Rows */}
              {brokerOverviewData.map((b: any) => (
                <React.Fragment key={b.broker_id}>
                  <div className="bento-grid-cell mono">{b.broker_id}</div>
                  <div className="bento-grid-cell mono">{b.broker_host}</div>
                  <div className="bento-grid-cell mono">{b.leader_percent?.toFixed(1) ?? 0}%</div>
                  <div className="bento-grid-cell mono">{b.leader_count}</div>
                  <div className="bento-grid-cell mono">{b.replica_count}</div>
                  <div className="bento-grid-cell">
                    <LabelTag text={b.is_controller ? 'Controller' : 'Follower'} color={b.is_controller ? 'red' : 'blue'} />
                  </div>
                </React.Fragment>
              ))}
            </div>
          </div>
        </div>
      )}

      {/* JMX Unavailable Alert */}
      {!jmxAvailable && (
        <Alert
          message="JMX Exporter 未配置"
          description="Broker 级别指标（延迟、流量、JVM 等）依赖 JMX Exporter，请在集群配置中设置 JMX Exporter URL"
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
        />
      )}

      {/* Merged Charts (23) */}
      {jmxAvailable && (
        <DashboardGrid
          storageKey="broker-monitor-v2"
          cols={{ lg: 12, md: 12, sm: 6, xs: 4 }}
          rowHeight={45}
          items={chartItems}
        />
      )}

      {/* Promql Debugger */}
      <PromqlDebugger
        open={debugOpen}
        onClose={() => setDebugOpen(false)}
        defaultPromqls={defaultPromqls.current}
        overrides={overrides}
        onSetOverride={setOverride}
        onResetOverride={resetOverride}
        onResetAll={resetAll}
        onApplied={undefined}
        labelMap={queryLabels}
      />
    </Spin>
  )
}

export default React.memo(BrokerMonitor)
