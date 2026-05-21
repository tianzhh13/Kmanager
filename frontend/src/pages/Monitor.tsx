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

  // 时间范围选择
  const [timeRange, setTimeRange] = useState<'quick' | 'custom'>('quick')
  const [quickRange, setQuickRange] = useState<string>('1h')
  const [customRange, setCustomRange] = useState<[Dayjs, Dayjs] | null>(null)

  // 加载集群列表
  useEffect(() => {
    const loadClusters = async () => {
      try {
        const res = await clusterAPI.list()
        setClusters(res.data || [])
        if (res.data?.length > 0) {
          const clusterIdParam = searchParams.get('clusterId')
          const tabParam = searchParams.get('tab')
          const topicNameParam = searchParams.get('topicName')

          if (clusterIdParam) {
            const targetCluster = res.data.find((c: ClusterOption) => c.cluster_id === parseInt(clusterIdParam))
            setSelectedCluster(targetCluster || res.data[0])
          } else {
            setSelectedCluster(res.data[0])
          }

          if (tabParam && ['overview', 'broker', 'topic'].includes(tabParam)) {
            setActiveTab(tabParam)
          }

          if (topicNameParam && tabParam === 'topic') {
            // TopicMonitor 会根据 activeTab 和 selectedTopic 自行处理
          }
        }
      } catch (error) {
        message.error('加载集群列表失败')
      }
    }
    loadClusters()
  }, [])

  // 加载实时监控数据
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
        />
      ) : null,
    },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card
        title="集群监控"
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
