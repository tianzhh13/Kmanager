import { useState, useEffect } from 'react'
import { Button, Modal, Form, Input, Select, message, Transfer } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, StopOutlined, CheckOutlined, SettingOutlined, ClusterOutlined } from '@ant-design/icons'
import api from '../services/api'
import { clusterAPI } from '../services/cluster'
import TopicPermissionModal from '../components/TopicPermissionModal'
import { StatCard, LabelTag, SearchBar, AvatarInitials } from '../components/bento'

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

const AVATAR_COLORS = ['#f97316', '#3b82f6', '#10b981', '#8b5cf6', '#ec4899', '#ef4444', '#f59e0b', '#06b6d4']

const roleColorMap: Record<string, 'red' | 'blue' | 'green'> = {
  super_admin: 'red',
  cluster_admin: 'blue',
  normal_user: 'green',
}
const roleLabelMap: Record<string, string> = {
  super_admin: 'SUPER ADMIN',
  cluster_admin: 'CLUSTER ADMIN',
  normal_user: 'USER',
}

const UserManagement: React.FC = () => {
  const [loading, setLoading] = useState(false)
  const [isModalVisible, setIsModalVisible] = useState(false)
  const [isEditModal, setIsEditModal] = useState(false)
  const [editingUser, setEditingUser] = useState<User | null>(null)
  const [users, setUsers] = useState<User[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize] = useState(20)
  const [form] = Form.useForm()
  const [permModalVisible, setPermModalVisible] = useState(false)
  const [selectedUser, setSelectedUser] = useState<User | null>(null)
  const [clusterAuthModalVisible, setClusterAuthModalVisible] = useState(false)
  const [clusterAuthUser, setClusterAuthUser] = useState<User | null>(null)
  const [allClusters, setAllClusters] = useState<Cluster[]>([])
  const [userClusters, setUserClusters] = useState<number[]>([])
  const [targetClusters, setTargetClusters] = useState<number[]>([])
  const [clusterAuthLoading, setClusterAuthLoading] = useState(false)
  const [searchText, setSearchText] = useState('')
  const [roleFilter, setRoleFilter] = useState<string>('')
  const [statusFilter, setStatusFilter] = useState<string>('')

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
    Modal.confirm({
      title: '确认删除',
      content: '确定要删除该用户吗？此操作不可恢复。',
      onOk: async () => {
        try {
          await api.delete(`/users/${userId}`)
          message.success('删除成功')
          fetchUsers()
        } catch (error: any) {
          message.error(error.response?.data?.error || '删除失败')
        }
      },
    })
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
      const clustersRes = await clusterAPI.list(1, 1000)
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

  const superAdminCount = users.filter(u => u.role === 'super_admin').length
  const clusterAdminCount = users.filter(u => u.role === 'cluster_admin').length
  const normalUserCount = users.filter(u => u.role === 'normal_user').length

  const filteredUsers = users.filter(u => {
    if (searchText && !u.username.toLowerCase().includes(searchText.toLowerCase()) && !u.email.toLowerCase().includes(searchText.toLowerCase())) return false
    if (roleFilter && u.role !== roleFilter) return false
    if (statusFilter && u.status !== statusFilter) return false
    return true
  })

  const gridCols = '36px 1.1fr 1.2fr 1fr 0.7fr 1.6fr 340px'

  const transferDataSource = allClusters.map(c => ({
    key: String(c.cluster_id),
    title: c.cluster_name,
  }))

  return (
    <div>
      {/* Header */}
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

      {/* Stat cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 16, marginBottom: 20 }}>
        <StatCard label="USER TOTAL" value={total} />
        <StatCard label="SUPER ADMIN" value={superAdminCount} color="red" />
        <StatCard label="CLUSTER ADMIN" value={clusterAdminCount} color="blue" />
        <StatCard label="NORMAL USER" value={normalUserCount} color="green" />
      </div>

      {/* Search & filters */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 20 }}>
        <SearchBar value={searchText} onChange={setSearchText} placeholder="搜索用户名或邮箱..." />
        <Select
          placeholder="全部角色"
          value={roleFilter || undefined}
          onChange={(val) => setRoleFilter(val || '')}
          style={{ width: 150 }}
          allowClear
        >
          <Select.Option value="super_admin">超级管理员</Select.Option>
          <Select.Option value="cluster_admin">集群管理员</Select.Option>
          <Select.Option value="normal_user">普通用户</Select.Option>
        </Select>
        <Select
          placeholder="全部状态"
          value={statusFilter || undefined}
          onChange={(val) => setStatusFilter(val || '')}
          style={{ width: 120 }}
          allowClear
        >
          <Select.Option value="active">活跃</Select.Option>
          <Select.Option value="disabled">禁用</Select.Option>
        </Select>
      </div>

      {/* Table header */}
      <div className="bento-table-header" style={{ gridTemplateColumns: gridCols }}>
        <div>ID</div>
        <div>Username</div>
        <div>Email</div>
        <div>Role</div>
        <div>Status</div>
        <div>Created</div>
        <div style={{ textAlign: 'right' }}>Actions</div>
      </div>

      {/* Table body */}
      <div className="bento-table-body">
        {loading && <div style={{ textAlign: 'center', padding: 48, color: 'var(--text-3)' }}>加载中...</div>}
        {!loading && filteredUsers.map((user, idx) => {
          const isDisabled = user.status === 'disabled'
          return (
            <div
              key={user.user_id}
              className={`bento-table-row${isDisabled ? ' bento-table-row--disabled' : ''}`}
              style={{ gridTemplateColumns: gridCols }}
            >
              <div style={{ fontSize: 12, color: 'var(--text-3)' }}>{user.user_id}</div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                <AvatarInitials
                  name={user.username}
                  color={AVATAR_COLORS[idx % AVATAR_COLORS.length]}
                />
                <span className="bento-row-name" style={{ fontWeight: 700, fontSize: 14 }}>{user.username}</span>
              </div>
              <div style={{ fontSize: 12, color: 'var(--text-2)', fontFamily: 'var(--font-mono)' }}>{user.email}</div>
              <div>
                <LabelTag text={roleLabelMap[user.role] || user.role} color={roleColorMap[user.role] || 'neutral'} />
              </div>
              <div>
                <LabelTag text={user.status === 'active' ? 'ACTIVE' : 'DISABLED'} color={user.status === 'active' ? 'green' : 'red'} />
              </div>
              <div style={{ fontSize: 12, color: 'var(--text-3)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{user.created_at}</div>
              <div style={{ textAlign: 'right', display: 'flex', gap: 4, justifyContent: 'flex-end', flexWrap: 'nowrap' }}>
                <button className="bento-action-btn" style={{ padding: '3px 7px', fontSize: 12 }} onClick={() => handleEdit(user)}>
                  <EditOutlined /> 编辑
                </button>
                {user.role === 'cluster_admin' && (
                  <button className="bento-action-btn" style={{ padding: '3px 7px', fontSize: 12, borderColor: 'var(--info)', color: 'var(--info)' }} onClick={() => handleOpenClusterAuth(user)}>
                    <ClusterOutlined /> 授权集群
                  </button>
                )}
                {user.role === 'normal_user' && (
                  <button className="bento-action-btn" style={{ padding: '3px 7px', fontSize: 12, borderColor: 'var(--color-success)', color: 'var(--color-success)' }} onClick={() => handleOpenPermModal(user)}>
                    <SettingOutlined /> 分配权限
                  </button>
                )}
                <button
                  className="bento-action-btn"
                  style={{ padding: '3px 7px', fontSize: 12, color: user.status === 'active' ? 'var(--color-warning)' : 'var(--color-success)' }}
                  onClick={() => handleToggleStatus(user.user_id, user.status)}
                >
                  {user.status === 'active' ? <><StopOutlined /> 禁用</> : <><CheckOutlined /> 启用</>}
                </button>
                <button className="bento-action-btn bento-action-btn--danger" style={{ padding: '3px 7px', fontSize: 12 }} onClick={() => handleDelete(user.user_id)}>
                  <DeleteOutlined /> 删除
                </button>
              </div>
            </div>
          )
        })}
        {!loading && filteredUsers.length === 0 && (
          <div style={{ textAlign: 'center', padding: 48, color: 'var(--text-3)' }}>暂无用户数据</div>
        )}
      </div>

      {/* Pagination */}
      {total > pageSize && (
        <div className="bento-pagination">
          <span className="bento-pagination-info">
            Showing {(page - 1) * pageSize + 1}-{Math.min(page * pageSize, total)} of {total}
          </span>
          <div className="bento-pagination-buttons">
            <button className="bento-pagination-btn" disabled={page <= 1} onClick={() => setPage(page - 1)}>&larr;</button>
            {Array.from({ length: Math.ceil(total / pageSize) }, (_, i) => i + 1)
              .filter(p => Math.abs(p - page) <= 2)
              .map(p => (
                <button
                  key={p}
                  className={`bento-pagination-btn${p === page ? ' bento-pagination-btn--active' : ''}`}
                  onClick={() => setPage(p)}
                >{p}</button>
              ))}
            <button className="bento-pagination-btn" disabled={page >= Math.ceil(total / pageSize)} onClick={() => setPage(page + 1)}>&rarr;</button>
          </div>
        </div>
      )}

      {/* Create User Modal */}
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

      {/* Edit User Modal */}
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

      {/* Cluster Auth Modal */}
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
