import { useEffect, useState } from 'react'
import { Card, Row, Col, Statistic } from 'antd'
import { ClusterOutlined, FileTextOutlined, UserOutlined, DashboardOutlined } from '@ant-design/icons'
import api from '../services/api'

interface DashboardStats {
  cluster_count: number
  topic_count: number
  scram_user_count: number
}

const Dashboard: React.FC = () => {
  const [stats, setStats] = useState<DashboardStats>({
    cluster_count: 0,
    topic_count: 0,
    scram_user_count: 0,
  })
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const fetchStats = async () => {
      try {
        const res = await api.get<DashboardStats>('/dashboard/stats')
        setStats(res.data)
      } catch (error) {
        console.error('Failed to fetch stats:', error)
      } finally {
        setLoading(false)
      }
    }
    fetchStats()
  }, [])

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
              value="正常"
              prefix={<DashboardOutlined />}
              valueStyle={{ color: '#faad14' }}
            />
          </Card>
        </Col>
      </Row>
    </div>
  )
}

export default Dashboard
