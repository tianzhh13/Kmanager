import { useEffect, useState } from 'react'
import { Card, Row, Col, Statistic } from 'antd'
import { ClusterOutlined, FileTextOutlined, UserOutlined, LineChartOutlined } from '@ant-design/icons'
import { clusterService } from '../services/cluster'

const Dashboard: React.FC = () => {
  const [stats, setStats] = useState({
    clusterCount: 0,
    topicCount: 0,
    userCount: 0,
  })

  useEffect(() => {
    const fetchStats = async () => {
      try {
        const clusterRes = await clusterService.list(1, 1)
        setStats({
          clusterCount: clusterRes.total || 0,
          topicCount: 0,
          userCount: 0,
        })
      } catch (error) {
        console.error('Failed to fetch stats:', error)
      }
    }
    fetchStats()
  }, [])

  return (
    <div>
      <h1 style={{ marginBottom: 24 }}>仪表盘</h1>
      <Row gutter={16}>
        <Col span={6}>
          <Card>
            <Statistic
              title="集群数量"
              value={stats.clusterCount}
              prefix={<ClusterOutlined />}
              valueStyle={{ color: '#1890ff' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="Topic 数量"
              value={stats.topicCount}
              prefix={<FileTextOutlined />}
              valueStyle={{ color: '#52c41a' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="用户数量"
              value={stats.userCount}
              prefix={<UserOutlined />}
              valueStyle={{ color: '#722ed1' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="监控指标"
              value={0}
              prefix={<LineChartOutlined />}
              valueStyle={{ color: '#faad14' }}
            />
          </Card>
        </Col>
      </Row>
    </div>
  )
}

export default Dashboard