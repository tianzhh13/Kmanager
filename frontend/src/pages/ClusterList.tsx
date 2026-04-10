import { useEffect, useState } from 'react'
import { Table, Button, Space, Modal, Form, Input, Select, message, Tag } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, CheckCircleOutlined } from '@ant-design/icons'
import { useAppDispatch, useAppSelector } from '../store/hooks'
import { fetchClusters, createCluster, testClusterConnection } from '../store/slices/clusterSlice'

const ClusterList: React.FC = () => {
  const dispatch = useAppDispatch()
  const { clusters, total, loading } = useAppSelector((state) => state.clusters)
  const [isModalVisible, setIsModalVisible] = useState(false)
  const [form] = Form.useForm()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)

  useEffect(() => {
    dispatch(fetchClusters({ page, pageSize }))
  }, [dispatch, page, pageSize])

  const handleCreate = async () => {
    try {
      const values = await form.validateFields()
      await dispatch(createCluster(values)).unwrap()
      message.success('创建成功')
      setIsModalVisible(false)
      form.resetFields()
      dispatch(fetchClusters({ page, pageSize }))
    } catch (error: any) {
      message.error(error || '创建失败')
    }
  }

  const handleTestConnection = async (id: number) => {
    try {
      await dispatch(testClusterConnection(id)).unwrap()
      message.success('连接测试成功')
    } catch (error: any) {
      message.error(error || '连接测试失败')
    }
  }

  const columns = [
    { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
    { title: '集群名称', dataIndex: 'cluster_name', key: 'cluster_name' },
    { title: 'Bootstrap Servers', dataIndex: 'bootstrap_servers', key: 'bootstrap_servers' },
    { title: '认证类型', dataIndex: 'auth_type', key: 'auth_type', 
      render: (type: string) => (
        <Tag color={type === 'plaintext' ? 'green' : type === 'scram' ? 'blue' : 'orange'}>
          {type.toUpperCase()}
        </Tag>
      )
    },
    { title: '状态', dataIndex: 'status', key: 'status',
      render: (status: string) => (
        <Tag color={status === 'active' ? 'success' : 'error'}>
          {status === 'active' ? '活跃' : '禁用'}
        </Tag>
      )
    },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at' },
    { title: '操作', key: 'action', width: 200,
      render: (_: any, record: any) => (
        <Space>
          <Button 
            type="link" 
            icon={<CheckCircleOutlined />} 
            onClick={() => handleTestConnection(record.id)}
          >
            测试
          </Button>
          <Button type="link" icon={<EditOutlined />}>编辑</Button>
          <Button type="link" danger icon={<DeleteOutlined />}>删除</Button>
        </Space>
      )
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h1>集群管理</h1>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setIsModalVisible(true)}>
          创建集群
        </Button>
      </div>
      
      <Table
        columns={columns}
        dataSource={clusters}
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
        title="创建集群"
        open={isModalVisible}
        onOk={handleCreate}
        onCancel={() => { setIsModalVisible(false); form.resetFields() }}
        width={600}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="cluster_name" label="集群名称" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="bootstrap_servers" label="Bootstrap Servers" rules={[{ required: true }]}>
            <Input placeholder="localhost:9092" />
          </Form.Item>
          <Form.Item name="auth_type" label="认证类型" rules={[{ required: true }]}>
            <Select>
              <Select.Option value="plaintext">PLAINTEXT</Select.Option>
              <Select.Option value="scram">SCRAM</Select.Option>
              <Select.Option value="kerberos">Kerberos</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="prometheus_url" label="Prometheus URL">
            <Input placeholder="http://localhost:9090" />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default ClusterList