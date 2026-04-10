import { useState, useEffect } from 'react'
import { Table, Button, Space, Modal, Form, Input, Select, Tag, message, Popconfirm } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, StopOutlined, CheckOutlined } from '@ant-design/icons'
import api from '../services/api'

interface User {
  id: number
  username: string
  email: string
  role: string
  status: string
  created_at: string
}

const UserManagement: React.FC = () => {
  const [loading, setLoading] = useState(false)
  const [isModalVisible, setIsModalVisible] = useState(false)
  const [users, setUsers] = useState<User[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [form] = Form.useForm()

  useEffect(() => {
    fetchUsers()
  }, [page, pageSize])

  const fetchUsers = async () => {
    setLoading(true)
    try {
      const response = await api.get('/users', { params: { page, page_size: pageSize } })
      setUsers(response.data.data || [])
      setTotal(response.data.total || 0)
    } catch (error: any) {
      message.error(error.response?.data?.error || '获取用户列表失败')
    } finally {
      setLoading(false)
    }
  }

  const handleCreate = async () => {
    try {
      const values = await form.validateFields()
      await api.post('/users', values)
      message.success('创建成功')
      setIsModalVisible(false)
      form.resetFields()
      fetchUsers()
    } catch (error: any) {
      message.error(error.response?.data?.error || '创建失败')
    }
  }

  const handleDelete = async (userId: number) => {
    try {
      await api.delete(`/users/${userId}`)
      message.success('删除成功')
      fetchUsers()
    } catch (error: any) {
      message.error(error.response?.data?.error || '删除失败')
    }
  }

  const handleToggleStatus = async (userId: number, currentStatus: string) => {
    try {
      if (currentStatus === 'active') {
        await api.put(`/users/${userId}/disable`)
        message.success('已禁用用户')
      } else {
        await api.put(`/users/${userId}/enable`)
        message.success('已启用用户')
      }
      fetchUsers()
    } catch (error: any) {
      message.error(error.response?.data?.error || '操作失败')
    }
  }

  const columns = [
    { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
    { title: '用户名', dataIndex: 'username', key: 'username' },
    { title: '邮箱', dataIndex: 'email', key: 'email' },
    { title: '角色', dataIndex: 'role', key: 'role',
      render: (role: string) => {
        const colorMap: Record<string, string> = {
          super_admin: 'red',
          cluster_admin: 'blue',
          readonly: 'green',
        }
        const labelMap: Record<string, string> = {
          super_admin: '超级管理员',
          cluster_admin: '集群管理员',
          readonly: '只读用户',
        }
        return <Tag color={colorMap[role] || 'default'}>{labelMap[role] || role}</Tag>
      }
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
      render: () => (
        <Space>
          <Button type="link" icon={<EditOutlined />}>编辑</Button>
          <Button type="link" icon={<StopOutlined />}>禁用</Button>
          <Button type="link" danger icon={<DeleteOutlined />}>删除</Button>
        </Space>
      )
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h1>用户管理</h1>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setIsModalVisible(true)}>
          创建用户
        </Button>
      </div>

      <Table
        columns={columns}
        dataSource={[]}
        rowKey="id"
        loading={loading}
        pagination={{ pageSize: 20 }}
        locale={{ emptyText: '暂无用户数据' }}
      />

      <Modal
        title="创建用户"
        open={isModalVisible}
        onOk={handleCreate}
        onCancel={() => { setIsModalVisible(false); form.resetFields() }}
        width={500}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="username" label="用户名" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input placeholder="3-64个字符" />
          </Form.Item>
          <Form.Item name="email" label="邮箱" rules={[{ required: true, type: 'email', message: '请输入有效邮箱' }]}>
            <Input placeholder="user@example.com" />
          </Form.Item>
          <Form.Item name="password" label="密码" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password placeholder="至少8个字符" />
          </Form.Item>
          <Form.Item name="role" label="角色" rules={[{ required: true }]}>
            <Select placeholder="选择角色">
              <Select.Option value="super_admin">超级管理员</Select.Option>
              <Select.Option value="cluster_admin">集群管理员</Select.Option>
              <Select.Option value="readonly">只读用户</Select.Option>
            </Select>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default UserManagement