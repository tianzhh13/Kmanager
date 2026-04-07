import { useState, useEffect } from 'react'
import { Table, Button, Space, Card, Modal, Form, Select, Input, message } from 'antd'
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons'
import { getACLs, createACL, deleteACL } from '../services/acl'

const ACLList: React.FC = () => {
  const [loading, setLoading] = useState(false)
  const [acls, setAcls] = useState<any[]>([])
  const [clusters, setClusters] = useState<any[]>([])
  const [modalVisible, setModalVisible] = useState(false)
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([])
  const [form] = Form.useForm()

  const columns = [
    { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
    { title: '资源类型', dataIndex: 'resource_type', key: 'resource_type' },
    { title: '资源名称', dataIndex: 'resource_name', key: 'resource_name' },
    { title: 'Principal', dataIndex: 'principal', key: 'principal' },
    { title: '操作', dataIndex: 'operation', key: 'operation' },
    { title: '权限', dataIndex: 'permission', key: 'permission' },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at' },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: any) => (
        <Button type="link" danger icon={<DeleteOutlined />} onClick={() => handleDelete(record.id)}>
          删除
        </Button>
      ),
    },
  ]

  useEffect(() => {
    fetchACLs()
  }, [])

  const fetchACLs = async () => {
    setLoading(true)
    try {
      const data = await getACLs()
      setAcls(data)
    } catch (error) {
      message.error('获取ACL列表失败')
    } finally {
      setLoading(false)
    }
  }

  const handleCreate = async (values: any) => {
    try {
      await createACL(values)
      message.success('创建ACL成功')
      setModalVisible(false)
      form.resetFields()
      fetchACLs()
    } catch (error) {
      message.error('创建ACL失败')
    }
  }

  const handleDelete = async (id: number) => {
    Modal.confirm({
      title: '确认删除',
      content: '确定要删除这条ACL规则吗？',
      onOk: async () => {
        try {
          await deleteACL(id)
          message.success('删除成功')
          fetchACLs()
        } catch (error) {
          message.error('删除失败')
        }
      },
    })
  }

  const handleBatchDelete = async () => {
    if (selectedRowKeys.length === 0) {
      message.warning('请选择要删除的ACL')
      return
    }
    Modal.confirm({
      title: '批量删除',
      content: `确定要删除选中的 ${selectedRowKeys.length} 条ACL规则吗？`,
      onOk: async () => {
        try {
          for (const id of selectedRowKeys) {
            await deleteACL(id as number)
          }
          message.success('批量删除成功')
          setSelectedRowKeys([])
          fetchACLs()
        } catch (error) {
          message.error('批量删除失败')
        }
      },
    })
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h1>ACL 管理</h1>
        <Space>
          {selectedRowKeys.length > 0 && (
            <Button danger icon={<DeleteOutlined />} onClick={handleBatchDelete}>
              批量删除 ({selectedRowKeys.length})
            </Button>
          )}
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalVisible(true)}>
            创建 ACL
          </Button>
        </Space>
      </div>
      <Card>
        <Table
          columns={columns}
          dataSource={acls}
          rowKey="id"
          loading={loading}
          pagination={{ pageSize: 20 }}
          rowSelection={{ selectedRowKeys, onChange: setSelectedRowKeys }}
          locale={{ emptyText: '暂无 ACL 数据' }}
        />
      </Card>

      <Modal
        title="创建 ACL"
        open={modalVisible}
        onCancel={() => {
          setModalVisible(false)
          form.resetFields()
        }}
        onOk={() => form.submit()}
      >
        <Form form={form} layout="vertical" onFinish={handleCreate}>
          <Form.Item name="cluster_id" label="集群" rules={[{ required: true, message: '请选择集群' }]}>
            <Select placeholder="选择集群">
              {clusters.map(c => (
                <Select.Option key={c.id} value={c.id}>{c.name}</Select.Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item name="resource_type" label="资源类型" rules={[{ required: true, message: '请选择资源类型' }]}>
            <Select placeholder="选择资源类型">
              <Select.Option value="Topic">Topic</Select.Option>
              <Select.Option value="Group">Group</Select.Option>
              <Select.Option value="Cluster">Cluster</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="resource_name" label="资源名称" rules={[{ required: true, message: '请输入资源名称' }]}>
            <Input placeholder="资源名称（如 test-topic）" />
          </Form.Item>
          <Form.Item name="principal" label="Principal" rules={[{ required: true, message: '请输入Principal' }]}>
            <Input placeholder="如 User:admin" />
          </Form.Item>
          <Form.Item name="operation" label="操作" rules={[{ required: true, message: '请选择操作' }]}>
            <Select placeholder="选择操作">
              <Select.Option value="Read">Read</Select.Option>
              <Select.Option value="Write">Write</Select.Option>
              <Select.Option value="Create">Create</Select.Option>
              <Select.Option value="Delete">Delete</Select.Option>
              <Select.Option value="Alter">Alter</Select.Option>
              <Select.Option value="Describe">Describe</Select.Option>
              <Select.Option value="All">All</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="permission" label="权限类型" rules={[{ required: true, message: '请选择权限类型' }]}>
            <Select placeholder="选择权限类型">
              <Select.Option value="Allow">Allow</Select.Option>
              <Select.Option value="Deny">Deny</Select.Option>
            </Select>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default ACLList