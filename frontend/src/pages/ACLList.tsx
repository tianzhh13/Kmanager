import { useState, useEffect } from 'react'
import { Table, Button, Space, Card, Modal, Form, Select, Input, message, Tag } from 'antd'
import { PlusOutlined, DeleteOutlined, SyncOutlined, KeyOutlined, EyeOutlined } from '@ant-design/icons'
import { scramUserService, ScramUser } from '../services/scramUser'
import { clusterAPI } from '../services/cluster'
import { createACL, getACLs, deleteACL, ACL } from '../services/acl'

interface Cluster {
  cluster_id: number
  cluster_name: string
}

const ACLList: React.FC = () => {
  const [loading, setLoading] = useState(false)
  const [users, setUsers] = useState<ScramUser[]>([])
  const [clusters, setClusters] = useState<Cluster[]>([])
  const [selectedClusterId, setSelectedClusterId] = useState<number | null>(null)
  const [syncing, setSyncing] = useState(false)
  
  // 创建用户弹窗
  const [createUserVisible, setCreateUserVisible] = useState(false)
  const [createUserForm] = Form.useForm()
  
  // 权限设置弹窗
  const [aclModalVisible, setAclModalVisible] = useState(false)
  const [aclForm] = Form.useForm()
  const [selectedUsername, setSelectedUsername] = useState<string>('')
  
  // 查看权限弹窗
  const [viewAclVisible, setViewAclVisible] = useState(false)
  const [viewAclUsername, setViewAclUsername] = useState<string>('')
  const [userAcls, setUserAcls] = useState<ACL[]>([])
  const [viewAclLoading, setViewAclLoading] = useState(false)
  
  const columns = [
    { title: '用户名', dataIndex: 'username', key: 'username' },
    { title: '认证机制', dataIndex: 'mechanism', key: 'mechanism', width: 150 },
    { title: '同步状态', dataIndex: 'sync_status', key: 'sync_status', width: 100 },
    { title: '最后同步时间', dataIndex: 'last_sync_at', key: 'last_sync_at', width: 180 },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 180 },
    {
      title: '操作',
      key: 'action',
      width: 250,
      render: (_: any, record: ScramUser) => (
        <Space>
          <Button 
            type="link" 
            icon={<EyeOutlined />} 
            onClick={() => handleViewAcls(record.username)}
          >
            查看权限
          </Button>
          <Button 
            type="link" 
            icon={<KeyOutlined />} 
            onClick={() => handleOpenAclModal(record.username)}
          >
            权限设置
          </Button>
          <Button 
            type="link" 
            danger 
            icon={<DeleteOutlined />} 
            onClick={() => handleDeleteUser(record.username)}
          >
            删除
          </Button>
        </Space>
      ),
    },
  ]

  // 加载集群列表
  useEffect(() => {
    const loadClusters = async () => {
      try {
        const res = await clusterAPI.list(1, 100)
        console.log('=== DEBUG: Cluster list ===', res)
        const clusterList = res.data || []
        setClusters(clusterList)
        if (clusterList.length > 0) {
          setSelectedClusterId(clusterList[0].cluster_id)
        }
      } catch (err) {
        console.error('Failed to load clusters:', err)
      }
    }
    loadClusters()
  }, [])

  // 当集群选择变化时，加载该集群的用户
  useEffect(() => {
    if (selectedClusterId) {
      fetchUsers()
    }
  }, [selectedClusterId])

  const fetchUsers = async () => {
    if (!selectedClusterId) return
    setLoading(true)
    try {
      console.log('=== DEBUG: Fetching users for cluster ===', selectedClusterId)
      const res = await scramUserService.list(selectedClusterId)
      console.log('=== DEBUG: User list response ===', res)
      setUsers(res?.data || [])
    } catch (error: any) {
      console.error('=== DEBUG: Fetch users error ===', error)
      message.error('获取用户列表失败')
    } finally {
      setLoading(false)
    }
  }

  // 打开创建用户弹窗
  const handleOpenCreateUser = () => {
    if (!selectedClusterId) {
      message.warning('请先选择集群')
      return
    }
    createUserForm.setFieldsValue({ 
      cluster_id: selectedClusterId,
      mechanism: 'SCRAM-SHA-256' 
    })
    setCreateUserVisible(true)
  }

  // 创建用户
  const handleCreateUser = async (values: any) => {
    try {
      console.log('=== DEBUG: Create user form values ===', values)
      await scramUserService.create({
        cluster_id: selectedClusterId!,
        username: values.username,
        password: values.password,
        mechanism: values.mechanism,
      })
      message.success('用户创建成功')
      setCreateUserVisible(false)
      createUserForm.resetFields()
      fetchUsers()
    } catch (error: any) {
      console.error('=== DEBUG: Create user error ===', error)
      const errorMsg = error?.response?.data?.error || error?.message || '创建用户失败'
      message.error(errorMsg)
    }
  }

  // 删除用户
  const handleDeleteUser = async (username: string) => {
    if (!selectedClusterId) {
      message.warning('请先选择集群')
      return
    }
    Modal.confirm({
      title: '确认删除',
      content: `确定要删除用户 "${username}" 吗？此操作将同时删除 Kafka 中的用户凭证。`,
      onOk: async () => {
        try {
          await scramUserService.delete(username, selectedClusterId)
          message.success('删除成功')
          fetchUsers()
        } catch (error: any) {
          console.error('=== DEBUG: Delete user error ===', error)
          const errorMsg = error?.response?.data?.error || error?.message || '删除失败'
          message.error(errorMsg)
        }
      },
    })
  }

  // 同步用户
  const handleSync = async () => {
    if (!selectedClusterId) {
      message.warning('请先选择集群')
      return
    }
    setSyncing(true)
    try {
      await scramUserService.sync(selectedClusterId)
      message.success('同步成功')
      fetchUsers()
    } catch (error: any) {
      console.error('=== DEBUG: Sync users error ===', error)
      const errorMsg = error?.response?.data?.error || error?.message || '同步失败'
      message.error(errorMsg)
    } finally {
      setSyncing(false)
    }
  }

  // 打开权限设置弹窗
  const handleOpenAclModal = (username: string) => {
    if (!selectedClusterId) {
      message.warning('请先选择集群')
      return
    }
    setSelectedUsername(username)
    aclForm.setFieldsValue({
      cluster_id: selectedClusterId,
      principal: `User:${username}`,
    })
    setAclModalVisible(true)
  }

  // 查看用户权限
  const handleViewAcls = async (username: string) => {
    if (!selectedClusterId) {
      message.warning('请先选择集群')
      return
    }
    setViewAclUsername(username)
    setViewAclLoading(true)
    setViewAclVisible(true)
    try {
      const principal = `User:${username}`
      const acls = await getACLs({ cluster_id: selectedClusterId!, principal })
      setUserAcls(acls)
    } catch (error: any) {
      console.error('=== DEBUG: View ACLs error ===', error)
      message.error('获取权限列表失败')
      setUserAcls([])
    } finally {
      setViewAclLoading(false)
    }
  }

  // 删除单条 ACL
  const handleDeleteAcl = async (acl: ACL) => {
    Modal.confirm({
      title: '确认删除',
      content: `确定要删除该权限规则吗？`,
      onOk: async () => {
        try {
          await deleteACL(acl.id, selectedClusterId!)
          message.success('删除成功')
          // 刷新权限列表
          const principal = `User:${viewAclUsername}`
          const acls = await getACLs({ cluster_id: selectedClusterId!, principal })
          setUserAcls(acls)
        } catch (error: any) {
          console.error('=== DEBUG: Delete ACL error ===', error)
          const errorMsg = error?.response?.data?.error || error?.message || '删除失败'
          message.error(errorMsg)
        }
      },
    })
  }

  // 创建 ACL 权限
  const handleCreateAcl = async (values: any) => {
    if (!selectedClusterId) {
      message.warning('请先选择集群')
      return
    }
    try {
      const operations = Array.isArray(values.operation) ? values.operation : [values.operation]
      
      for (const op of operations) {
        const payload = {
          cluster_id: selectedClusterId,
          resource_type: values.resource_type,
          resource_name: values.resource_name,
          principal: values.principal,
          operation: op,
          permission_type: values.permission,
        }
        console.log('=== DEBUG: Create ACL payload ===', payload)
        await createACL(payload)
      }
      
      message.success(`成功设置 ${operations.length} 条权限规则`)
      setAclModalVisible(false)
      aclForm.resetFields()
    } catch (error: any) {
      console.error('=== DEBUG: Create ACL error ===', error)
      const errorMsg = error?.response?.data?.error || error?.message || '权限设置失败'
      message.error(errorMsg)
    }
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h1>SCRAM 用户管理</h1>
        <Space>
          <Select
            placeholder="选择集群"
            value={selectedClusterId}
            onChange={(value) => setSelectedClusterId(value)}
            style={{ width: 200 }}
          >
            {clusters.map(c => (
              <Select.Option key={c.cluster_id} value={c.cluster_id}>{c.cluster_name}</Select.Option>
            ))}
          </Select>
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
            onClick={handleOpenCreateUser}
            disabled={!selectedClusterId}
          >
            创建用户
          </Button>
        </Space>
      </div>
      <Card>
        <Table
          columns={columns}
          dataSource={users}
          rowKey="user_id"
          loading={loading}
          pagination={{ pageSize: 20 }}
          locale={{ emptyText: selectedClusterId ? '暂无用户数据' : '请先选择集群' }}
        />
      </Card>

      {/* 创建用户弹窗 */}
      <Modal
        title="创建 SCRAM 用户"
        open={createUserVisible}
        onCancel={() => {
          setCreateUserVisible(false)
          createUserForm.resetFields()
        }}
        onOk={() => createUserForm.submit()}
      >
        <Form form={createUserForm} layout="vertical" onFinish={handleCreateUser}>
          <Form.Item name="cluster_id" label="集群" rules={[{ required: true }]}>
            <Select placeholder="选择集群" disabled>
              {clusters.map(c => (
                <Select.Option key={c.cluster_id} value={c.cluster_id}>{c.cluster_name}</Select.Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item name="username" label="用户名" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input placeholder="请输入用户名" />
          </Form.Item>
          <Form.Item name="password" label="密码" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password placeholder="请输入密码" />
          </Form.Item>
          <Form.Item name="mechanism" label="认证机制" rules={[{ required: true, message: '请选择认证机制' }]}>
            <Select placeholder="选择认证机制">
              <Select.Option value="SCRAM-SHA-256">SCRAM-SHA-256</Select.Option>
              <Select.Option value="SCRAM-SHA-512">SCRAM-SHA-512</Select.Option>
            </Select>
          </Form.Item>
        </Form>
      </Modal>

      {/* 查看权限弹窗 */}
      <Modal
        title={`用户权限 - ${viewAclUsername}`}
        open={viewAclVisible}
        onCancel={() => {
          setViewAclVisible(false)
          setUserAcls([])
        }}
        footer={[
          <Button key="close" onClick={() => setViewAclVisible(false)}>
            关闭
          </Button>,
        ]}
        width={800}
      >
        <Table
          dataSource={userAcls}
          rowKey="id"
          loading={viewAclLoading}
          size="small"
          pagination={false}
          locale={{ emptyText: '该用户暂无权限规则' }}
          columns={[
            { title: '资源类型', dataIndex: 'resource_type', key: 'resource_type', width: 100 },
            { title: '资源名称', dataIndex: 'resource_name', key: 'resource_name' },
            { title: '操作', dataIndex: 'operation', key: 'operation', width: 100 },
            { 
              title: '权限', 
              dataIndex: 'permission', 
              key: 'permission', 
              width: 80,
              render: (permission: string) => (
                <Tag color={permission === 'Allow' ? 'green' : 'red'}>
                  {permission}
                </Tag>
              )
            },
            { title: 'Host', dataIndex: 'host', key: 'host', width: 80 },
            {
              title: '操作',
              key: 'action',
              width: 80,
              render: (_: any, record: ACL) => (
                <Button 
                  type="link" 
                  danger 
                  size="small"
                  onClick={() => handleDeleteAcl(record)}
                >
                  删除
                </Button>
              ),
            },
          ]}
        />
      </Modal>

      {/* 权限设置弹窗 */}
      <Modal
        title={`权限设置 - ${selectedUsername}`}
        open={aclModalVisible}
        onCancel={() => {
          setAclModalVisible(false)
          aclForm.resetFields()
        }}
        onOk={() => aclForm.submit()}
        width={600}
      >
        <Form form={aclForm} layout="vertical" onFinish={handleCreateAcl}>
          <Form.Item name="principal" label="Principal">
            <Input disabled />
          </Form.Item>
          <Form.Item name="resource_type" label="资源类型" rules={[{ required: true, message: '请选择资源类型' }]}>
            <Select placeholder="选择资源类型">
              <Select.Option value="Topic">Topic</Select.Option>
              <Select.Option value="Group">Consumer Group</Select.Option>
              <Select.Option value="Cluster">Cluster</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="resource_name" label="资源名称" rules={[{ required: true, message: '请输入资源名称' }]}>
            <Input placeholder="资源名称（如 test-topic 或 * 表示所有）" />
          </Form.Item>
          <Form.Item name="operation" label="操作" rules={[{ required: true, message: '请选择操作' }]}>
            <Select placeholder="选择操作" mode="multiple">
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
              <Select.Option value="Allow">Allow（允许）</Select.Option>
              <Select.Option value="Deny">Deny（拒绝）</Select.Option>
            </Select>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default ACLList
