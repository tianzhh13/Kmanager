import { useState, useEffect } from 'react'
import { Table, Button, Space, Modal, Form, Input, Select, Tag, message, Popconfirm, Transfer, Row, Col, Statistic, Card } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, StopOutlined, CheckOutlined, SettingOutlined, ClusterOutlined, TeamOutlined, SafetyOutlined, UserSwitchOutlined } from '@ant-design/icons'
import api from '../services/api'
import { clusterAPI } from '../services/cluster'
import TopicPermissionModal from '../components/TopicPermissionModal'

interface User {
  user_id: number
  username: string
  email: string
  role: string
  status: string
  created_at: string
}

interface Cluster {
  cluster_id: number
  cluster_name: string
}

const UserManagement: React.FC = () => {
  const [loading, setLoading] = useState(false)
  const [isModalVisible, setIsModalVisible] = useState(false)
  const [isEditModal, setIsEditModal] = useState(false)
  const [editingUser, setEditingUser] = useState<User | null>(null)
  const [users, setUsers] = useState<User[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [form] = Form.useForm()
  const [permModalVisible, setPermModalVisible] = useState(false)
  const [selectedUser, setSelectedUser] = useState<User | null>(null)
  const [clusterAuthModalVisible, setClusterAuthModalVisible] = useState(false)
  const [clusterAuthUser, setClusterAuthUser] = useState<User | null>(null)
  const [allClusters, setAllClusters] = useState<Cluster[]>([])
  const [userClusters, setUserClusters] = useState<number[]>([])
  const [targetClusters, setTargetClusters] = useState<number[]>([])
  const [clusterAuthLoading, setClusterAuthLoading] = useState(false)

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

  const handleEdit = (record: User) => {
    setEditingUser(record)
    setIsEditModal(true)
    form.setFieldsValue({
      username: record.username,
      email: record.email,
      role: record.role,
    })
  }

  const handleUpdate = async () => {
    if (!editingUser) return
    try {
      const values = await form.validateFields()
      const updateData: any = {
        username: values.username,
        email: values.email,
        role: values.role,
      }
      if (values.password) {
        updateData.password = values.password
      }
      await api.put(`/users/${editingUser.user_id}`, updateData)
      message.success('更新成功')
      setIsEditModal(false)
      setEditingUser(null)
      form.resetFields()
      fetchUsers()
    } catch (error: any) {
      message.error(error.response?.data?.error || '更新失败')
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
        await api.post(`/users/${userId}/disable`)
        message.success('已禁用用户')
      } else {
        await api.post(`/users/${userId}/enable`)
        message.success('已启用用户')
      }
      fetchUsers()
    } catch (error: any) {
      message.error(error.response?.data?.error || '操作失败')
    }
  }

  const handleOpenPermModal = (user: User) => {
    setSelectedUser(user)
    setPermModalVisible(true)
  }

  const handleOpenClusterAuth = async (user: User) => {
    setClusterAuthUser(user)
    setClusterAuthModalVisible(true)
    setClusterAuthLoading(true)
    try {
      const clustersRes = await clusterAPI.list()
      const clusters = clustersRes.data || []
      setAllClusters(clusters)
      const userClustersRes = await api.get(`/clusters/user/${user.user_id}`)
      const userClusterIds = (userClustersRes.data.data || []).map((c: Cluster) => c.cluster_id)
      setUserClusters(userClusterIds)
      setTargetClusters(userClusterIds)
    } catch (error) {
      console.error('Failed to load cluster data', error)
    } finally {
      setClusterAuthLoading(false)
    }
  }

  const handleClusterAuthChange = (targetKeys: React.Key[]) => {
    setTargetClusters(targetKeys.map(Number))
  }

  const handleSaveClusterAuth = async () => {
    if (!clusterAuthUser) return
    setClusterAuthLoading(true)
    try {
      const toAdd = targetClusters.filter(id => !userClusters.includes(id))
      const toRemove = userClusters.filter(id => !targetClusters.includes(id))
      for (const clusterId of toAdd) {
        await api.post(`/clusters/${clusterId}/grant`, { user_id: clusterAuthUser.user_id })
      }
      for (const clusterId of toRemove) {
        await api.post(`/clusters/${clusterId}/revoke`, { user_id: clusterAuthUser.user_id })
      }
      message.success('集群授权更新成功')
      setClusterAuthModalVisible(false)
      setClusterAuthUser(null)
    } catch (error: any) {
      message.error(error.response?.data?.error || '集群授权失败')
    } finally {
      setClusterAuthLoading(false)
    }
  }

  const columns = [
    { title: 'ID', dataIndex: 'user_id', key: 'user_id', width: 60,
      render: (v: number) => <span style={{ color: 'var(--text-tertiary)' }}>{v}</span>,
    },
    { title: '用户名', dataIndex: 'username', key: 'username',
      render: (text: string) => <strong style={{ color: 'var(--text-heading)' }}>{text}</strong>,
    },
    { title: '邮箱', dataIndex: 'email', key: 'email',
      render: (text: string) => <span className="text-mono" style={{ fontSize: 12 }}>{text}</span>,
    },
    { title: '角色', dataIndex: 'role', key: 'role',
      render: (role: string) => {
        const colorMap: Record<string, string> = { super_admin: 'error', cluster_admin: 'processing', normal_user: 'success' }
        const labelMap: Record<string, string> = { super_admin: '超级管理员', cluster_admin: '集群管理员', normal_user: '普通用户' }
        return <Tag color={colorMap[role] || 'default'}>{labelMap[role] || role}</Tag>
      }
    },
    { title: '状态', dataIndex: 'status', key: 'status',
      render: (status: string) => (
        <Tag color={status === 'active' ? 'success' : 'error'}>{status === 'active' ? '活跃' : '禁用'}</Tag>
      )
    },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at',
      render: (text: string) => <span style={{ color: 'var(--text-tertiary)', fontSize: 13 }}>{text}</span>,
    },
    { title: '操作', key: 'action', width: 350,
      render: (_: any, record: User) => (
        <Space>
          <Button type="link" icon={<EditOutlined />} onClick={() => handleEdit(record)}>编辑</Button>
          {record.role === 'cluster_admin' && (
            <Button type="link" icon={<ClusterOutlined />} onClick={() => handleOpenClusterAuth(record)}>授权集群</Button>
          )}
          {record.role === 'normal_user' && (
            <Button type="link" icon={<SettingOutlined />} onClick={() => handleOpenPermModal(record)}>分配权限</Button>
          )}
          <Popconfirm
            title={record.status === 'active' ? '确定要禁用该用户吗？' : '确定要启用该用户吗？'}
            onConfirm={() => handleToggleStatus(record.user_id, record.status)}
          >
            <Button type="link" icon={record.status === 'active' ? <StopOutlined /> : <CheckOutlined />}>
              {record.status === 'active' ? '禁用' : '启用'}
            </Button>
          </Popconfirm>
          <Popconfirm title="确定要删除该用户吗？" onConfirm={() => handleDelete(record.user_id)}>
            <Button type="link" danger icon={<DeleteOutlined />}>删除</Button>
          </Popconfirm>
        </Space>
      )
    },
  ]

  const transferDataSource = allClusters.map(c => ({
    key: String(c.cluster_id),
    title: c.cluster_name,
  }))

  return (
    <div>
      <div className="page-header">
        <div className="page-header-row">
          <div>
            <h1>用户管理</h1>
            <div className="page-accent-line" />
          </div>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setIsModalVisible(true)}>
            创建用户
          </Button>
        </div>
      </div>

      {/* 统计卡片 */}
      <Row gutter={[12, 12]} style={{ marginBottom: 20 }}>
        <Col xs={12} sm={6}>
          <Card size="small" className="stat-card">
            <Statistic title="用户总数" value={total} prefix={<TeamOutlined />}
              valueStyle={{ fontWeight: 700, fontSize: 22, color: 'var(--brand-primary)' }} />
          </Card>
        </Col>
        <Col xs={12} sm={6}>
          <Card size="small" className="stat-card">
            <Statistic title="超级管理员"
              value={users.filter(u => u.role === 'super_admin').length}
              prefix={<SafetyOutlined />}
              valueStyle={{ fontWeight: 700, fontSize: 22, color: 'var(--color-error)' }} />
          </Card>
        </Col>
        <Col xs={12} sm={6}>
          <Card size="small" className="stat-card">
            <Statistic title="集群管理员"
              value={users.filter(u => u.role === 'cluster_admin').length}
              prefix={<UserSwitchOutlined />}
              valueStyle={{ fontWeight: 700, fontSize: 22, color: 'var(--color-info)' }} />
          </Card>
        </Col>
        <Col xs={12} sm={6}>
          <Card size="small" className="stat-card">
            <Statistic title="普通用户"
              value={users.filter(u => u.role === 'normal_user').length}
              prefix={<TeamOutlined />}
              valueStyle={{ fontWeight: 700, fontSize: 22, color: 'var(--color-success)' }} />
          </Card>
        </Col>
      </Row>

      <Table
        columns={columns}
        dataSource={users}
        rowKey="user_id"
        loading={loading}
        pagination={{
          current: page,
          pageSize,
          total,
          onChange: (p, ps) => { setPage(p); setPageSize(ps) },
        }}
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
              <Select.Option value="normal_user">普通用户</Select.Option>
            </Select>
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="编辑用户"
        open={isEditModal}
        onOk={handleUpdate}
        onCancel={() => { setIsEditModal(false); setEditingUser(null); form.resetFields() }}
        width={500}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="username" label="用户名" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input placeholder="3-64个字符" />
          </Form.Item>
          <Form.Item name="email" label="邮箱" rules={[{ required: true, type: 'email', message: '请输入有效邮箱' }]}>
            <Input placeholder="user@example.com" />
          </Form.Item>
          <Form.Item name="password" label="新密码" extra="留空则不修改密码">
            <Input.Password placeholder="至少8个字符" />
          </Form.Item>
          <Form.Item name="role" label="角色" rules={[{ required: true }]}>
            <Select placeholder="选择角色">
              <Select.Option value="super_admin">超级管理员</Select.Option>
              <Select.Option value="cluster_admin">集群管理员</Select.Option>
              <Select.Option value="normal_user">普通用户</Select.Option>
            </Select>
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={`集群授权 - ${clusterAuthUser?.username || ''}`}
        open={clusterAuthModalVisible}
        onOk={handleSaveClusterAuth}
        onCancel={() => { setClusterAuthModalVisible(false); setClusterAuthUser(null) }}
        confirmLoading={clusterAuthLoading}
        width={600}
      >
        <Transfer
          dataSource={transferDataSource}
          titles={['未授权', '已授权']}
          targetKeys={targetClusters.map(String)}
          onChange={handleClusterAuthChange}
          render={(item) => item.title}
          listStyle={{ width: 250, height: 300 }}
          showSearch
          filterOption={(inputValue, option) =>
            (option?.title as string).toLowerCase().includes(inputValue.toLowerCase())
          }
        />
      </Modal>

      <TopicPermissionModal
        visible={permModalVisible}
        user={selectedUser ? { user_id: selectedUser.user_id, username: selectedUser.username } : null}
        onClose={() => { setPermModalVisible(false); setSelectedUser(null) }}
        onSuccess={() => fetchUsers()}
      />
    </div>
  )
}

export default UserManagement
