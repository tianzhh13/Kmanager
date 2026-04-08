import { useState, useEffect } from 'react'
import { Card, Row, Col, Select, DatePicker, Spin, message, Statistic, Table, Tabs } from 'antd'
import { Line } from '@ant-design/charts'
import { clusterAPI } from '../services/cluster'
import { metricsAPI } from '../services/metrics'
import type { ColumnsType } from 'antd/es/table'

const { RangePicker } = DatePicker

interface ClusterOption {
  cluster_id: number
  cluster_name: string
}

interface ClusterMetrics {
  cluster_id: number
  broker_count: number
  topic_count: number
  message_rate: number
  bytes_in_rate: number
  bytes_out_rate: number
}

interface BrokerMetrics {
  broker_host: string
  cpu_usage: number
  memory_usage: number
  network_in_rate: number
  network_out_rate: number
}

interface TopicMetrics {
  topic_name: string
  message_rate_in: number
  bytes_rate_in: number
  bytes_rate_out: number
  partition_count: number
}

interface ConsumerGroupMetrics {
  consumer_group: string
  lag: number
  consume_rate: number
  member_count: number
}

const Monitor: React.FC = () => {
  const [loading, setLoading] = useState(false)
  const [clusters, setClusters] = useState<ClusterOption[]>([])
  const [selectedCluster, setSelectedCluster] = useState<number | null>(null)
  const [dateRange, setDateRange] = useState<[string, string]>([
    new Date(Date.now() - 3600000).toISOString(),
    new Date().toISOString()
  ])
  const [clusterMetrics, setClusterMetrics] = useState<ClusterMetrics | null>(null)
  const [brokerMetrics, setBrokerMetrics] = useState<BrokerMetrics[]>([])
  const [topicMetrics, setTopicMetrics] = useState<TopicMetrics[]>([])
  const [consumerGroupMetrics, setConsumerGroupMetrics] = useState<ConsumerGroupMetrics[]>([])
  const [activeTab, setActiveTab] = useState('overview')

  // 加载集群列表
  useEffect(() => {
    loadClusters()
  }, [])

  // 加载监控数据
  useEffect(() => {
    if (selectedCluster) {
      loadMetrics()
    }
  }, [selectedCluster, dateRange])

  const loadClusters = async () => {
    try {
      const res = await clusterAPI.list()
      setClusters(res.data || [])
      if (res.data?.length > 0) {
        setSelectedCluster(res.data[0].cluster_id)
      }
    } catch (error) {
      message.error('加载集群列表失败')
    }
  }

  const loadMetrics = async () => {
    if (!selectedCluster) return
    
    setLoading(true)
    try {
      const [start, end] = dateRange
      const params = { start, end }

      // 并行加载各类指标
      const [clusterRes, brokerRes, topicRes, consumerRes] = await Promise.allSettled([
        metricsAPI.getClusterMetrics(selectedCluster, params),
        metricsAPI.getBrokerMetrics(selectedCluster, params),
        metricsAPI.getTopicMetrics(selectedCluster, params),
        metricsAPI.getConsumerGroupMetrics(selectedCluster, params)
      ])

      if (clusterRes.status === 'fulfilled') {
        setClusterMetrics(clusterRes.value)
      }
      if (brokerRes.status === 'fulfilled') {
        setBrokerMetrics(brokerRes.value || [])
      }
      if (topicRes.status === 'fulfilled') {
        setTopicMetrics(topicRes.value || [])
      }
      if (consumerRes.status === 'fulfilled') {
        setConsumerGroupMetrics(consumerRes.value || [])
      }
    } catch (error) {
      message.error('加载监控数据失败')
    } finally {
      setLoading(false)
    }
  }

  const handleDateRangeChange = (dates: any, dateStrings: [string, string]) => {
    if (dates) {
      setDateRange(dateStrings)
    }
  }

  const brokerColumns: ColumnsType<BrokerMetrics> = [
    { title: 'Broker Host', dataIndex: 'broker_host', key: 'broker_host' },
    { 
      title: 'CPU 使用率', 
      dataIndex: 'cpu_usage', 
      key: 'cpu_usage',
      render: (val: number) => `${val?.toFixed(2) || 0}%`
    },
    { 
      title: '内存使用', 
      dataIndex: 'memory_usage', 
      key: 'memory_usage',
      render: (val: number) => val ? `${(val / 1024 / 1024).toFixed(2)} MB` : '-'
    },
    { 
      title: '网络流入', 
      dataIndex: 'network_in_rate', 
      key: 'network_in_rate',
      render: (val: number) => val ? `${val.toFixed(2)} MB/s` : '-'
    },
    { 
      title: '网络流出', 
      dataIndex: 'network_out_rate', 
      key: 'network_out_rate',
      render: (val: number) => val ? `${val.toFixed(2)} MB/s` : '-'
    }
  ]

  const topicColumns: ColumnsType<TopicMetrics> = [
    { title: 'Topic 名称', dataIndex: 'topic_name', key: 'topic_name' },
    { 
      title: '消息速率', 
      dataIndex: 'message_rate_in', 
      key: 'message_rate_in',
      render: (val: number) => val ? `${val.toFixed(2)} msg/s` : '-'
    },
    { 
      title: '字节流入', 
      dataIndex: 'bytes_rate_in', 
      key: 'bytes_rate_in',
      render: (val: number) => val ? `${(val / 1024 / 1024).toFixed(2)} MB/s` : '-'
    },
    { 
      title: '字节流出', 
      dataIndex: 'bytes_rate_out', 
      key: 'bytes_rate_out',
      render: (val: number) => val ? `${(val / 1024 / 1024).toFixed(2)} MB/s` : '-'
    },
    { 
      title: '分区数', 
      dataIndex: 'partition_count', 
      key: 'partition_count' 
    }
  ]

  const consumerColumns: ColumnsType<ConsumerGroupMetrics> = [
    { title: '消费组', dataIndex: 'consumer_group', key: 'consumer_group' },
    { 
      title: '消费延迟', 
      dataIndex: 'lag', 
      key: 'lag',
      render: (val: number) => val ? val.toFixed(0) : '-'
    },
    { 
      title: '消费速率', 
      dataIndex: 'consume_rate', 
      key: 'consume_rate',
      render: (val: number) => val ? `${val.toFixed(2)} msg/s` : '-'
    },
    { 
      title: '成员数', 
      dataIndex: 'member_count', 
      key: 'member_count' 
    }
  ]

  const tabItems = [
    {
      key: 'overview',
      label: '集群概览',
      children: (
        <Row gutter={16}>
          <Col span={6}>
            <Card>
              <Statistic 
                title="Broker 数量" 
                value={clusterMetrics?.broker_count || 0} 
                valueStyle={{ color: '#1890ff' }}
              />
            </Card>
          </Col>
          <Col span={6}>
            <Card>
              <Statistic 
                title="Topic 数量" 
                value={clusterMetrics?.topic_count || 0} 
                valueStyle={{ color: '#52c41a' }}
              />
            </Card>
          </Col>
          <Col span={6}>
            <Card>
              <Statistic 
                title="消息速率" 
                value={clusterMetrics?.message_rate || 0} 
                suffix="msg/s"
                valueStyle={{ color: '#faad14' }}
              />
            </Card>
          </Col>
          <Col span={6}>
            <Card>
              <Statistic 
                title="字节流入" 
                value={clusterMetrics?.bytes_in_rate ? (clusterMetrics.bytes_in_rate / 1024 / 1024).toFixed(2) : 0} 
                suffix="MB/s"
                valueStyle={{ color: '#f5222d' }}
              />
            </Card>
          </Col>
        </Row>
      )
    },
    {
      key: 'broker',
      label: 'Broker 监控',
      children: (
        <Table 
          dataSource={brokerMetrics} 
          columns={brokerColumns} 
          rowKey="broker_host"
          pagination={false}
        />
      )
    },
    {
      key: 'topic',
      label: 'Topic 监控',
      children: (
        <Table 
          dataSource={topicMetrics} 
          columns={topicColumns} 
          rowKey="topic_name"
          pagination={{ pageSize: 10 }}
        />
      )
    },
    {
      key: 'consumer',
      label: '消费组监控',
      children: (
        <Table 
          dataSource={consumerGroupMetrics} 
          columns={consumerColumns} 
          rowKey="consumer_group"
          pagination={{ pageSize: 10 }}
        />
      )
    }
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card 
        title="集群监控" 
        extra={
          <Space>
            <Select
              placeholder="选择集群"
              value={selectedCluster}
              onChange={setSelectedCluster}
              style={{ width: 200 }}
              options={clusters.map(c => ({ label: c.cluster_name, value: c.cluster_id }))}
            />
            <RangePicker
              showTime
              onChange={handleDateRangeChange}
              defaultValue={[
                dayjs(dateRange[0]), 
                dayjs(dateRange[1])
              ]}
            />
          </Space>
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

import { Space } from 'antd'
import dayjs from 'dayjs'

export default Monitor