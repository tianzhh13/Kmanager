import { useState, useEffect } from 'react'
import { Table, Button, Space, Modal, Form, Input, Select, Tag, message, Popconfirm, Transfer } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, StopOutlined, CheckOutlined, SettingOutlined, ClusterOutlined } from '@ant-design/icons'
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
  // 集群授权相关
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

  // 编辑用户
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
      // 不传密码则不修改密码
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

  // 修复：禁用/启用用 POST（后端路由是 POST）
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

  // 集群授权
  const handleOpenClusterAuth = async (user: User) => {
    setClusterAuthUser(user)
    setClusterAuthModalVisible(true)
    setClusterAuthLoading(true)
    try {
      // 获取所有集群
      const clustersRes = await clusterAPI.list()
      const clusters = clustersRes.data || []
      setAllClusters(clusters)
      // 获取用户已授权的集群（使用新 API）
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
      // 计算需要新增和删除的集群
      const toAdd = targetClusters.filter(id => !userClusters.includes(id))
      const toRemove = userClusters.filter(id => !targetClusters.includes(id))

      // 批量授权
      for (const clusterId of toAdd) {
        await api.post(`/clusters/${clusterId}/grant`, { user_id: clusterAuthUser.user_id })
      }
      // 批量撤销
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
    { title: 'ID', dataIndex: 'user_id', key: 'user_id', width: 60 },
    { title: '用户名', dataIndex: 'username', key: 'username' },
    { title: '邮箱', dataIndex: 'email', key: 'email' },
    { title: '角色', dataIndex: 'role', key: 'role',
      render: (role: string) => {
        const colorMap: Record<string, string> = {
          super_admin: 'red',
          cluster_admin: 'blue',
          normal_user: 'green',
        }
        const labelMap: Record<string, string> = {
          super_admin: '超级管理员',
          cluster_admin: '集群管理员',
          normal_user: '普通用户',
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
    { title: '操作', key: 'action', width: 350,
      render: (_: any, record: User) => (
        <Space>
          <Button type="link" icon={<EditOutlined />} onClick={() => handleEdit(record)}>
            编辑
          </Button>
          {/* 集群管理员：授权集群 */}
          {record.role === 'cluster_admin' && (
            <Button type="link" icon={<ClusterOutlined />} onClick={() => handleOpenClusterAuth(record)}>
              授权集群
            </Button>
          )}
          {/* 普通用户：分配 Topic 权限 */}
          {record.role === 'normal_user' && (
            <Button type="link" icon={<SettingOutlined />} onClick={() => handleOpenPermModal(record)}>
              分配权限
            </Button>
          )}
          <Popconfirm
            title={record.status === 'active' ? '确定要禁用该用户吗？' : '确定要启用该用户吗？'}
            onConfirm={() => handleToggleStatus(record.user_id, record.status)}
          >
            <Button type="link" icon={record.status === 'active' ? <StopOutlined /> : <CheckOutlined />}>
              {record.status === 'active' ? '禁用' : '启用'}
            </Button>
          </Popconfirm>
          <Popconfirm
            title="确定要删除该用户吗？"
            onConfirm={() => handleDelete(record.user_id)}
          >
            <Button type="link" danger icon={<DeleteOutlined />}>删除</Button>
          </Popconfirm>
        </Space>
      )
    },
  ]

  // Transfer 数据源
  const transferDataSource = allClusters.map(c => ({
    key: String(c.cluster_id),
    title: c.cluster_name,
  }))

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

      {/* 创建用户弹窗 */}
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

      {/* 编辑用户弹窗 */}
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

      {/* 集群授权弹窗 */}
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

      {/* Topic 权限分配弹窗 */}
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
