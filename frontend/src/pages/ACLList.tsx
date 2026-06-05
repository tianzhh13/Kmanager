import { useState, useEffect } from 'react'
import { Button, Modal, Form, Select, Input, message } from 'antd'
import { PlusOutlined, DeleteOutlined, SyncOutlined, EyeOutlined, KeyOutlined } from '@ant-design/icons'
import { scramUserService, ScramUser } from '../services/scramUser'
import { clusterAPI } from '../services/cluster'
import { createACL, getUserACLsFromKafka, deleteACLFromKafka, UserACLInfo } from '../services/acl'
import { StatCard, LabelTag, SearchBar, AvatarInitials } from '../components/bento'

interface Cluster {
  cluster_id: number
  cluster_name: string
  auth_type: string
  sasl_mechanism?: string
}

const RESOURCE_OPERATIONS: Record<string, { value: string; label: string }[]> = {
  Topic: [
    { value: 'Read', label: 'Read - 消费消息' },
    { value: 'Write', label: 'Write - 生产消息' },
    { value: 'Create', label: 'Create - 创建（部分场景）' },
    { value: 'Delete', label: 'Delete - 删除 Topic' },
    { value: 'Alter', label: 'Alter - 修改配置' },
    { value: 'Describe', label: 'Describe - 查看元数据' },
    { value: 'DescribeConfigs', label: 'DescribeConfigs - 查看配置' },
    { value: 'AlterConfigs', label: 'AlterConfigs - 修改配置' },
  ],
  Group: [
    { value: 'Read', label: 'Read - 加入消费组/提交偏移量' },
    { value: 'Delete', label: 'Delete - 删除消费组' },
    { value: 'Describe', label: 'Describe - 查看消费组信息' },
    { value: 'Alter', label: 'Alter - 修改消费组状态' },
  ],
  Cluster: [
    { value: 'Create', label: 'Create - 创建 Topic' },
    { value: 'Delete', label: 'Delete - 删除 Topic' },
    { value: 'Alter', label: 'Alter - 修改集群配置' },
    { value: 'Describe', label: 'Describe - 查看集群信息' },
    { value: 'DescribeConfigs', label: 'DescribeConfigs - 查看配置' },
    { value: 'AlterConfigs', label: 'AlterConfigs - 修改配置' },
    { value: 'ClusterAction', label: 'ClusterAction - 集群操作（Leader选举等）' },
    { value: 'IdempotentWrite', label: 'IdempotentWrite - 幂等写入' },
  ],
}

const AVATAR_COLORS = ['#f97316', '#3b82f6', '#10b981', '#8b5cf6', '#ec4899', '#ef4444', '#f59e0b']

const ACLList: React.FC = () => {
  const [loading, setLoading] = useState(false)
  const [users, setUsers] = useState<ScramUser[]>([])
  const [clusters, setClusters] = useState<Cluster[]>([])
  const [selectedClusterId, setSelectedClusterId] = useState<number | null>(null)
  const [syncing, setSyncing] = useState(false)

  const [createUserVisible, setCreateUserVisible] = useState(false)
  const [createUserForm] = Form.useForm()

  const [aclModalVisible, setAclModalVisible] = useState(false)
  const [aclForm] = Form.useForm()
  const [selectedUsername, setSelectedUsername] = useState<string>('')
  const [selectedResourceType, setSelectedResourceType] = useState<string>('')

  const [viewAclVisible, setViewAclVisible] = useState(false)
  const [viewAclUsername, setViewAclUsername] = useState<string>('')
  const [userAcls, setUserAcls] = useState<UserACLInfo[]>([])
  const [viewAclLoading, setViewAclLoading] = useState(false)

  const [searchText, setSearchText] = useState('')
  const [expandedUser, setExpandedUser] = useState<string | null>(null)
  const [expandedAcls, setExpandedAcls] = useState<UserACLInfo[]>([])
  const [expandedAclsLoading, setExpandedAclsLoading] = useState(false)

  useEffect(() => {
    const loadClusters = async () => {
      try {
        const res = await clusterAPI.list(1, 100)
        const clusterList = res.data || []
        const scramClusters = clusterList.filter((c: Cluster) =>
          c.sasl_mechanism === 'SCRAM-SHA-256' || c.sasl_mechanism === 'SCRAM-SHA-512'
        )
        setClusters(scramClusters)
        if (scramClusters.length > 0) {
          setSelectedClusterId(scramClusters[0].cluster_id)
        }
      } catch (err) {
        console.error('Failed to load clusters:', err)
      }
    }
    loadClusters()
  }, [])

  useEffect(() => {
    if (selectedClusterId) {
      fetchUsers()
    }
  }, [selectedClusterId])

  const fetchUsers = async () => {
    if (!selectedClusterId) return
    setLoading(true)
    try {
      const res = await scramUserService.list(selectedClusterId)
      setUsers(res?.data || [])
    } catch (error: any) {
      message.error('获取用户列表失败')
    } finally {
      setLoading(false)
    }
  }

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

  const handleCreateUser = async (values: any) => {
    try {
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
      const errorMsg = error?.response?.data?.error || error?.message || '创建用户失败'
      message.error(errorMsg)
    }
  }

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
          const errorMsg = error?.response?.data?.error || error?.message || '删除失败'
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
      await scramUserService.sync(selectedClusterId)
      message.success('同步成功')
      fetchUsers()
    } catch (error: any) {
      const errorMsg = error?.response?.data?.error || error?.message || '同步失败'
      message.error(errorMsg)
    } finally {
      setSyncing(false)
    }
  }

  const handleOpenAclModal = (username: string) => {
    if (!selectedClusterId) {
      message.warning('请先选择集群')
      return
    }
    setSelectedUsername(username)
    setSelectedResourceType('')
    aclForm.setFieldsValue({
      cluster_id: selectedClusterId,
      principal: `User:${username}`,
    })
    setAclModalVisible(true)
  }

  const handleResourceTypeChange = (value: string) => {
    setSelectedResourceType(value)
    aclForm.setFieldsValue({ operation: undefined })
  }

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
      const acls = await getUserACLsFromKafka(selectedClusterId!, principal)
      setUserAcls(acls)
    } catch (error: any) {
      message.error('获取权限列表失败')
      setUserAcls([])
    } finally {
      setViewAclLoading(false)
    }
  }

  const handleToggleExpand = async (username: string) => {
    if (expandedUser === username) {
      setExpandedUser(null)
      setExpandedAcls([])
      return
    }
    setExpandedUser(username)
    setExpandedAclsLoading(true)
    setExpandedAcls([])
    try {
      const principal = `User:${username}`
      const acls = await getUserACLsFromKafka(selectedClusterId!, principal)
      setExpandedAcls(acls)
    } catch {
      // inline expand silently fails
    } finally {
      setExpandedAclsLoading(false)
    }
  }

  const handleDeleteAcl = async (acl: UserACLInfo) => {
    Modal.confirm({
      title: '确认删除',
      content: '确定要删除该权限规则吗？',
      onOk: async () => {
        try {
          await deleteACLFromKafka(selectedClusterId!, {
            resource_type: acl.resource_type,
            resource_name: acl.resource_name,
            resource_pattern: acl.resource_pattern || 'LITERAL',
            principal: acl.principal,
            host: acl.host || '*',
            operation: acl.operation,
            permission_type: acl.permission_type,
          })
          message.success('删除成功')
          const principal = `User:${viewAclUsername}`
          const acls = await getUserACLsFromKafka(selectedClusterId!, principal)
          setUserAcls(acls)
        } catch (error: any) {
          const errorMsg = error?.response?.data?.error || error?.message || '删除失败'
          message.error(errorMsg)
        }
      },
    })
  }

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
        await createACL(payload)
      }
      message.success(`成功设置 ${operations.length} 条权限规则`)
      setAclModalVisible(false)
      aclForm.resetFields()
    } catch (error: any) {
      const errorMsg = error?.response?.data?.error || error?.message || '权限设置失败'
      message.error(errorMsg)
    }
  }

  const scram256Count = users.filter(u => u.mechanism === 'SCRAM-SHA-256').length
  const scram512Count = users.filter(u => u.mechanism === 'SCRAM-SHA-512').length

  const filteredUsers = users.filter(u =>
    !searchText || u.username.toLowerCase().includes(searchText.toLowerCase())
  )

  const gridCols = '2fr 1fr 1.2fr 1.2fr 240px'

  const getResourceTypeColor = (type: string): 'orange' | 'blue' | 'purple' => {
    if (type === 'Topic') return 'orange'
    if (type === 'Group') return 'blue'
    return 'purple'
  }

  return (
    <div>
      {/* Header */}
      <div className="page-header">
        <div className="page-header-row">
          <div>
            <h1>SCRAM 用户管理</h1>
            <div className="page-accent-line" />
          </div>
          <div style={{ display: 'flex', gap: 10, alignItems: 'center' }}>
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
            <Button icon={<SyncOutlined spin={syncing} />} onClick={handleSync} loading={syncing} disabled={!selectedClusterId}>
              同步
            </Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={handleOpenCreateUser} disabled={!selectedClusterId}>
              创建用户
            </Button>
          </div>
        </div>
      </div>

      {/* Stat cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 16, marginBottom: 20 }}>
        <StatCard label="USER TOTAL" value={users.length} />
        <StatCard label="SCRAM-SHA-256" value={scram256Count} />
        <StatCard label="SCRAM-SHA-512" value={scram512Count} />
      </div>

      {clusters.length === 0 && (
        <div style={{ textAlign: 'center', padding: '40px 0', color: 'var(--text-3)' }}>
          暂无支持 SCRAM 用户管理的集群<br />
          <span style={{ fontSize: 12 }}>仅支持 SASL 机制为 SCRAM-SHA-256 或 SCRAM-SHA-512 的集群</span>
        </div>
      )}

      {clusters.length > 0 && (
        <>
          {/* Search */}
          <div style={{ display: 'flex', gap: 12, marginBottom: 20 }}>
            <SearchBar value={searchText} onChange={setSearchText} placeholder="搜索用户名..." />
          </div>

          {/* Table header */}
          <div className="bento-table-header" style={{ gridTemplateColumns: gridCols }}>
            <div>Username</div>
            <div style={{ textAlign: 'center' }}>Mechanism</div>
            <div style={{ textAlign: 'center' }}>Last Sync</div>
            <div style={{ textAlign: 'center' }}>Created</div>
            <div style={{ textAlign: 'right' }}>Actions</div>
          </div>

          {/* Table body */}
          <div className="bento-table-body">
            {loading && (
              <div style={{ textAlign: 'center', padding: 48, color: 'var(--text-3)' }}>加载中...</div>
            )}
            {!loading && filteredUsers.map((user, idx) => (
              <div key={user.user_id} style={{ borderBottom: '1px solid var(--border)' }}>
                <div
                  className="bento-table-row"
                  style={{ gridTemplateColumns: gridCols, cursor: 'pointer' }}
                  onClick={() => handleToggleExpand(user.username)}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                    <AvatarInitials name={user.username} color={AVATAR_COLORS[idx % AVATAR_COLORS.length]} />
                    <span style={{ fontWeight: 700, fontSize: 14, fontFamily: 'var(--font-mono)' }}>{user.username}</span>
                  </div>
                  <div style={{ textAlign: 'center' }}>
                    <LabelTag text={user.mechanism} color={user.mechanism === 'SCRAM-SHA-256' ? 'purple' : 'blue'} />
                  </div>
                  <div style={{ textAlign: 'center', fontSize: 12, color: 'var(--text-3)' }}>{user.last_sync_at ? user.last_sync_at.replace('T', ' ').replace(/\+08:00$/, '').replace(/\+00:00$/, '') : '-'}</div>
                  <div style={{ textAlign: 'center', fontSize: 12, color: 'var(--text-3)' }}>{user.created_at ? user.created_at.replace('T', ' ').replace(/\+08:00$/, '').replace(/\+00:00$/, '') : '-'}</div>
                  <div style={{ textAlign: 'right', display: 'flex', gap: 6, justifyContent: 'flex-end' }}>
                    <button className="bento-action-btn" onClick={(e) => { e.stopPropagation(); handleViewAcls(user.username) }}>
                      <EyeOutlined /> ACL
                    </button>
                    <button className="bento-action-btn bento-action-btn--primary" onClick={(e) => { e.stopPropagation(); handleOpenAclModal(user.username) }}>
                      <KeyOutlined /> + Rule
                    </button>
                    <button className="bento-action-btn bento-action-btn--danger" onClick={(e) => { e.stopPropagation(); handleDeleteUser(user.username) }}>
                      <DeleteOutlined /> 删除
                    </button>
                  </div>
                </div>

                {/* Inline ACL expand */}
                {expandedUser === user.username && (
                  <div style={{ padding: '0 24px 16px' }}>
                    <div style={{ background: 'var(--surface)', border: '1px solid var(--border)', borderRadius: 12, padding: 16 }}>
                      <div style={{ fontSize: 11, fontWeight: 700, color: 'var(--text-3)', textTransform: 'uppercase', letterSpacing: 1, marginBottom: 12 }}>
                        ACL Rules for {user.username}
                      </div>
                      {expandedAclsLoading && <div style={{ fontSize: 12, color: 'var(--text-3)' }}>加载中...</div>}
                      {!expandedAclsLoading && expandedAcls.length === 0 && (
                        <div style={{ fontSize: 12, color: 'var(--text-3)' }}>暂无权限规则</div>
                      )}
                      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                        {expandedAcls.map((acl, i) => (
                          <div
                            key={i}
                            style={{
                              display: 'flex', alignItems: 'center', gap: 10, padding: '8px 12px',
                              background: 'var(--card)', borderRadius: 8,
                              border: acl.permission_type === 'Deny' ? '1px solid rgba(239,68,68,0.15)' : 'none'
                            }}
                          >
                            <LabelTag text={acl.resource_type} color={getResourceTypeColor(acl.resource_type)} />
                            <span style={{ fontSize: 12, fontWeight: 600, fontFamily: 'var(--font-mono)', flex: 1 }}>{acl.resource_name}</span>
                            <span style={{ fontSize: 12, color: 'var(--text-2)', fontWeight: 600 }}>{acl.operation}</span>
                            <LabelTag text={acl.permission_type} color={acl.permission_type === 'Allow' ? 'green' : 'red'} />
                            <span style={{ fontSize: 11, color: 'var(--text-3)', fontFamily: 'var(--font-mono)' }}>{acl.host}</span>
                            <button className="bento-action-btn bento-action-btn--danger" style={{ fontSize: 10, padding: '2px 8px' }} onClick={() => handleDeleteAcl(acl)}>
                              删除
                            </button>
                          </div>
                        ))}
                      </div>
                    </div>
                  </div>
                )}
              </div>
            ))}
            {!loading && filteredUsers.length === 0 && (
              <div style={{ textAlign: 'center', padding: 48, color: 'var(--text-3)' }}>
                {selectedClusterId ? '暂无用户数据' : '请先选择集群'}
              </div>
            )}
          </div>
        </>
      )}

      {/* Create User Modal */}
      <Modal
        title="创建 SCRAM 用户"
        open={createUserVisible}
        onCancel={() => { setCreateUserVisible(false); createUserForm.resetFields() }}
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

      {/* View ACL Modal */}
      <Modal
        title={`用户权限 - ${viewAclUsername}`}
        open={viewAclVisible}
        onCancel={() => { setViewAclVisible(false); setUserAcls([]) }}
        footer={[
          <Button key="close" onClick={() => setViewAclVisible(false)}>关闭</Button>,
        ]}
        width={800}
      >
        <div className="bento-table-header" style={{ gridTemplateColumns: '1fr 1.5fr 1fr 80px 80px 80px' }}>
          <div>资源类型</div>
          <div>资源名称</div>
          <div>操作</div>
          <div>权限</div>
          <div>Host</div>
          <div style={{ textAlign: 'right' }}>操作</div>
        </div>
        <div className="bento-table-body">
          {viewAclLoading && <div style={{ textAlign: 'center', padding: 24, color: 'var(--text-3)' }}>加载中...</div>}
          {!viewAclLoading && userAcls.length === 0 && <div style={{ textAlign: 'center', padding: 24, color: 'var(--text-3)' }}>该用户暂无权限规则</div>}
          {!viewAclLoading && userAcls.map((acl, index) => (
            <div key={`${acl.resource_type}-${acl.resource_name}-${acl.operation}-${index}`} className="bento-table-row" style={{ gridTemplateColumns: '1fr 1.5fr 1fr 80px 80px 80px' }}>
              <LabelTag text={acl.resource_type} color={getResourceTypeColor(acl.resource_type)} />
              <span style={{ fontSize: 12, fontFamily: 'var(--font-mono)' }}>{acl.resource_name}</span>
              <span style={{ fontSize: 12 }}>{acl.operation}</span>
              <LabelTag text={acl.permission_type} color={acl.permission_type === 'Allow' ? 'green' : 'red'} />
              <span style={{ fontSize: 11, fontFamily: 'var(--font-mono)', color: 'var(--text-3)' }}>{acl.host}</span>
              <div style={{ textAlign: 'right' }}>
                <button className="bento-action-btn bento-action-btn--danger" style={{ fontSize: 10 }} onClick={() => handleDeleteAcl(acl)}>删除</button>
              </div>
            </div>
          ))}
        </div>
      </Modal>

      {/* Create ACL Modal */}
      <Modal
        title={`权限设置 - ${selectedUsername}`}
        open={aclModalVisible}
        onCancel={() => { setAclModalVisible(false); aclForm.resetFields() }}
        onOk={() => aclForm.submit()}
        width={600}
      >
        <Form form={aclForm} layout="vertical" onFinish={handleCreateAcl}>
          <Form.Item name="principal" label="Principal">
            <Input disabled />
          </Form.Item>
          <Form.Item name="resource_type" label="资源类型" rules={[{ required: true, message: '请选择资源类型' }]}>
            <Select placeholder="选择资源类型" onChange={handleResourceTypeChange}>
              <Select.Option value="Topic">Topic（主题）</Select.Option>
              <Select.Option value="Group">Group（消费组）</Select.Option>
              <Select.Option value="Cluster">Cluster（集群）</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="resource_name" label="资源名称" rules={[{ required: true, message: '请输入资源名称' }]}>
            <Input placeholder={selectedResourceType === 'Cluster' ? '集群资源通常填 kafka-cluster 或 *' : '资源名称（如 test-topic 或 * 表示所有）'} />
          </Form.Item>
          <Form.Item name="operation" label="操作" rules={[{ required: true, message: '请选择操作' }]}>
            <Select
              placeholder={selectedResourceType ? '选择操作' : '请先选择资源类型'}
              mode="multiple"
              disabled={!selectedResourceType}
            >
              {(RESOURCE_OPERATIONS[selectedResourceType] || []).map(op => (
                <Select.Option key={op.value} value={op.value}>{op.label}</Select.Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item name="permission" label="权限类型" rules={[{ required: true, message: '请选择权限类型' }]}>
            <Select placeholder="选择权限类型">
              <Select.Option value="Allow">Allow（允许）</Select.Option>
              <Select.Option value="Deny">Deny（拒绝）</Select.Option>
            </Select>
          </Form.Item>
          {selectedResourceType && (
            <div style={{ padding: '8px 12px', background: 'var(--color-info-bg)', borderRadius: 'var(--radius-md)', fontSize: 12, color: 'var(--text-secondary)' }}>
              已根据资源类型 <strong>{selectedResourceType}</strong> 过滤可用操作
            </div>
          )}
        </Form>
      </Modal>
    </div>
  )
}

export default ACLList
