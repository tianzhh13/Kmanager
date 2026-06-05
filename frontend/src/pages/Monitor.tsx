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

const Monitor: React.FC = () => {
  const [searchParams] = useSearchParams()
  const [loading, setLoading] = useState(false)
  const [clusters, setClusters] = useState<ClusterOption[]>([])
  const [selectedCluster, setSelectedCluster] = useState<ClusterOption | null>(null)
  const [metrics, setMetrics] = useState<ClusterMetricsResponse | null>(null)
  const [activeTab, setActiveTab] = useState('overview')

  const [timeRange, setTimeRange] = useState<'quick' | 'custom'>('quick')
  const [quickRange, setQuickRange] = useState<string>('1h')
  const [customRange, setCustomRange] = useState<[Dayjs, Dayjs] | null>(null)

  useEffect(() => {
    const loadClusters = async () => {
      try {
        const res = await clusterAPI.list()
        setClusters(res.data || [])
        if (res.data?.length > 0) {
          const clusterIdParam = searchParams.get('clusterId')
          const tabParam = searchParams.get('tab')

          if (clusterIdParam) {
            const targetCluster = res.data.find((c: ClusterOption) => c.cluster_id === parseInt(clusterIdParam))
            setSelectedCluster(targetCluster || res.data[0])
          } else {
            setSelectedCluster(res.data[0])
          }

          if (tabParam && ['overview', 'broker', 'topic'].includes(tabParam)) {
            setActiveTab(tabParam)
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
      {/* Header with controls inline */}
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

      {/* Custom tabs */}
      <div style={{ display: 'flex', gap: 4, marginBottom: 20 }}>
        {tabs.map(tab => (
          <button
            key={tab.key}
            className={`bento-action-btn${activeTab === tab.key ? ' bento-pagination-btn--active' : ''}`}
            style={{
              padding: '8px 20px',
              fontSize: 13,
              fontWeight: activeTab === tab.key ? 700 : 500,
              background: activeTab === tab.key ? 'var(--brand)' : 'var(--card)',
              color: activeTab === tab.key ? '#fff' : 'var(--text-2)',
              border: activeTab === tab.key ? 'none' : '1px solid var(--border)',
              borderRadius: 10,
            }}
            onClick={() => setActiveTab(tab.key)}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab content */}
      <Spin spinning={loading}>
        {activeTab === 'overview' && selectedCluster && (
          <ClusterOverview
            cluster={selectedCluster}
            timeRange={timeRange}
            quickRange={quickRange}
            customRange={customRange}
            metrics={metrics}
            jmxAvailable={metrics?.jmx_exporter_available ?? false}
          />
        )}
        {activeTab === 'broker' && selectedCluster && (
          <BrokerMonitor
            cluster={selectedCluster}
            timeRange={timeRange}
            quickRange={quickRange}
            customRange={customRange}
            activeTab={activeTab}
            jmxAvailable={metrics?.jmx_exporter_available ?? false}
          />
        )}
        {activeTab === 'topic' && selectedCluster && (
          <TopicMonitor
            cluster={selectedCluster}
            timeRange={timeRange}
            quickRange={quickRange}
            customRange={customRange}
            metrics={metrics}
            activeTab={activeTab}
            jmxAvailable={metrics?.jmx_exporter_available ?? false}
          />
        )}
      </Spin>
    </div>
  )
}

export default Monitor
