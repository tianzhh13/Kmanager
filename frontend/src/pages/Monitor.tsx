import { useState, useEffect } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Card, Spin, message, Tabs } from 'antd'
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

  const tabItems = [
    {
      key: 'overview',
      label: '集群概览',
      children: selectedCluster ? (
        <ClusterOverview
          cluster={selectedCluster}
          timeRange={timeRange}
          quickRange={quickRange}
          customRange={customRange}
          metrics={metrics}
          jmxAvailable={metrics?.jmx_exporter_available ?? false}
        />
      ) : null,
    },
    {
      key: 'broker',
      label: 'Broker 监控',
      children: selectedCluster ? (
        <BrokerMonitor
          cluster={selectedCluster}
          timeRange={timeRange}
          quickRange={quickRange}
          customRange={customRange}
          activeTab={activeTab}
          jmxAvailable={metrics?.jmx_exporter_available ?? false}
        />
      ) : null,
    },
    {
      key: 'topic',
      label: 'Topic 监控',
      children: selectedCluster ? (
        <TopicMonitor
          cluster={selectedCluster}
          timeRange={timeRange}
          quickRange={quickRange}
          customRange={customRange}
          metrics={metrics}
          activeTab={activeTab}
          jmxAvailable={metrics?.jmx_exporter_available ?? false}
        />
      ) : null,
    },
  ]

  return (
    <div>
      <div className="page-header">
        <h1>监控中心</h1>
        <div className="page-accent-line" />
      </div>

      <Card
        title={null}
        extra={
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

export default Monitor
