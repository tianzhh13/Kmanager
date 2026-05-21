import { useEffect, useState } from 'react'
import { Table, Button, Space, Modal, Form, Input, Select, InputNumber, message } from 'antd'
import { PlusOutlined, DeleteOutlined, SyncOutlined, LineChartOutlined } from '@ant-design/icons'
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

  // 加载集群列表
  useEffect(() => {
    const loadClusters = async () => {
      setClustersLoading(true)
      try {
        const res = await clusterAPI.list(1, 100)
        const clusterList = res.data || []
        setClusters(clusterList)
        // 默认选择第一个集群
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

  // 当集群选择变化时，加载该集群的 Topic
  useEffect(() => {
    if (selectedClusterId) {
      dispatch(fetchTopics({ page, pageSize, clusterId: selectedClusterId }))
    }
  }, [dispatch, page, pageSize, selectedClusterId])

  // 打开创建弹窗
  const handleOpenModal = () => {
    if (!selectedClusterId) {
      message.warning('请先选择集群')
      return
    }
    setIsModalVisible(true)
  }

  // Modal 打开后设置表单值
  useEffect(() => {
    if (isModalVisible && selectedClusterId) {
      form.setFieldsValue({
        cluster_id: selectedClusterId,
        partitions: 1,
        replication_factor: 1,
      })
    }
  }, [isModalVisible, selectedClusterId, form])

  // 创建 Topic
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
      // 刷新列表
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

  // 删除 Topic
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

  // 同步 Topic
  const handleSync = async () => {
    if (!selectedClusterId) {
      message.warning('请先选择集群')
      return
    }
    setSyncing(true)
    try {
      await topicService.sync(selectedClusterId)
      message.success('同步成功')
      // 刷新列表
      dispatch(fetchTopics({ page, pageSize, clusterId: selectedClusterId }))
    } catch (error: any) {
      const errorMsg = error?.response?.data?.error || error?.message || '同步失败'
      message.error(errorMsg)
    } finally {
      setSyncing(false)
    }
  }

  // 跳转到监控页面
  const handleGoToMonitor = (topicName: string) => {
    if (!selectedClusterId) return
    navigate(`/monitor?clusterId=${selectedClusterId}&tab=topic&topicName=${encodeURIComponent(topicName)}`)
  }

  const columns = [
    {
      title: 'Topic 名称',
      dataIndex: 'topic_name',
      key: 'topic_name',
      render: (text: string) => (
        <Space>
          <a onClick={() => handleGoToMonitor(text)} style={{ color: '#1890ff' }}>{text}</a>
          <Button
            type="link"
            size="small"
            icon={<LineChartOutlined />}
            onClick={() => handleGoToMonitor(text)}
            title="查看监控"
          />
        </Space>
      )
    },
    { title: '分区数', dataIndex: 'partitions', key: 'partitions', width: 100 },
    { title: '副本数', dataIndex: 'replication_factor', key: 'replication_factor', width: 100 },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at' },
    { title: '操作', key: 'action', width: 150,
      render: (_: any, record: any) => (
        <Space>
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
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h1>Topic 管理</h1>
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
          <Form.Item name="cluster_id" label="所属集群" rules={[{ required: true, message: '请选择集群' }]}>
            <Select 
              placeholder="请选择集群" 
              disabled
            >
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
    </div>
  )
}

export default TopicList
