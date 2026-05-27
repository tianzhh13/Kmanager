import { useEffect, useState } from 'react'
import { Card, Row, Col, Statistic, Spin, Tag, Empty, Typography } from 'antd'
import {
  ClusterOutlined,
  FileTextOutlined,
  TeamOutlined,
  WarningOutlined,
  DashboardOutlined,
  SafetyCertificateOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ExclamationCircleOutlined,
  QuestionCircleOutlined,
} from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { clusterAPI, Cluster } from '../services/cluster'
import { metricsAPI, ClusterMetricsResponse } from '../services/metrics'
import api from '../services/api'

const { Text } = Typography

interface DashboardStats {
  totalClusters: number
  activeClusters: number
  totalTopics: number
  totalPartitions: number
  totalConsumerGroups: number
  totalLag: number
  totalUsers: number
  clustersByAuth: Record<string, number>
  clusterHealth: ClusterHealth[]
}

interface ClusterHealth {
  cluster: Cluster
  metrics: ClusterMetricsResponse | null
  loading: boolean
}

const healthIconMap: Record<string, React.ReactNode> = {
  healthy: <CheckCircleOutlined />,
  warning: <ExclamationCircleOutlined />,
  error: <CloseCircleOutlined />,
  unknown: <QuestionCircleOutlined />,
}

const Dashboard: React.FC = () => {
  const [loading, setLoading] = useState(true)
  const [stats, setStats] = useState<DashboardStats>({
    totalClusters: 0,
    activeClusters: 0,
    totalTopics: 0,
    totalPartitions: 0,
    totalConsumerGroups: 0,
    totalLag: 0,
    totalUsers: 0,
    clustersByAuth: {},
    clusterHealth: [],
  })
  const [clusterHealth, setClusterHealth] = useState<ClusterHealth[]>([])
  const navigate = useNavigate()

  useEffect(() => {
    loadDashboard()
  }, [])

  const loadDashboard = async () => {
    setLoading(true)
    try {
      // 1. 获取所有集群
      const clusterRes = await clusterAPI.list(1, 100)
      const clusters: Cluster[] = clusterRes.data || []
      const activeClusters = clusters.filter(c => c.status === 'active')

      // 按认证类型分组
      const authMap: Record<string, number> = {}
      clusters.forEach(c => {
        const type = c.auth_type === 'none' || c.auth_type === 'plaintext' ? 'PLAINTEXT' : c.auth_type.toUpperCase()
        authMap[type] = (authMap[type] || 0) + 1
      })

      // 2. 并发获取每个集群的 metrics
      const healthItems: ClusterHealth[] = clusters.map(c => ({
        cluster: c,
        metrics: null,
        loading: true,
      }))
      setClusterHealth(healthItems)

      // 并发请求 metrics
      const metricsPromises = clusters.map(async (cluster) => {
        try {
          const res = await metricsAPI.getClusterMetrics(cluster.cluster_id)
          return { cluster_id: cluster.cluster_id, data: res.data }
        } catch {
          return { cluster_id: cluster.cluster_id, data: null }
        }
      })
      const metricsResults = await Promise.all(metricsPromises)
      const metricsMap = new Map(metricsResults.map(r => [r.cluster_id, r.data]))

      // 3. 聚合统计
      let totalTopics = 0
      let totalPartitions = 0
      let totalConsumerGroups = 0
      let totalLag = 0

      metricsResults.forEach(result => {
        if (result.data) {
          totalTopics += result.data.topic_count || 0
          totalConsumerGroups += result.data.consumer_groups?.length || 0
          totalLag += result.data.consumer_groups?.reduce((sum: number, g: any) => sum + g.total_lag, 0) || 0
        }
      })

      // 4. 获取用户总数
      let totalUsers = 0
      try {
        const userRes = await api.get('/users', { params: { page: 1, page_size: 1 } })
        totalUsers = userRes.data?.total || 0
      } catch { /* 非超管无权限，保持 0 */ }

      setStats({
        totalClusters: clusters.length,
        activeClusters: activeClusters.length,
        totalTopics,
        totalPartitions,
        totalConsumerGroups,
        totalLag,
        totalUsers,
        clustersByAuth: authMap,
        clusterHealth: [],
      })

      // 更新 health items
      setClusterHealth(clusters.map(c => ({
        cluster: c,
        metrics: metricsMap.get(c.cluster_id) || null,
        loading: false,
      })))
    } catch (error) {
      console.error('Failed to load dashboard:', error)
    } finally {
      setLoading(false)
    }
  }

  const getClusterHealthStatus = (h: ClusterHealth) => {
    const m = h.metrics
    if (!m) return { status: 'unknown', label: '未知', color: 'default' }
    if (!m.kafka_exporter_available) return { status: 'error', label: '连接失败', color: 'error' }
    
    const urp = m.broker_metrics?.under_replicated_partitions || 0
    const offline = m.broker_metrics?.offline_partitions_count || 0
    const totalLag = m.consumer_groups?.reduce((sum: number, g: any) => sum + g.total_lag, 0) || 0

    if (offline > 0) return { status: 'error', label: '有离线分区', color: 'error' }
    if (urp > 0) return { status: 'warning', label: '副本不足', color: 'warning' }
    if (totalLag > 100000) return { status: 'warning', label: '延迟较高', color: 'warning' }
    return { status: 'healthy', label: '正常', color: 'success' }
  }

  if (loading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: 400 }}>
        <Spin size="large" />
      </div>
    )
  }

  return (
    <div>
      {/* 页面标题 */}
      <div className="page-header">
        <h1>仪表盘</h1>
        <div className="page-accent-line" />
      </div>

      {/* === 第一行：全局统计卡片 === */}
      <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
        <Col xs={12} sm={8} md={6}>
          <Card size="small" className="stat-card" onClick={() => navigate('/clusters')} style={{ cursor: 'pointer' }}>
            <Statistic
              title="集群总数"
              value={stats.totalClusters}
              prefix={<ClusterOutlined />}
              valueStyle={{ color: 'var(--text-heading)', fontWeight: 700, fontSize: 26 }}
            />
            <div style={{ marginTop: 8 }}>
              <Tag color="success">{stats.activeClusters} 活跃</Tag>
              {stats.totalClusters - stats.activeClusters > 0 && (
                <Tag color="error">{stats.totalClusters - stats.activeClusters} 禁用</Tag>
              )}
            </div>
          </Card>
        </Col>
        <Col xs={12} sm={8} md={6}>
          <Card size="small" className="stat-card" onClick={() => navigate('/topics')} style={{ cursor: 'pointer' }}>
            <Statistic
              title="Topic 总数"
              value={stats.totalTopics}
              prefix={<FileTextOutlined />}
              valueStyle={{ color: 'var(--text-heading)', fontWeight: 700, fontSize: 26 }}
            />
            <div style={{ marginTop: 8 }}>
              <Text type="secondary" style={{ fontSize: 12 }}>{stats.totalConsumerGroups} 个消费组</Text>
            </div>
          </Card>
        </Col>
        <Col xs={12} sm={8} md={6}>
          <Card size="small" className="stat-card">
            <Statistic
              title="总消费延迟"
              value={stats.totalLag}
              prefix={<WarningOutlined />}
              valueStyle={{ color: stats.totalLag > 100000 ? 'var(--color-error)' : 'var(--text-heading)', fontWeight: 700, fontSize: 26 }}
            />
            <div style={{ marginTop: 8 }}>
              <Text type="secondary" style={{ fontSize: 12 }}>{stats.totalConsumerGroups} 个消费组</Text>
            </div>
          </Card>
        </Col>
        <Col xs={12} sm={8} md={6}>
          <Card size="small" className="stat-card">
            <Statistic
              title="用户总数"
              value={stats.totalUsers}
              prefix={<TeamOutlined />}
              valueStyle={{ color: 'var(--text-heading)', fontWeight: 700, fontSize: 26 }}
            />
            <div style={{ marginTop: 8 }}>
              <Text type="secondary" style={{ fontSize: 12 }}>仅超级管理员可见</Text>
            </div>
          </Card>
        </Col>
      </Row>

      {/* === 第二行：集群健康状态列表 === */}
      <Card
        title={
          <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <SafetyCertificateOutlined style={{ color: 'var(--brand-primary)' }} />
            集群健康状态
          </span>
        }
        size="small"
      >
        {clusterHealth.length === 0 ? (
          <Empty description="暂无集群数据" />
        ) : (
          <Row gutter={[12, 12]}>
            {clusterHealth.map(h => {
              const health = getClusterHealthStatus(h)
              const brokerCount = h.metrics?.broker_count || 0
              const topicCount = h.metrics?.topic_count || 0
              const cgCount = h.metrics?.consumer_groups?.length || 0
              const jmxAvailable = h.metrics?.jmx_exporter_available ?? false

              return (
                <Col xs={24} sm={12} md={8} lg={6} key={h.cluster.cluster_id}>
                  <div
                    className="cluster-health-card"
                    onClick={() => navigate(`/monitor?clusterId=${h.cluster.cluster_id}`)}
                    style={{
                      padding: 16,
                      borderRadius: 'var(--radius-lg)',
                      border: '1px solid var(--content-card-border)',
                      background: 'var(--content-card-bg)',
                      cursor: 'pointer',
                      transition: 'all var(--transition-fast)',
                      position: 'relative',
                      overflow: 'hidden',
                    }}
                  >
                    {/* 左侧状态色条 */}
                    <div className={`health-status-bar health-status-bar${health.status}`} />
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 12, paddingLeft: 4 }}>
                      <div>
                        <div style={{ fontWeight: 600, fontSize: 14, color: 'var(--text-heading)', marginBottom: 2 }}>
                          {h.cluster.cluster_name}
                        </div>
                        <div className="text-mono" style={{ fontSize: 11, color: 'var(--text-tertiary)', maxWidth: 180, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                          {h.cluster.bootstrap_servers}
                        </div>
                      </div>
                      <Tag color={health.color} style={{ margin: 0 }}>
                        {healthIconMap[health.status]} {health.label}
                      </Tag>
                    </div>
                    <div style={{ display: 'flex', gap: 16, fontSize: 12, color: 'var(--text-secondary)', paddingLeft: 4 }}>
                      <span><strong style={{ color: 'var(--text-heading)' }}>{brokerCount}</strong> Broker</span>
                      <span><strong style={{ color: 'var(--text-heading)' }}>{topicCount}</strong> Topic</span>
                      <span><strong style={{ color: 'var(--text-heading)' }}>{cgCount}</strong> 消费组</span>
                    </div>
                    <div style={{ marginTop: 8, display: 'flex', gap: 4, paddingLeft: 4 }}>
                      <Tag color={h.cluster.auth_type === 'none' || h.cluster.auth_type === 'plaintext' ? 'success' : 'processing'} style={{ fontSize: 11, margin: 0 }}>
                        {h.cluster.auth_type?.toUpperCase() || 'NONE'}
                      </Tag>
                      {jmxAvailable && <Tag color="default" style={{ fontSize: 11, margin: 0 }}>JMX</Tag>}
                    </div>
                  </div>
                </Col>
              )
            })}
          </Row>
        )}
      </Card>

      {/* === 第三行：快速入口 === */}
      <Row gutter={[16, 16]} style={{ marginTop: 24 }}>
        <Col xs={24} sm={12} md={8}>
          <Card
            size="small"
            hoverable
            className="quick-entry-card"
            onClick={() => navigate('/clusters')}
            style={{ cursor: 'pointer' }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
              <div className="icon-bg icon-bg-primary" style={{ width: 44, height: 44 }}>
                <ClusterOutlined />
              </div>
              <div>
                <div style={{ fontWeight: 600, fontSize: 14 }}>集群管理</div>
                <div style={{ fontSize: 12, color: 'var(--text-tertiary)', marginTop: 2 }}>管理 Kafka 集群配置与认证</div>
              </div>
            </div>
          </Card>
        </Col>
        <Col xs={24} sm={12} md={8}>
          <Card
            size="small"
            hoverable
            className="quick-entry-card"
            onClick={() => navigate('/monitor')}
            style={{ cursor: 'pointer' }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
              <div className="icon-bg icon-bg-success" style={{ width: 44, height: 44 }}>
                <DashboardOutlined />
              </div>
              <div>
                <div style={{ fontWeight: 600, fontSize: 14 }}>监控中心</div>
                <div style={{ fontSize: 12, color: 'var(--text-tertiary)', marginTop: 2 }}>实时监控集群性能指标</div>
              </div>
            </div>
          </Card>
        </Col>
        <Col xs={24} sm={12} md={8}>
          <Card
            size="small"
            hoverable
            className="quick-entry-card"
            onClick={() => navigate('/topics')}
            style={{ cursor: 'pointer' }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
              <div className="icon-bg icon-bg-info" style={{ width: 44, height: 44 }}>
                <FileTextOutlined />
              </div>
              <div>
                <div style={{ fontWeight: 600, fontSize: 14 }}>Topic 管理</div>
                <div style={{ fontSize: 12, color: 'var(--text-tertiary)', marginTop: 2 }}>创建、查看、管理 Topic</div>
              </div>
            </div>
          </Card>
        </Col>
      </Row>
    </div>
  )
}

export default Dashboard
