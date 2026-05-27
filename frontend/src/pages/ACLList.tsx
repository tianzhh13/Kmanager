import { useState, useEffect } from 'react'
import { Table, Button, Space, Card, Modal, Form, Select, Input, message, Tag, Row, Col, Statistic } from 'antd'
import { PlusOutlined, DeleteOutlined, SyncOutlined, KeyOutlined, EyeOutlined, LockOutlined, TeamOutlined } from '@ant-design/icons'
import { scramUserService, ScramUser } from '../services/scramUser'
import { clusterAPI } from '../services/cluster'
import { createACL, getUserACLsFromKafka, deleteACLFromKafka, UserACLInfo } from '../services/acl'

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
  
  const columns = [
    { title: '用户名', dataIndex: 'username', key: 'username',
      render: (name: string) => <strong style={{ color: 'var(--text-heading)' }}>{name}</strong>,
    },
    { title: '认证机制', dataIndex: 'mechanism', key: 'mechanism', width: 150,
      render: (v: string) => <Tag color="processing">{v}</Tag>,
    },
    { title: '同步状态', dataIndex: 'sync_status', key: 'sync_status', width: 100 },
    { title: '最后同步时间', dataIndex: 'last_sync_at', key: 'last_sync_at', width: 180,
      render: (text: string) => <span style={{ color: 'var(--text-tertiary)', fontSize: 13 }}>{text || '-'}</span>,
    },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 180,
      render: (text: string) => <span style={{ color: 'var(--text-tertiary)', fontSize: 13 }}>{text || '-'}</span>,
    },
    {
      title: '操作',
      key: 'action',
      width: 250,
      render: (_: any, record: ScramUser) => (
        <Space>
          <Button type="link" icon={<EyeOutlined />} onClick={() => handleViewAcls(record.username)}>
            查看权限
          </Button>
          <Button type="link" icon={<KeyOutlined />} onClick={() => handleOpenAclModal(record.username)}>
            权限设置
          </Button>
          <Button type="link" danger icon={<DeleteOutlined />} onClick={() => handleDeleteUser(record.username)}>
            删除
          </Button>
        </Space>
      ),
    },
  ]

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

  return (
    <div>
      <div className="page-header">
        <div className="page-header-row">
          <div>
            <h1>SCRAM 用户管理</h1>
            <div className="page-accent-line" />
          </div>
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
            <Button icon={<SyncOutlined spin={syncing} />} onClick={handleSync} loading={syncing} disabled={!selectedClusterId}>
              同步
            </Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={handleOpenCreateUser} disabled={!selectedClusterId}>
              创建用户
            </Button>
          </Space>
        </div>
      </div>

      {/* 统计卡片 */}
      <Row gutter={[12, 12]} style={{ marginBottom: 20 }}>
        <Col xs={12} sm={8}>
          <Card size="small" className="stat-card">
            <Statistic title="用户总数" value={users.length} prefix={<TeamOutlined />}
              valueStyle={{ fontWeight: 700, fontSize: 22, color: 'var(--color-info)' }} />
          </Card>
        </Col>
        <Col xs={12} sm={8}>
          <Card size="small" className="stat-card">
            <Statistic title="SCRAM-256" value={users.filter(u => u.mechanism === 'SCRAM-SHA-256').length}
              prefix={<LockOutlined />}
              valueStyle={{ fontWeight: 700, fontSize: 22, color: 'var(--color-success)' }} />
          </Card>
        </Col>
        <Col xs={12} sm={8}>
          <Card size="small" className="stat-card">
            <Statistic title="SCRAM-512" value={users.filter(u => u.mechanism === 'SCRAM-SHA-512').length}
              prefix={<LockOutlined />}
              valueStyle={{ fontWeight: 700, fontSize: 22, color: 'var(--brand-accent)' }} />
          </Card>
        </Col>
      </Row>
      
      {clusters.length === 0 && (
        <Card>
          <div style={{ textAlign: 'center', padding: '40px 0', color: 'var(--text-tertiary)' }}>
            暂无支持 SCRAM 用户管理的集群<br/>
            <span style={{ fontSize: 12 }}>仅支持 SASL 机制为 SCRAM-SHA-256 或 SCRAM-SHA-512 的集群</span>
          </div>
        </Card>
      )}
      
      {clusters.length > 0 && (
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
      )}

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
          rowKey={(record, index) => `${record.resource_type}-${record.resource_name}-${record.operation}-${index}`}
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
              dataIndex: 'permission_type', 
              key: 'permission_type', 
              width: 80,
              render: (permission_type: string) => (
                <Tag color={permission_type === 'Allow' ? 'success' : 'error'}>
                  {permission_type}
                </Tag>
              )
            },
            { title: 'Host', dataIndex: 'host', key: 'host', width: 80 },
            {
              title: '操作',
              key: 'action',
              width: 80,
              render: (_: any, record: UserACLInfo) => (
                <Button type="link" danger size="small" onClick={() => handleDeleteAcl(record)}>
                  删除
                </Button>
              ),
            },
          ]}
        />
      </Modal>

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
