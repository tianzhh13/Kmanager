import { useEffect, useState } from 'react'
import { Table, Button, Space, Modal, Form, Input, Select, InputNumber, message, Tag } from 'antd'
import { PlusOutlined, DeleteOutlined, SyncOutlined } from '@ant-design/icons'
import { useAppDispatch, useAppSelector } from '../store/hooks'
import { fetchTopics, createTopic, deleteTopic } from '../store/slices/topicSlice'
import { clusterAPI } from '../services/cluster'

const TopicList: React.FC = () => {
  const dispatch = useAppDispatch()
  const { topics, total, loading } = useAppSelector((state) => state.topics)
  const [isModalVisible, setIsModalVisible] = useState(false)
  const [form] = Form.useForm()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [clusters, setClusters] = useState<any[]>([])

  useEffect(() => {
    dispatch(fetchTopics({ page, pageSize }))
    clusterAPI.list(1, 100).then(res => setClusters(res.data || []))
  }, [dispatch, page, pageSize])

  const handleCreate = async () => {
    try {
      const values = await form.validateFields()
      await dispatch(createTopic(values)).unwrap()
      message.success('创建成功')
      setIsModalVisible(false)
      form.resetFields()
      dispatch(fetchTopics({ page, pageSize }))
    } catch (error: any) {
      message.error(error || '创建失败')
    }
  }

  const handleDelete = async (topicName: string) => {
    Modal.confirm({
      title: '确认删除',
      content: `确定要删除 Topic "${topicName}" 吗？此操作不可恢复。`,
      onOk: async () => {
        try {
          await dispatch(deleteTopic(topicName)).unwrap()
          message.success('删除成功')
        } catch (error: any) {
          message.error(error || '删除失败')
        }
      },
    })
  }

  const columns = [
    { title: 'Topic 名称', dataIndex: 'topic_name', key: 'topic_name' },
    { title: '分区数', dataIndex: 'partitions', key: 'partitions', width: 100 },
    { title: '副本数', dataIndex: 'replication_factor', key: 'replication_factor', width: 100 },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at' },
    { title: '操作', key: 'action', width: 150,
      render: (_: any, record: any) => (
        <Space>
          <Button type="link" danger icon={<DeleteOutlined />} onClick={() => handleDelete(record.topic_name)}>
            删除
          </Button>
        </Space>
      )
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h1>Topic 管理</h1>
        <Space>
          <Button icon={<SyncOutlined />}>同步</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setIsModalVisible(true)}>
            创建 Topic
          </Button>
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
      />

      <Modal
        title="创建 Topic"
        open={isModalVisible}
        onOk={handleCreate}
        onCancel={() => { setIsModalVisible(false); form.resetFields() }}
        width={600}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="cluster_id" label="所属集群" rules={[{ required: true }]}>
            <Select placeholder="选择集群">
              {clusters.map(c => (
                <Select.Option key={c.id} value={c.id}>{c.cluster_name}</Select.Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item name="topic_name" label="Topic 名称" rules={[{ required: true }]}>
            <Input placeholder="请输入 Topic 名称" />
          </Form.Item>
          <Form.Item name="partitions" label="分区数" rules={[{ required: true }]}>
            <InputNumber min={1} max={100} defaultValue={1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="replication_factor" label="副本数" rules={[{ required: true }]}>
            <InputNumber min={1} max={10} defaultValue={1} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default TopicList