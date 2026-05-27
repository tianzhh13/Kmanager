import React, { useState, useEffect, useCallback } from 'react'
import { Card, Select, Spin, Statistic, Table, Space, Tag, Alert } from 'antd'
import ReactECharts from 'echarts-for-react'
import dayjs, { Dayjs } from 'dayjs'
import axios from '../../services/api'
import DashboardGrid from '../../components/DashboardGrid'
import { usePromqlOverrides, useDefaultPromqls, PromqlDebugger, PromqlDebugButton } from '../../components/PromqlDebugger'
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
  jmxAvailable?: boolean
}

const BrokerMonitor: React.FC<BrokerMonitorProps> = ({ cluster, timeRange, quickRange, customRange, activeTab, jmxAvailable }) => {
  const [brokerOverviewData, setBrokerOverviewData] = useState<any[]>([])
  const [brokerOverviewLoading, setBrokerOverviewLoading] = useState(false)
  const [selectedBroker, setSelectedBroker] = useState<string>('all')
  const [fullTimes, setFullTimes] = useState<string[]>([])
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

  // 包装 buildMultiSeriesChartOption，自动注入 fullTimes
  const buildChart = (title: string, data: any, color: string, unit: string, fmt?: (v: number) => string) => {
    return buildMultiSeriesChartOption(title, data, color, unit, fmt, fullTimes)
  }

  const getBrokerLatencyChartOption = (title: string, data: any) => {
    if (!data || (!data.single && Object.keys(data.brokers || {}).length === 0)) {
      return { title: { text: title, left: 'center', textStyle: { fontSize: 14, color: '#999' } }, graphic: { type: 'text', left: 'center', top: 'middle', style: { text: '暂无数据', fill: '#999', fontSize: 14 } }, xAxis: { type: 'category', data: [] }, yAxis: { type: 'value' }, series: [] }
    }
    if (data.single) {
      return buildLineChartOption(title, data.single, '#1890ff', 'ms')
    }
    return buildChart(title, data.brokers, '#1890ff', 'ms')
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
        id, query, start: start.unix(), end: end.unix(), step
      })

      const instantQuery = (id: string, query: string): BatchQueryItem => ({
        id, query, start: instantStart.unix(), end: instantEnd.unix(), step: '60s'
      })

      const queries: BatchQueryItem[] = [
        rangeQuery('pro', q('pro', `kafka_network_requestmetrics_totaltimems{cluster_id="${clusterId}",request="Produce"${brokerFilter}}`)),
        rangeQuery('fetch', q('fetch', `kafka_network_requestmetrics_totaltimems{cluster_id="${clusterId}",request="FetchConsumer"${brokerFilter}}`)),
        rangeQuery('follower', q('follower', `kafka_network_requestmetrics_totaltimems{cluster_id="${clusterId}",request="FetchFollower"${brokerFilter}}`)),
        rangeQuery('lag', q('lag', `kafka_server_replicafetchermanager_maxlag{cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('bytesIn', q('bytesIn', `rate(kafka_server_brokertopicmetrics_bytesin_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`)),
        rangeQuery('bytesOut', q('bytesOut', `rate(kafka_server_brokertopicmetrics_bytesout_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`)),
        rangeQuery('queueTime', q('queueTime', `kafka_network_requestmetrics_requestqueuetimems{cluster_id="${clusterId}",request="Produce"${brokerFilter}}`)),
        rangeQuery('localTime', q('localTime', `kafka_network_requestmetrics_localtimems{cluster_id="${clusterId}",request="Produce"${brokerFilter}}`)),
        rangeQuery('remoteTime', q('remoteTime', `kafka_network_requestmetrics_remotetimems{cluster_id="${clusterId}",request="FetchConsumer"${brokerFilter}}`)),
        rangeQuery('throttleTime', q('throttleTime', `kafka_network_requestmetrics_throttletimems{cluster_id="${clusterId}",request="Produce"${brokerFilter}}`)),
        rangeQuery('errors', q('errors', `sum by (request, error) (rate(kafka_network_requestmetrics_errors_total{cluster_id="${clusterId}",error!~"NONE"${brokerFilter}}[30s]))`)),
        rangeQuery('replIn', q('replIn', `rate(kafka_server_brokertopicmetrics_replicationbytesin_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`)),
        rangeQuery('replOut', q('replOut', `rate(kafka_server_brokertopicmetrics_replicationbytesout_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`)),
        rangeQuery('reassignIn', q('reassignIn', `rate(kafka_server_brokertopicmetrics_reassignmentbytesin_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`)),
        rangeQuery('reassignOut', q('reassignOut', `rate(kafka_server_brokertopicmetrics_reassignmentbytesout_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`)),
        rangeQuery('isrShrinks', q('isrShrinks', `rate(kafka_server_replicamanager_isrshrinks_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`)),
        rangeQuery('isrExpands', q('isrExpands', `rate(kafka_server_replicamanager_isrexpands_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`)),
        rangeQuery('responseQueue', q('responseQueue', `kafka_network_requestchannel_responsequeuesize{cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('handlerIdle', q('handlerIdle', `kafka_server_kafkarequesthandlerpool_requesthandleravgidle_percent{cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('networkIdle', q('networkIdle', `kafka_network_socketserver_networkprocessoravgidlepercent{cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('diskRead', q('diskRead', `rate(kafka_server_kafkaserver_linux_disk_read_bytes{cluster_id="${clusterId}"${brokerFilter}}[30s])`)),
        rangeQuery('diskWrite', q('diskWrite', `rate(kafka_server_kafkaserver_linux_disk_write_bytes{cluster_id="${clusterId}"${brokerFilter}}[30s])`)),
        instantQuery('isrFailed', q('isrFailed', `sum(kafka_server_replicamanager_failedisrupdates_total{cluster_id="${clusterId}"${brokerFilter}})`)),
        rangeQuery('ctrlEventQueue', q('ctrlEventQueue', `kafka_controller_controllereventmanager_eventqueuetimems{cluster_id="${clusterId}"${brokerFilter}}`)),
        instantQuery('uncleanLeader', q('uncleanLeader', `sum(kafka_controller_controllerstats_uncleanleaderelections_total{cluster_id="${clusterId}"${brokerFilter}})`)),
        rangeQuery('delayedOps', q('delayedOps', `kafka_server_delayedoperationpurgatory_numdelayedoperations{cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('purgatory', q('purgatory', `kafka_server_delayedoperationpurgatory_purgatorysize{cluster_id="${clusterId}"${brokerFilter}}`)),
        instantQuery('fetchExpires', q('fetchExpires', `sum(kafka_server_delayedfetchmetrics_expires_total{cluster_id="${clusterId}"${brokerFilter}})`)),
        rangeQuery('minFetch', q('minFetch', `kafka_server_replicafetchermanager_minfetchrate{cluster_id="${clusterId}"${brokerFilter}}`)),
        instantQuery('failedParts', q('failedParts', `max(kafka_server_replicafetchermanager_failedpartitionscount{cluster_id="${clusterId}"${brokerFilter}})`)),
        instantQuery('deadThreads', q('deadThreads', `max(kafka_server_replicafetchermanager_deadthreadcount{cluster_id="${clusterId}"${brokerFilter}})`)),
        rangeQuery('logFlush', q('logFlush', `kafka_log_logflushstats_logflushrateandtimems{cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('cpu', q('cpu', `rate(process_cpu_seconds_total{cluster_id="${clusterId}"${brokerFilter}}[30s])`)),
        rangeQuery('resMem', q('resMem', `process_resident_memory_bytes{cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('virtMem', q('virtMem', `process_virtual_memory_bytes{cluster_id="${clusterId}"${brokerFilter}}`)),
        instantQuery('procStart', q('procStart', `max(process_start_time_seconds{cluster_id="${clusterId}"${brokerFilter}})`)),
        instantQuery('maxFds', q('maxFds', `max(process_max_fds{cluster_id="${clusterId}"${brokerFilter}})`)),
        rangeQuery('openFds', q('openFds', `process_open_fds{cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('cleanerDirty', q('cleanerDirty', `kafka_log_logcleanermanager_max_dirty_percent{cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('cleanerSince', q('cleanerSince', `kafka_log_logcleanermanager_time_since_last_run_ms{cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('cleanerBytes', q('cleanerBytes', `kafka_log_logcleanermanager_uncleanable_bytes{cluster_id="${clusterId}"${brokerFilter}}`)),
        instantQuery('cleanerParts', q('cleanerParts', `max(kafka_log_logcleanermanager_uncleanable_partitions_count{cluster_id="${clusterId}"${brokerFilter}})`)),
        rangeQuery('cleanerRecopy', q('cleanerRecopy', `kafka_log_logcleaner_cleaner_recopy_percent{cluster_id="${clusterId}"${brokerFilter}}`)),
        instantQuery('cleanerDead', q('cleanerDead', `max(kafka_log_logcleaner_deadthreadcount{cluster_id="${clusterId}"${brokerFilter}})`)),
        rangeQuery('cleanerBuf', q('cleanerBuf', `kafka_log_logcleaner_max_buffer_utilization_percent{cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('cleanerClean', q('cleanerClean', `kafka_log_logcleaner_max_clean_time_secs{cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('cleanerCompact', q('cleanerCompact', `kafka_log_logcleaner_max_compaction_delay_secs{cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('jvmGc', q('jvmGc', `jvm_gc_collection_seconds_sum{cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('jvmGcCount', q('jvmGcCount', `jvm_gc_collection_seconds_count{cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('jvmMem', q('jvmMem', `jvm_memory_pool_collection_used_bytes{cluster_id="${clusterId}"${brokerFilter}}`)),
        rangeQuery('jvmThreads', q('jvmThreads', `jvm_threads_current{cluster_id="${clusterId}"${brokerFilter}}`)),
        instantQuery('jvmDead', q('jvmDead', `max(jvm_threads_deadlocked{cluster_id="${clusterId}"${brokerFilter}})`)),
        rangeQuery('jvmBuf', q('jvmBuf', `jvm_buffer_pool_used_bytes{cluster_id="${clusterId}"${brokerFilter}}`)),
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

      // 根据查询范围生成完整时间轴，供图表补全缺失数据
      const times: string[] = []
      let cursor = getTimeRange().start
      const { end: endTs, step: stepStr } = getTimeRange()
      while (cursor.isBefore(endTs) || cursor.isSame(endTs, 'minute')) {
        times.push(cursor.format('HH:mm'))
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
        <PromqlDebugButton onClick={() => setDebugOpen(true)} overrideCount={Object.keys(overrides).length} />
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

      {!jmxAvailable && (
        <Alert message="JMX Exporter 未配置" description="Broker 级别指标（延迟、流量、JVM 等）依赖 JMX Exporter，请在集群配置中设置 JMX Exporter URL" type="warning" showIcon style={{ marginBottom: 16 }} />
      )}

      {jmxAvailable && (
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
          { i: 'errors', x: 6, y: 12, w: 6, h: 6, component: <Card size="small"><ReactECharts key={`err-${selectedBroker}-${quickRange}`} option={buildChart('请求错误速率', brokerErrorsData, '#f5222d', 'errors/s')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'queue-time', x: 0, y: 18, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`qt-${selectedBroker}-${quickRange}`} option={getBrokerLatencyChartOption('请求排队延迟 P99', brokerQueueTimeData)} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'local-time', x: 4, y: 18, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`lt-${selectedBroker}-${quickRange}`} option={getBrokerLatencyChartOption('本地处理延迟 P99', brokerLocalTimeData)} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'remote-time', x: 8, y: 18, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`rt-${selectedBroker}-${quickRange}`} option={getBrokerLatencyChartOption('远程等待延迟 P99', brokerRemoteTimeData)} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'throttle-time', x: 0, y: 24, w: 6, h: 6, component: <Card size="small"><ReactECharts key={`tt-${selectedBroker}-${quickRange}`} option={getBrokerLatencyChartOption('限流延迟 P99', brokerThrottleTimeData)} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'bytes-in', x: 6, y: 24, w: 6, h: 6, component: <Card size="small"><ReactECharts key={`bi-${selectedBroker}-${quickRange}`} option={buildChart('字节流入速率', brokerBytesInData.brokers || {}, '#52c41a', 'bytes/s', (v) => formatBytesForChart(v))} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'bytes-out', x: 0, y: 30, w: 6, h: 6, component: <Card size="small"><ReactECharts key={`bo-${selectedBroker}-${quickRange}`} option={buildChart('字节流出速率', brokerBytesOutData.brokers || {}, '#faad14', 'bytes/s', (v) => formatBytesForChart(v))} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'repl-in', x: 6, y: 30, w: 6, h: 6, component: <Card size="small"><ReactECharts key={`ri-${selectedBroker}-${quickRange}`} option={buildChart('副本同步流入', brokerReplicationInData.brokers || {}, '#1890ff', 'B/s', (v) => formatBytesForChart(v))} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'repl-out', x: 0, y: 36, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`ro-${selectedBroker}-${quickRange}`} option={buildChart('副本同步流出', brokerReplicationOutData.brokers || {}, '#722ed1', 'B/s', (v) => formatBytesForChart(v))} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'reassign-in', x: 4, y: 36, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`rai-${selectedBroker}-${quickRange}`} option={buildChart('分区迁移流入', brokerReassignmentInData.brokers || {}, '#13c2c2', 'B/s', (v) => formatBytesForChart(v))} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'reassign-out', x: 8, y: 36, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`rao-${selectedBroker}-${quickRange}`} option={buildChart('分区迁移流出', brokerReassignmentOutData.brokers || {}, '#eb2f96', 'B/s', (v) => formatBytesForChart(v))} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'isr-shrinks', x: 0, y: 42, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`is-${selectedBroker}-${quickRange}`} option={buildChart('ISR 收缩速率', brokerIsrShrinksData.brokers || {}, '#f5222d', '次/秒')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'isr-expands', x: 4, y: 42, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`ie-${selectedBroker}-${quickRange}`} option={buildChart('ISR 扩展速率', brokerIsrExpandsData.brokers || {}, '#52c41a', '次/秒')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'response-queue', x: 8, y: 42, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`rq-${selectedBroker}-${quickRange}`} option={buildChart('响应队列大小', brokerResponseQueueData.brokers || {}, '#1890ff', '个')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'handler-idle', x: 0, y: 48, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`hi-${selectedBroker}-${quickRange}`} option={buildChart('Handler 空闲率', brokerHandlerIdleData.brokers || {}, '#52c41a', '%', (v) => (v * 100).toFixed(1) + '%')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'network-idle', x: 4, y: 48, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`ni-${selectedBroker}-${quickRange}`} option={buildChart('网络 Processor 空闲率', brokerNetworkIdleData.brokers || {}, '#722ed1', '%', (v) => (v * 100).toFixed(1) + '%')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'disk-read', x: 8, y: 48, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`dr-${selectedBroker}-${quickRange}`} option={buildChart('磁盘读取速率', brokerDiskReadData.brokers || {}, '#1890ff', 'B/s', (v) => formatBytesForChart(v))} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'disk-write', x: 0, y: 54, w: 6, h: 6, component: <Card size="small"><ReactECharts key={`dw-${selectedBroker}-${quickRange}`} option={buildChart('磁盘写入速率', brokerDiskWriteData.brokers || {}, '#faad14', 'B/s', (v) => formatBytesForChart(v))} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'controller-event', x: 6, y: 54, w: 6, h: 6, component: <Card size="small"><ReactECharts key={`ce-${selectedBroker}-${quickRange}`} option={buildChart('Controller 事件排队耗时', brokerControllerEventQueueData.brokers || {}, '#722ed1', 'ms')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'delayed-ops', x: 0, y: 60, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`do-${selectedBroker}-${quickRange}`} option={buildChart('延迟操作数', brokerDelayedOperationsData.brokers || {}, '#f5222d', '个')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'min-fetch', x: 4, y: 60, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`mf-${selectedBroker}-${quickRange}`} option={buildChart('Follower 最小拉取速率', brokerMinFetchRateData.brokers || {}, '#1890ff', '条/秒')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'log-flush', x: 8, y: 60, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`lf2-${selectedBroker}-${quickRange}`} option={buildChart('日志 Flush 耗时 P99', brokerLogFlushTimeData.brokers || {}, '#faad14', 'ms')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'purgatory', x: 0, y: 66, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`pg-${selectedBroker}-${quickRange}`} option={buildChart('Purgatory 大小', brokerPurgatorySizeData.brokers || {}, '#faad14', '个')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'cpu-usage', x: 4, y: 66, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`cpu-${selectedBroker}-${quickRange}`} option={buildChart('进程 CPU 使用率', brokerProcessCpuData.brokers || {}, '#1890ff', '%', (v) => (v * 100).toFixed(2) + '%')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'resident-mem', x: 8, y: 66, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`rm-${selectedBroker}-${quickRange}`} option={buildChart('进程驻留内存', brokerProcessResidentMemoryData.brokers || {}, '#52c41a', 'B', (v) => formatBytesForChart(v))} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'virtual-mem', x: 0, y: 72, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`vm-${selectedBroker}-${quickRange}`} option={buildChart('进程虚拟内存', brokerProcessVirtualMemoryData.brokers || {}, '#722ed1', 'B', (v) => formatBytesForChart(v))} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'open-fds', x: 4, y: 72, w: 8, h: 6, component: <Card size="small"><ReactECharts key={`of-${selectedBroker}-${quickRange}`} option={buildChart('已用文件描述符', brokerProcessOpenFdsData.brokers || {}, '#faad14', '个')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'max-dirty', x: 0, y: 78, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`md-${selectedBroker}-${quickRange}`} option={buildChart('最大脏比例', brokerLogCleanerMaxDirtyData.brokers || {}, '#f5222d', '%')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'time-since', x: 4, y: 78, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`ts-${selectedBroker}-${quickRange}`} option={buildChart('上次清理间隔', brokerLogCleanerTimeSinceLastRunData.brokers || {}, '#1890ff', 'ms')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'uncleanable-bytes', x: 8, y: 78, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`ub-${selectedBroker}-${quickRange}`} option={buildChart('不可清理字节数', brokerLogCleanerUncleanableBytesData.brokers || {}, '#faad14', 'B', (v) => formatBytesForChart(v))} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'recopy', x: 0, y: 84, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`rc-${selectedBroker}-${quickRange}`} option={buildChart('Cleaner 重新复制比例', brokerLogCleanerRecopyData.brokers || {}, '#722ed1', '%')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'max-buffer', x: 4, y: 84, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`mb-${selectedBroker}-${quickRange}`} option={buildChart('Cleaner 最大缓冲利用率', brokerLogCleanerMaxBufferData.brokers || {}, '#13c2c2', '%')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'max-clean', x: 8, y: 84, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`mc-${selectedBroker}-${quickRange}`} option={buildChart('Cleaner 最大清理时间', brokerLogCleanerMaxCleanTimeData.brokers || {}, '#eb2f96', '秒')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'max-compact', x: 0, y: 90, w: 6, h: 6, component: <Card size="small"><ReactECharts key={`mc2-${selectedBroker}-${quickRange}`} option={buildChart('Cleaner 最大压缩延迟', brokerLogCleanerMaxCompactionDelayData.brokers || {}, '#fa8c16', '秒')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'gc-sum', x: 6, y: 90, w: 6, h: 6, component: <Card size="small"><ReactECharts key={`gs-${selectedBroker}-${quickRange}`} option={buildChart('GC 耗时', brokerJvmGcData.brokers || {}, '#f5222d', '秒')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'gc-count', x: 0, y: 96, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`gc-${selectedBroker}-${quickRange}`} option={buildChart('GC 次数', brokerJvmGcCountData.brokers || {}, '#faad14', '次')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'jvm-mem', x: 4, y: 96, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`jm-${selectedBroker}-${quickRange}`} option={buildChart('JVM 内存池已用', brokerJvmMemoryPoolData.brokers || {}, '#1890ff', 'B', (v) => formatBytesForChart(v))} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'jvm-threads', x: 8, y: 96, w: 4, h: 6, component: <Card size="small"><ReactECharts key={`jt-${selectedBroker}-${quickRange}`} option={buildChart('JVM 线程数', brokerJvmThreadsData.brokers || {}, '#52c41a', '个')} style={{ height: 250 }} notMerge={true} /></Card> },
          { i: 'jvm-buffer', x: 0, y: 102, w: 6, h: 6, component: <Card size="small"><ReactECharts key={`jb-${selectedBroker}-${quickRange}`} option={buildChart('JVM Buffer 池已用', brokerJvmBufferPoolData.brokers || {}, '#722ed1', 'B', (v) => formatBytesForChart(v))} style={{ height: 250 }} notMerge={true} /></Card> },
        ]}
      />
      )}
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
