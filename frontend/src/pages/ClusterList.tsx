import { useEffect, useState } from 'react'
import { Table, Button, Space, Modal, Form, Input, Select, message, Tag, Upload, Row, Col, Statistic, Card } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, UploadOutlined, ClusterOutlined, CheckCircleOutlined, LockOutlined, SafetyCertificateOutlined } from '@ant-design/icons'
import { useAppDispatch, useAppSelector } from '../store/hooks'
import { fetchClusters, createCluster, updateCluster, deleteCluster, testConnectionForCreate } from '../store/slices/clusterSlice'
import { Cluster } from '../services/cluster'
import { clusterAPI } from '../services/cluster'

const ClusterList: React.FC = () => {
  const dispatch = useAppDispatch()
  const { clusters, total, loading } = useAppSelector((state) => state.clusters)
  const { user } = useAppSelector((state) => state.auth)
  const isSuperAdmin = user?.role === 'super_admin'
  const [isModalVisible, setIsModalVisible] = useState(false)
  const [isEditModal, setIsEditModal] = useState(false)
  const [editingCluster, setEditingCluster] = useState<Cluster | null>(null)
  const [form] = Form.useForm()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [authType, setAuthType] = useState<string>('none')
  const [testingConnection, setTestingConnection] = useState(false)
  const [keytabFile, setKeytabFile] = useState<File | null>(null)
  const [keytabTempId, setKeytabTempId] = useState<string>('')
  const [uploadingKeytab, setUploadingKeytab] = useState(false)

  useEffect(() => {
    dispatch(fetchClusters({ page, pageSize }))
  }, [dispatch, page, pageSize])

  const buildAuthConfig = (values: any): Record<string, any> | undefined => {
    const authConfig: Record<string, any> = {}
    if (values.auth_type === 'scram') {
      authConfig.username = values.scram_username
      authConfig.password = values.scram_password
      authConfig.mechanism = values.scram_mechanism || 'PLAIN'
    } else if (values.auth_type === 'kerberos') {
      authConfig.principal = values.kerberos_principal
      authConfig.service_name = values.kerberos_service_name || 'kafka'
      authConfig.krb5_content = values.krb5_content
      if (keytabTempId) {
        authConfig.keytab_temp_id = keytabTempId
      }
    }
    return Object.keys(authConfig).length > 0 ? authConfig : undefined
  }

  const handleKeytabUpload = async (file: File) => {
    setUploadingKeytab(true)
    try {
      const tempId = await clusterAPI.uploadKeytab(file)
      setKeytabTempId(tempId)
      setKeytabFile(file)
      message.success('Keytab 文件上传成功')
    } catch (error: any) {
      message.error(error || 'Keytab 上传失败')
    } finally {
      setUploadingKeytab(false)
    }
    return false
  }

  const handleCreate = async () => {
    try {
      const values = await form.validateFields()
      
      if (values.auth_type === 'kerberos') {
        if (!keytabTempId) {
          message.error('请先上传 Keytab 文件')
          return
        }
        if (!values.krb5_content) {
          message.error('请填写 krb5.conf 配置内容')
          return
        }
      }
      
      const authConfig = buildAuthConfig(values)
      const testData = {
        cluster_name: values.cluster_name,
        bootstrap_servers: values.bootstrap_servers,
        auth_type: values.auth_type,
        auth_config: authConfig,
      }
      
      setTestingConnection(true)
      await dispatch(testConnectionForCreate(testData)).unwrap()
      setTestingConnection(false)
      
      const clusterData = {
        cluster_name: values.cluster_name,
        bootstrap_servers: values.bootstrap_servers,
        auth_type: values.auth_type,
        auth_config: authConfig,
        jmx_exporter_urls: values.jmx_exporter_urls,
        description: values.description,
      }
      
      await dispatch(createCluster(clusterData)).unwrap()
      message.success('创建成功')
      setIsModalVisible(false)
      form.resetFields()
      setAuthType('none')
      setKeytabFile(null)
      setKeytabTempId('')
      dispatch(fetchClusters({ page, pageSize }))
    } catch (error: any) {
      message.error(error || '创建失败')
    } finally {
      setTestingConnection(false)
    }
  }

  const handleEdit = (record: Cluster) => {
    setEditingCluster(record)
    setIsEditModal(true)
    form.setFieldsValue({
      cluster_name: record.cluster_name,
      jmx_exporter_urls: record.jmx_exporter_urls,
      description: record.description,
    })
  }

  const handleUpdate = async () => {
    if (!editingCluster) return
    try {
      const values = await form.validateFields()
      const updateData = {
        cluster_name: values.cluster_name,
        jmx_exporter_urls: values.jmx_exporter_urls,
        description: values.description,
      }
      await dispatch(updateCluster({ id: editingCluster.cluster_id, data: updateData })).unwrap()
      message.success('更新成功')
      setIsEditModal(false)
      setEditingCluster(null)
      form.resetFields()
      dispatch(fetchClusters({ page, pageSize }))
    } catch (error: any) {
      message.error(error || '更新失败')
    }
  }

  const handleDelete = (id: number) => {
    Modal.confirm({
      title: '确认删除',
      content: '确定要删除这个集群吗？此操作不可恢复。',
      okText: '确认',
      cancelText: '取消',
      onOk: async () => {
        try {
          await dispatch(deleteCluster(id)).unwrap()
          message.success('删除成功')
        } catch (error: any) {
          message.error(error || '删除失败')
        }
      },
    })
  }

  const columns = [
    { title: '集群名称', dataIndex: 'cluster_name', key: 'cluster_name',
      render: (name: string) => <strong style={{ color: 'var(--text-heading)' }}>{name}</strong>,
    },
    { title: 'Bootstrap Servers', dataIndex: 'bootstrap_servers', key: 'bootstrap_servers',
      render: (text: string) => <span className="text-mono" style={{ fontSize: 12 }}>{text}</span>,
    },
    { title: '认证类型', dataIndex: 'auth_type', key: 'auth_type', 
      render: (type: string) => (
        <Tag color={type === 'none' ? 'success' : type === 'plaintext' ? 'success' : type === 'scram' ? 'processing' : 'warning'}>
          {type ? type.toUpperCase() : 'NONE'}
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
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at',
      render: (text: string) => <span style={{ color: 'var(--text-tertiary)', fontSize: 13 }}>{text}</span>,
    },
    { title: '操作', key: 'action', width: 150,
      render: (_: any, record: Cluster) => (
        <Space>
          {isSuperAdmin && (
            <>
              <Button type="link" icon={<EditOutlined />} onClick={() => handleEdit(record)}>
                编辑
              </Button>
              <Button type="link" danger icon={<DeleteOutlined />} onClick={() => handleDelete(record.cluster_id)}>
                删除
              </Button>
            </>
          )}
        </Space>
      )
    },
  ]

  const renderForm = () => {
    if (isEditModal && editingCluster) {
      return (
        <Form form={form} layout="vertical">
          <Form.Item name="cluster_name" label="集群名称" rules={[{ required: true, message: '请输入集群名称' }]}>
            <Input placeholder="例如：生产环境集群" />
          </Form.Item>
          <Form.Item label="Bootstrap Servers">
            <Input value={editingCluster.bootstrap_servers} disabled />
          </Form.Item>
          <Form.Item label="认证类型">
            <Input value={editingCluster.auth_type?.toUpperCase()} disabled />
          </Form.Item>
          <Form.Item 
            name="jmx_exporter_urls" 
            label="JMX Exporter URLs"
            extra="多个 Broker 的 JMX Exporter 地址，逗号分隔"
          >
            <Input placeholder="例如：http://broker1:7071,http://broker2:7071,http://broker3:7071" />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} placeholder="集群用途说明" />
          </Form.Item>
        </Form>
      )
    }
    
    return (
      <Form form={form} layout="vertical">
        <Form.Item name="cluster_name" label="集群名称" rules={[{ required: true, message: '请输入集群名称' }]}>
          <Input placeholder="例如：生产环境集群" />
        </Form.Item>
        <Form.Item name="bootstrap_servers" label="Bootstrap Servers" rules={[{ required: true, message: '请输入 Bootstrap Servers' }]}>
          <Input placeholder="例如：kafka1:9092,kafka2:9092,kafka3:9092" />
        </Form.Item>
        <Form.Item name="auth_type" label="认证类型" rules={[{ required: true, message: '请选择认证类型' }]}>
          <Select onChange={(value) => setAuthType(value)}>
            <Select.Option value="plaintext">PLAINTEXT（无认证）</Select.Option>
            <Select.Option value="scram">SASL（用户名密码）</Select.Option>
            <Select.Option value="kerberos">Kerberos</Select.Option>
          </Select>
        </Form.Item>

      {authType === 'scram' && (
        <>
          <Form.Item name="scram_username" label="用户名" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input placeholder="Kafka 用户名" />
          </Form.Item>
          <Form.Item name="scram_password" label="密码" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password placeholder="Kafka 密码" />
          </Form.Item>
          <Form.Item 
            name="scram_mechanism" 
            label="SASL 机制" 
            initialValue="PLAIN"
            extra="PLAIN: 简单用户名密码；SCRAM-SHA-256/512: 带哈希的安全认证"
          >
            <Select>
              <Select.Option value="PLAIN">PLAIN</Select.Option>
              <Select.Option value="SCRAM-SHA-256">SCRAM-SHA-256</Select.Option>
              <Select.Option value="SCRAM-SHA-512">SCRAM-SHA-512</Select.Option>
            </Select>
          </Form.Item>
        </>
      )}

      {authType === 'kerberos' && (
        <>
          <Form.Item 
            name="kerberos_principal" 
            label="Principal" 
            rules={[{ required: true, message: '请输入 Principal' }]}
            extra="格式：user@REALM 或 user/hostname@REALM"
          >
            <Input placeholder="例如：kafka-client/node01@EXAMPLE.COM" />
          </Form.Item>
          <Form.Item 
            name="kerberos_service_name" 
            label="Service Name" 
            initialValue="kafka"
          >
            <Input placeholder="默认：kafka" />
          </Form.Item>
          <Form.Item 
            name="krb5_content" 
            label="krb5.conf 配置" 
            rules={[{ required: true, message: '请填写 krb5.conf 配置内容' }]}
            extra="请粘贴 krb5.conf 文件的完整内容"
          >
            <Input.TextArea 
              rows={10} 
              placeholder={`[libdefaults]
  default_realm = EXAMPLE.COM
  dns_lookup_realm = false
  dns_lookup_kdc = false

[realms]
  EXAMPLE.COM = {
    kdc = kdc.example.com
    admin_server = kdc.example.com
  }`}
            />
          </Form.Item>
          <Form.Item 
            label="Keytab 文件" 
            required
          >
            <Upload
              beforeUpload={handleKeytabUpload}
              accept=".keytab"
              maxCount={1}
              fileList={keytabFile ? [keytabFile as any] : []}
              onRemove={() => {
                setKeytabFile(null)
                setKeytabTempId('')
              }}
            >
              <Button icon={<UploadOutlined />} loading={uploadingKeytab}>
                {uploadingKeytab ? '上传中...' : '选择 Keytab 文件'}
              </Button>
            </Upload>
            {keytabTempId && <span style={{ marginLeft: 8, color: 'var(--color-success)', fontSize: 12 }}>已上传</span>}
          </Form.Item>
        </>
      )}

      <Form.Item 
        name="jmx_exporter_urls" 
        label="JMX Exporter URLs"
        extra="多个 Broker 的 JMX Exporter 地址，逗号分隔"
      >
        <Input placeholder="例如：http://broker1:7071,http://broker2:7071,http://broker3:7071" />
      </Form.Item>
      <Form.Item name="description" label="描述">
        <Input.TextArea rows={3} placeholder="集群用途说明" />
      </Form.Item>
    </Form>
    )
  }

  return (
    <div>
      <div className="page-header">
        <div className="page-header-row">
          <div>
            <h1>集群管理</h1>
            <div className="page-accent-line" />
          </div>
          {isSuperAdmin && (
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setIsModalVisible(true)}>
              创建集群
            </Button>
          )}
        </div>
      </div>

      {/* 统计卡片 */}
      <Row gutter={[12, 12]} style={{ marginBottom: 20 }}>
        <Col xs={12} sm={6}>
          <Card size="small" className="stat-card">
            <Statistic title="集群总数" value={total} prefix={<ClusterOutlined />}
              valueStyle={{ fontWeight: 700, fontSize: 22, color: 'var(--brand-primary)' }} />
          </Card>
        </Col>
        <Col xs={12} sm={6}>
          <Card size="small" className="stat-card">
            <Statistic title="活跃集群" value={clusters.filter(c => c.status === 'active').length}
              prefix={<CheckCircleOutlined />}
              valueStyle={{ fontWeight: 700, fontSize: 22, color: 'var(--color-success)' }} />
          </Card>
        </Col>
        <Col xs={12} sm={6}>
          <Card size="small" className="stat-card">
            <Statistic title="SCRAM 集群"
              value={clusters.filter(c => c.sasl_mechanism === 'SCRAM-SHA-256' || c.sasl_mechanism === 'SCRAM-SHA-512').length}
              prefix={<LockOutlined />}
              valueStyle={{ fontWeight: 700, fontSize: 22, color: 'var(--color-info)' }} />
          </Card>
        </Col>
        <Col xs={12} sm={6}>
          <Card size="small" className="stat-card">
            <Statistic title="JMX 已配置"
              value={clusters.filter(c => c.jmx_exporter_urls).length}
              prefix={<SafetyCertificateOutlined />}
              valueStyle={{ fontWeight: 700, fontSize: 22, color: 'var(--brand-accent)' }} />
          </Card>
        </Col>
      </Row>
      
      <Table
        columns={columns}
        dataSource={clusters}
        rowKey="cluster_id"
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
        onCancel={() => {
          if (keytabTempId) {
            clusterAPI.deleteTempKeytab(keytabTempId).catch(() => {})
          }
          setIsModalVisible(false)
          form.resetFields()
          setAuthType('none')
          setKeytabFile(null)
          setKeytabTempId('')
        }}
        confirmLoading={testingConnection}
        okText="创建"
        cancelText="取消"
        width={700}
      >
        {renderForm()}
        <div style={{ marginTop: 16, padding: '10px 12px', background: 'var(--color-info-bg)', borderRadius: 'var(--radius-md)', fontSize: 13, color: 'var(--text-secondary)' }}>
          创建前会自动测试 Kafka 集群连接，只有连接成功才能创建。
        </div>
      </Modal>

      <Modal
        title="编辑集群"
        open={isEditModal}
        onOk={handleUpdate}
        onCancel={() => { 
          setIsEditModal(false)
          setEditingCluster(null)
          form.resetFields()
          setAuthType('none')
          setKeytabFile(null)
          setKeytabTempId('')
        }}
        okText="更新"
        cancelText="取消"
        width={700}
      >
        {renderForm()}
        <div style={{ marginTop: 16, padding: '10px 12px', background: 'var(--color-warning-bg)', borderRadius: 'var(--radius-md)', fontSize: 13, color: 'var(--text-secondary)' }}>
          Bootstrap Servers 和认证配置创建后不可修改。
        </div>
      </Modal>
    </div>
  )
}

export default ClusterList
