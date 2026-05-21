import { useEffect, useState } from 'react'
import { Card, Row, Col, Statistic } from 'antd'
import { ClusterOutlined, FileTextOutlined, UserOutlined, DashboardOutlined } from '@ant-design/icons'
import api from '../services/api'

interface DashboardStats {
  cluster_count: number
  topic_count: number
  scram_user_count: number
}

interface MonitorStatus {
  status: string    // "正常", "异常", "未启用"
  enabled: boolean
  reachable: boolean
}

const monitorStatusColor: Record<string, string> = {
  '正常': '#52c41a',
  '异常': '#ff4d4f',
  '未启用': '#d9d9d9',
}

const DashboardPage: React.FC = () => {
  const [stats, setStats] = useState<DashboardStats>({
    cluster_count: 0,
    topic_count: 0,
    scram_user_count: 0,
  })
  const [monitorStatus, setMonitorStatus] = useState<MonitorStatus | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetchData()
  }, [])

  const fetchData = async () => {
    try {
      const [statsRes, statusRes] = await Promise.all([
        api.get<DashboardStats>('/dashboard/stats'),
        api.get<MonitorStatus>('/dashboard/monitor-status'),
      ])
      setStats(statsRes.data)
      setMonitorStatus(statusRes.data)
    } catch (error) {
      console.error('Failed to fetch dashboard data:', error)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div>
      <h1 style={{ marginBottom: 24 }}>仪表盘</h1>
      <Row gutter={16}>
        <Col span={6}>
          <Card loading={loading}>
            <Statistic
              title="集群数量"
              value={stats.cluster_count}
              prefix={<ClusterOutlined />}
              valueStyle={{ color: '#1890ff' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card loading={loading}>
            <Statistic
              title="Topic 数量"
              value={stats.topic_count}
              prefix={<FileTextOutlined />}
              valueStyle={{ color: '#52c41a' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card loading={loading}>
            <Statistic
              title="SCRAM 用户"
              value={stats.scram_user_count}
              prefix={<UserOutlined />}
              valueStyle={{ color: '#722ed1' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card loading={loading}>
            <Statistic
              title="监控状态"
              value={monitorStatus?.status ?? '检测中'}
              prefix={<DashboardOutlined />}
              valueStyle={{ color: monitorStatus ? monitorStatusColor[monitorStatus.status] || '#faad14' : '#d9d9d9' }}
            />
          </Card>
        </Col>
      </Row>
    </div>
  )
}

export default DashboardPage
