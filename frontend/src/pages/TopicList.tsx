import { useEffect, useState } from 'react'
import { Table, Button, Space, Modal, Form, Input, Select, InputNumber, message, Tag, Spin, Row, Col, Statistic, Card } from 'antd'
import { PlusOutlined, DeleteOutlined, SyncOutlined, LineChartOutlined, SettingOutlined, TeamOutlined, FileTextOutlined, PartitionOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { useAppDispatch, useAppSelector } from '../store/hooks'
import { fetchTopics, createTopic, deleteTopic } from '../store/slices/topicSlice'
import { clusterAPI } from '../services/cluster'
import { topicService } from '../services/topic'

interface Cluster {
  cluster_id: number
  cluster_name: string
}

const TopicList: React.FC = () => {
  const dispatch = useAppDispatch()
  const navigate = useNavigate()
  const { topics, total, loading } = useAppSelector((state) => state.topics)
  const { user } = useAppSelector((state) => state.auth)
  const isNormalUser = user?.role === 'normal_user'
  const [isModalVisible, setIsModalVisible] = useState(false)
  const [form] = Form.useForm()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [clusters, setClusters] = useState<Cluster[]>([])
  const [clustersLoading, setClustersLoading] = useState(false)
  const [selectedClusterId, setSelectedClusterId] = useState<number | null>(null)
  const [syncing, setSyncing] = useState(false)
  const [configModalVisible, setConfigModalVisible] = useState(false)
  const [cgModalVisible, setCgModalVisible] = useState(false)
  const [configTopicName, setConfigTopicName] = useState('')
  const [configData, setConfigData] = useState<any[]>([])
  const [configLoading, setConfigLoading] = useState(false)
  const [cgData, setCgData] = useState<any[]>([])
  const [cgLoading, setCgLoading] = useState(false)

  useEffect(() => {
    const loadClusters = async () => {
      setClustersLoading(true)
      try {
        const res = await clusterAPI.list(1, 100)
        const clusterList = res.data || []
        setClusters(clusterList)
        if (clusterList.length > 0) {
          setSelectedClusterId(clusterList[0].cluster_id)
        }
      } catch (err) {
        console.error('Failed to load clusters:', err)
      } finally {
        setClustersLoading(false)
      }
    }
    loadClusters()
  }, [])

  useEffect(() => {
    if (selectedClusterId) {
      dispatch(fetchTopics({ page, pageSize, clusterId: selectedClusterId }))
    }
  }, [dispatch, page, pageSize, selectedClusterId])

  const handleOpenModal = () => {
    if (!selectedClusterId) {
      message.warning('请先选择集群')
      return
    }
    setIsModalVisible(true)
  }

  useEffect(() => {
    if (isModalVisible && selectedClusterId) {
      form.setFieldsValue({
        cluster_id: selectedClusterId,
        partitions: 1,
        replication_factor: 1,
      })
    }
  }, [isModalVisible, selectedClusterId, form])

  const handleCreate = async () => {
    try {
      const values = await form.validateFields()
      const clusterId = Number(values.cluster_id)
      if (isNaN(clusterId) || clusterId <= 0) {
        message.error('请选择有效的集群')
        return
      }
      const payload = {
        cluster_id: clusterId,
        topic_name: values.topic_name,
        partitions: Number(values.partitions),
        replication_factor: Number(values.replication_factor),
      }
      await dispatch(createTopic(payload)).unwrap()
      message.success('创建成功')
      setIsModalVisible(false)
      form.resetFields()
      if (selectedClusterId) {
        dispatch(fetchTopics({ page, pageSize, clusterId: selectedClusterId }))
      }
    } catch (error: any) {
      const errorMsg = typeof error === 'string' 
        ? error 
        : (error?.message || error?.error || '创建失败')
      message.error(errorMsg)
    }
  }

  const handleDelete = async (topicName: string) => {
    if (!selectedClusterId) {
      message.warning('请先选择集群')
      return
    }
    Modal.confirm({
      title: '确认删除',
      content: `确定要删除 Topic "${topicName}" 吗？此操作不可恢复。`,
      onOk: async () => {
        try {
          await dispatch(deleteTopic({ topicName, clusterId: selectedClusterId })).unwrap()
          message.success('删除成功')
        } catch (error: any) {
          const errorMsg = typeof error === 'string' 
            ? error 
            : (error?.message || error?.error || '删除失败')
          message.error(errorMsg)
        }
      },
    })
  }

  const handleSync = async () => {
    if (!selectedClusterId) {
      message.warning('请先选择集群')
      return
    }
    setSyncing(true)
    try {
      await topicService.sync(selectedClusterId)
      message.success('同步成功')
      dispatch(fetchTopics({ page, pageSize, clusterId: selectedClusterId }))
    } catch (error: any) {
      const errorMsg = error?.response?.data?.error || error?.message || '同步失败'
      message.error(errorMsg)
    } finally {
      setSyncing(false)
    }
  }

  const handleGoToMonitor = (topicName: string) => {
    if (!selectedClusterId) return
    navigate(`/monitor?clusterId=${selectedClusterId}&tab=topic&topicName=${encodeURIComponent(topicName)}`)
  }

  const handleViewConfig = async (topicName: string) => {
    if (!selectedClusterId) return
    setConfigTopicName(topicName)
    setConfigModalVisible(true)
    setConfigLoading(true)
    setConfigData([])
    try {
      const res = await topicService.getConfig(topicName, selectedClusterId)
      setConfigData(res.data || [])
    } catch (error: any) {
      message.error(error?.response?.data?.error || '获取配置失败')
    } finally {
      setConfigLoading(false)
    }
  }

  const handleViewConsumerGroups = async (topicName: string) => {
    if (!selectedClusterId) return
    setCgModalVisible(true)
    setCgLoading(true)
    setCgData([])
    try {
      const res = await topicService.getConsumerGroups(topicName, selectedClusterId)
      setCgData(res.data || [])
    } catch (error: any) {
      message.error(error?.response?.data?.error || '获取消费组失败')
    } finally {
      setCgLoading(false)
    }
  }

  const columns = [
    {
      title: 'Topic 名称',
      dataIndex: 'topic_name',
      key: 'topic_name',
      render: (text: string) => (
        <Space>
          <a onClick={() => handleGoToMonitor(text)} style={{ fontWeight: 500 }}>{text}</a>
          <Button
            type="link"
            size="small"
            icon={<LineChartOutlined />}
            onClick={() => handleGoToMonitor(text)}
            title="查看监控"
            style={{ padding: 0 }}
          />
        </Space>
      )
    },
    { title: '分区数', dataIndex: 'partitions', key: 'partitions', width: 100,
      render: (v: number) => <Tag>{v}</Tag>,
    },
    { title: '副本数', dataIndex: 'replication_factor', key: 'replication_factor', width: 100,
      render: (v: number) => <Tag>{v}</Tag>,
    },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at',
      render: (text: string) => <span style={{ color: 'var(--text-tertiary)', fontSize: 13 }}>{text}</span>,
    },
    { title: '操作', key: 'action', width: 280,
      render: (_: any, record: any) => (
        <Space>
          <Button type="link" icon={<SettingOutlined />} onClick={() => handleViewConfig(record.topic_name)}>
            配置
          </Button>
          <Button type="link" icon={<TeamOutlined />} onClick={() => handleViewConsumerGroups(record.topic_name)}>
            消费组
          </Button>
          {!isNormalUser && (
            <Button type="link" danger icon={<DeleteOutlined />} onClick={() => handleDelete(record.topic_name)}>
              删除
            </Button>
          )}
        </Space>
      )
    },
  ]

  return (
    <div>
      <div className="page-header">
        <div className="page-header-row">
          <div>
            <h1>Topic 管理</h1>
            <div className="page-accent-line" />
          </div>
          <Space>
            <Select
              placeholder="选择集群"
              value={selectedClusterId}
              onChange={(value) => { setSelectedClusterId(value); setPage(1) }}
              style={{ width: 200 }}
              loading={clustersLoading}
            >
              {clusters.map(c => (
                <Select.Option key={c.cluster_id} value={c.cluster_id}>{c.cluster_name}</Select.Option>
              ))}
            </Select>
            {!isNormalUser && (
              <>
                <Button 
                  icon={<SyncOutlined spin={syncing} />} 
                  onClick={handleSync}
                  loading={syncing}
                  disabled={!selectedClusterId}
                >
                  同步
                </Button>
                <Button 
                  type="primary" 
                  icon={<PlusOutlined />} 
                  onClick={handleOpenModal}
                  disabled={!selectedClusterId}
                >
                  创建 Topic
                </Button>
              </>
            )}
          </Space>
        </div>
      </div>

      {/* 统计卡片 */}
      <Row gutter={[12, 12]} style={{ marginBottom: 20 }}>
        <Col xs={12} sm={8}>
          <Card size="small" className="stat-card">
            <Statistic title="Topic 总数" value={total} prefix={<FileTextOutlined />}
              valueStyle={{ fontWeight: 700, fontSize: 22, color: 'var(--color-success)' }} />
          </Card>
        </Col>
        <Col xs={12} sm={8}>
          <Card size="small" className="stat-card">
            <Statistic title="总分区数"
              value={topics.reduce((sum, t: any) => sum + (t.partitions || 0), 0)}
              prefix={<PartitionOutlined />}
              valueStyle={{ fontWeight: 700, fontSize: 22, color: 'var(--color-info)' }} />
          </Card>
        </Col>
        <Col xs={24} sm={8}>
          <Card size="small" className="stat-card">
            <Statistic title="总副本数"
              value={topics.reduce((sum, t: any) => sum + ((t.partitions || 0) * (t.replication_factor || 0)), 0)}
              prefix={<TeamOutlined />}
              valueStyle={{ fontWeight: 700, fontSize: 22, color: 'var(--brand-accent)' }} />
          </Card>
        </Col>
      </Row>
      
      <Table
        columns={columns}
        dataSource={topics}
        rowKey="id"
        loading={loading}
        pagination={{
          current: page,
          pageSize,
          total,
          onChange: (p, ps) => { setPage(p); setPageSize(ps) },
        }}
        locale={{ emptyText: selectedClusterId ? '暂无 Topic 数据' : '请先选择集群' }}
      />

      <Modal
        title="创建 Topic"
        open={isModalVisible}
        onOk={handleCreate}
        onCancel={() => { setIsModalVisible(false); form.resetFields() }}
        destroyOnClose
        width={600}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="cluster_id" label="所属集群" rules={[{ required: true }]}>
            <Select placeholder="请选择集群" disabled>
              {clusters.map(c => (
                <Select.Option key={c.cluster_id} value={c.cluster_id}>{c.cluster_name}</Select.Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item name="topic_name" label="Topic 名称" rules={[{ required: true, message: '请输入 Topic 名称' }]}>
            <Input placeholder="请输入 Topic 名称" />
          </Form.Item>
          <Form.Item name="partitions" label="分区数" rules={[{ required: true, message: '请输入分区数' }]} initialValue={1}>
            <InputNumber min={1} max={100} style={{ width: '100%' }} placeholder="请输入分区数" />
          </Form.Item>
          <Form.Item name="replication_factor" label="副本数" rules={[{ required: true, message: '请输入副本数' }]} initialValue={1}>
            <InputNumber min={1} max={10} style={{ width: '100%' }} placeholder="请输入副本数" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={`Topic 配置 - ${configTopicName}`}
        open={configModalVisible}
        footer={null}
        onCancel={() => setConfigModalVisible(false)}
        width={720}
      >
        <Spin spinning={configLoading}>
          <Table
            dataSource={configData}
            rowKey="name"
            size="small"
            pagination={false}
            scroll={{ y: 500 }}
            columns={[
              { title: '配置项', dataIndex: 'name', width: 220, render: (v: string) => <span className="text-mono" style={{ fontSize: 12 }}>{v}</span> },
              { title: '值', dataIndex: 'value', width: 200, render: (v: string) => <span className="text-mono" style={{ fontSize: 12 }}>{v}</span> },
              { title: '来源', dataIndex: 'source', width: 140 },
              { title: '只读', dataIndex: 'read_only', width: 70, render: (v: boolean) => <Tag color={v ? 'default' : 'success'}>{v ? '是' : '否'}</Tag> },
              { title: '默认值', dataIndex: 'is_default', width: 70, render: (v: boolean) => <Tag color={v ? 'default' : 'processing'}>{v ? '是' : '否'}</Tag> },
            ]}
          />
        </Spin>
      </Modal>

      <Modal
        title={`消费组 - ${configTopicName}`}
        open={cgModalVisible}
        footer={null}
        onCancel={() => setCgModalVisible(false)}
        width={600}
      >
        <Spin spinning={cgLoading}>
          {cgData.length === 0 && !cgLoading && (
            <div style={{ textAlign: 'center', color: 'var(--text-tertiary)', padding: 40, fontSize: 13 }}>该 Topic 暂无消费组</div>
          )}
          <Table
            dataSource={cgData}
            rowKey="group_id"
            size="small"
            pagination={false}
            columns={[
              { title: '消费组', dataIndex: 'group_id', render: (v: string) => <span className="text-mono" style={{ fontSize: 12 }}>{v}</span> },
              { title: '状态', dataIndex: 'state', width: 140, render: (state: string) => {
                const colorMap: Record<string, string> = { Stable: 'success', Empty: 'warning', Dead: 'error' }
                return <Tag color={colorMap[state] || 'default'}>{state}</Tag>
              }},
              { title: '成员数', dataIndex: 'member_count', width: 80 },
            ]}
          />
        </Spin>
      </Modal>
    </div>
  )
}

export default TopicList
