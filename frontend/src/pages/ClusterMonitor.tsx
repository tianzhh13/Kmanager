import { useState, useEffect } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Spin, message } from 'antd'
import { clusterAPI } from '../services/cluster'
import { metricsAPI, ClusterMetricsResponse } from '../services/metrics'
import { Dayjs } from 'dayjs'
import ClusterOverview from './monitor/ClusterOverview'
import BrokerMonitor from './monitor/BrokerMonitor'
import TopicMonitor from './monitor/TopicMonitor'
import MonitorControls from './monitor/MonitorControls'

interface ClusterOption {
  cluster_id: number
  cluster_name: string
}

const ClusterMonitor: React.FC = () => {
	  const [searchParams, setSearchParams] = useSearchParams()
	  const [loading, setLoading] = useState(false)
	  const [clusters, setClusters] = useState<ClusterOption[]>([])
	  const [selectedCluster, setSelectedCluster] = useState<ClusterOption | null>(null)
	  const [metrics, setMetrics] = useState<ClusterMetricsResponse | null>(null)

	  const [timeRange, setTimeRange] = useState<'quick' | 'custom'>('quick')
	  const [quickRange, setQuickRange] = useState<string>('1h')
	  const [customRange, setCustomRange] = useState<[Dayjs, Dayjs] | null>(null)

	  const topicNameParam = searchParams.get('topicName')
	  const consumerGroupParam = searchParams.get('consumerGroup')

	  // tab 唯一数据源：直接从 URL 计算，保证 URL 与展示始终一致
	  const tabParam = searchParams.get('tab')
	  const activeTab = tabParam && ['overview', 'broker', 'topic'].includes(tabParam) ? tabParam : 'overview'

	  const setActiveTab = (tab: string) => {
	    const params = new URLSearchParams(searchParams)
	    params.set('tab', tab)
	    // 切换到非 topic 标签时，清除 topicName/consumerGroup 参数，避免残留干扰后续操作
	    if (tab !== 'topic') {
	      params.delete('topicName')
	      params.delete('consumerGroup')
	    }
	    setSearchParams(params, { replace: true })
	  }

  // 同步选中集群到 URL
  useEffect(() => {
    if (selectedCluster) {
      const params = new URLSearchParams(searchParams)
      params.set('clusterId', String(selectedCluster.cluster_id))
      setSearchParams(params, { replace: true })
    }
  }, [selectedCluster?.cluster_id])

  useEffect(() => {
    const loadClusters = async () => {
      try {
        const res = await clusterAPI.list(1, 100)
        setClusters(res.data || [])
        if (res.data?.length > 0) {
          const clusterIdParam = searchParams.get('clusterId')

          if (clusterIdParam) {
            const targetCluster = res.data.find((c: ClusterOption) => c.cluster_id === parseInt(clusterIdParam))
            setSelectedCluster(targetCluster || res.data[0])
          } else {
            setSelectedCluster(res.data[0])
          }
        }
      } catch (error) {
        message.error('加载集群列表失败')
      }
    }
    loadClusters()
  }, [])

  useEffect(() => {
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
    loadMetrics()
  }, [selectedCluster])

  const tabs = [
    { key: 'overview', label: '集群概览' },
    { key: 'broker', label: 'Broker 监控' },
    { key: 'topic', label: 'Topic 监控' },
  ]

  return (
    <div>
      <div className="page-header">
        <div className="page-header-row">
          <div>
            <h1>监控中心</h1>
            <div className="page-accent-line" />
          </div>
          <MonitorControls
            selectedCluster={selectedCluster?.cluster_id}
            onClusterChange={(clusterId) => {
              const cluster = clusters.find(c => c.cluster_id === clusterId)
              setSelectedCluster(cluster || null)
            }}
            clusters={clusters}
            timeRange={timeRange}
            onTimeRangeChange={setTimeRange}
            quickRange={quickRange}
            onQuickRangeChange={setQuickRange}
            customRange={customRange}
            onCustomRangeChange={setCustomRange}
          />
        </div>
      </div>

      <div style={{ display: 'flex', gap: 4, marginBottom: 20 }}>
        {tabs.map(tab => (
          <button key={tab.key}
            className={`bento-action-btn${activeTab === tab.key ? ' bento-pagination-btn--active' : ''}`}
            style={{
              padding: '8px 20px', fontSize: 13,
              fontWeight: activeTab === tab.key ? 700 : 500,
              background: activeTab === tab.key ? 'var(--brand)' : 'var(--card)',
              color: activeTab === tab.key ? '#fff' : 'var(--text-2)',
              border: activeTab === tab.key ? 'none' : '1px solid var(--border)',
              borderRadius: 10,
            }}
            onClick={() => setActiveTab(tab.key)}>
            {tab.label}
          </button>
        ))}
      </div>

      <Spin spinning={loading}>
        {activeTab === 'overview' && selectedCluster && (
          <ClusterOverview cluster={selectedCluster} timeRange={timeRange} quickRange={quickRange}
            customRange={customRange} metrics={metrics} jmxAvailable={metrics?.jmx_exporter_available ?? false} />
        )}
        {activeTab === 'broker' && selectedCluster && (
          <BrokerMonitor cluster={selectedCluster} timeRange={timeRange} quickRange={quickRange}
            customRange={customRange} activeTab={activeTab} jmxAvailable={metrics?.jmx_exporter_available ?? false} />
        )}
        {activeTab === 'topic' && selectedCluster && (
          <TopicMonitor cluster={selectedCluster} timeRange={timeRange} quickRange={quickRange}
            customRange={customRange} metrics={metrics} activeTab={activeTab}
            jmxAvailable={metrics?.jmx_exporter_available ?? false}
            initialTopic={topicNameParam} initialConsumerGroup={consumerGroupParam} />
        )}
      </Spin>
    </div>
  )
}

export default ClusterMonitor