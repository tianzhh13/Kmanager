import { useEffect, useState } from 'react'
import { Table, Button, Space, Modal, Form, Input, Select, message, Tag } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import { useAppDispatch, useAppSelector } from '../store/hooks'
import { fetchClusters, createCluster, updateCluster, deleteCluster, testConnectionForCreate } from '../store/slices/clusterSlice'
import { Cluster } from '../services/cluster'

const ClusterList: React.FC = () => {
  const dispatch = useAppDispatch()
  const { clusters, total, loading } = useAppSelector((state) => state.clusters)
  const [isModalVisible, setIsModalVisible] = useState(false)
  const [isEditModal, setIsEditModal] = useState(false)
  const [editingCluster, setEditingCluster] = useState<Cluster | null>(null)
  const [form] = Form.useForm()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [authType, setAuthType] = useState<string>('none')
  const [testingConnection, setTestingConnection] = useState(false)

  useEffect(() => {
    dispatch(fetchClusters({ page, pageSize }))
  }, [dispatch, page, pageSize])

  // 构建认证配置
  const buildAuthConfig = (values: any): Record<string, any> | undefined => {
    const authConfig: Record<string, any> = {}
    if (values.auth_type === 'scram') {
      authConfig.username = values.scram_username
      authConfig.password = values.scram_password
      authConfig.mechanism = values.scram_mechanism || 'SCRAM-SHA-256'
      authConfig.security_protocol = values.security_protocol || 'SASL_SSL'
    } else if (values.auth_type === 'kerberos') {
      authConfig.principal = values.kerberos_principal
      authConfig.keytab_path = values.kerberos_keytab_path
      authConfig.realm = values.kerberos_realm
      authConfig.service_name = values.kerberos_service_name || 'kafka'
    }
    return Object.keys(authConfig).length > 0 ? authConfig : undefined
  }

  // 创建集群
  const handleCreate = async () => {
    try {
      const values = await form.validateFields()
      
      // 先测试连接
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
      
      // 连接测试成功，创建集群
      const clusterData = {
        cluster_name: values.cluster_name,
        bootstrap_servers: values.bootstrap_servers,
        auth_type: values.auth_type,
        auth_config: authConfig,
        prometheus_url: values.prometheus_url,
        description: values.description,
      }
      
      await dispatch(createCluster(clusterData)).unwrap()
      message.success('创建成功')
      setIsModalVisible(false)
      form.resetFields()
      setAuthType('none')
      dispatch(fetchClusters({ page, pageSize }))
    } catch (error: any) {
      message.error(error || '创建失败')
    } finally {
      setTestingConnection(false)
    }
  }

  // 编辑集群
  const handleEdit = (record: Cluster) => {
    setEditingCluster(record)
    setIsEditModal(true)
    
    // 解析认证配置
    let authConfig: Record<string, any> = {}
    if (record.auth_config) {
      try {
        authConfig = typeof record.auth_config === 'string' 
          ? JSON.parse(record.auth_config) 
          : record.auth_config
      } catch (e) {
        console.error('Failed to parse auth config:', e)
      }
    }
    
    // 设置表单值
    const formValues: any = {
      cluster_name: record.cluster_name,
      bootstrap_servers: record.bootstrap_servers,
      auth_type: record.auth_type,
      prometheus_url: record.prometheus_url,
      description: record.description,
    }
    
    // 根据认证类型设置认证配置
    if (record.auth_type === 'scram' && authConfig.username) {
      formValues.scram_username = authConfig.username
      formValues.scram_password = authConfig.password
      formValues.scram_mechanism = authConfig.mechanism || 'SCRAM-SHA-256'
      formValues.security_protocol = authConfig.security_protocol || 'SASL_SSL'
    } else if (record.auth_type === 'kerberos' && authConfig.principal) {
      formValues.kerberos_principal = authConfig.principal
      formValues.kerberos_keytab_path = authConfig.keytab_path
      formValues.kerberos_realm = authConfig.realm
      formValues.kerberos_service_name = authConfig.service_name || 'kafka'
    }
    
    form.setFieldsValue(formValues)
    setAuthType(record.auth_type)
  }

  // 更新集群
  const handleUpdate = async () => {
    if (!editingCluster) return
    
    try {
      const values = await form.validateFields()
      
      // 先测试连接
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
      
      // 连接测试成功，更新集群
      const updateData = {
        cluster_name: values.cluster_name,
        bootstrap_servers: values.bootstrap_servers,
        auth_type: values.auth_type,
        auth_config: authConfig,
        prometheus_url: values.prometheus_url,
        description: values.description,
      }
      
      await dispatch(updateCluster({ id: editingCluster.cluster_id, data: updateData })).unwrap()
      message.success('更新成功')
      setIsEditModal(false)
      setEditingCluster(null)
      form.resetFields()
      setAuthType('none')
      dispatch(fetchClusters({ page, pageSize }))
    } catch (error: any) {
      message.error(error || '更新失败')
    } finally {
      setTestingConnection(false)
    }
  }

  // 删除集群
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
    { title: 'ID', dataIndex: 'cluster_id', key: 'cluster_id', width: 60 },
    { title: '集群名称', dataIndex: 'cluster_name', key: 'cluster_name' },
    { title: 'Bootstrap Servers', dataIndex: 'bootstrap_servers', key: 'bootstrap_servers' },
    { title: '认证类型', dataIndex: 'auth_type', key: 'auth_type', 
      render: (type: string) => (
        <Tag color={type === 'none' ? 'green' : type === 'plaintext' ? 'green' : type === 'scram' ? 'blue' : 'orange'}>
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
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at' },
    { title: '操作', key: 'action', width: 150,
      render: (_: any, record: Cluster) => (
        <Space>
          <Button 
            type="link" 
            icon={<EditOutlined />}
            onClick={() => handleEdit(record)}
          >
            编辑
          </Button>
          <Button 
            type="link" 
            danger 
            icon={<DeleteOutlined />}
            onClick={() => handleDelete(record.cluster_id)}
          >
            删除
          </Button>
        </Space>
      )
    },
  ]

  // 渲染表单
  const renderForm = () => (
    <Form form={form} layout="vertical">
      <Form.Item name="cluster_name" label="集群名称" rules={[{ required: true, message: '请输入集群名称' }]}>
        <Input placeholder="例如：生产环境集群" />
      </Form.Item>
      
      <Form.Item name="bootstrap_servers" label="Bootstrap Servers" rules={[{ required: true, message: '请输入 Bootstrap Servers' }]}>
        <Input placeholder="例如：kafka1:9092,kafka2:9092,kafka3:9092" />
      </Form.Item>
      
      <Form.Item name="auth_type" label="认证类型" rules={[{ required: true, message: '请选择认证类型' }]}>
        <Select onChange={(value) => setAuthType(value)}>
          <Select.Option value="none">无认证</Select.Option>
          <Select.Option value="plaintext">PLAINTEXT（无认证，明文传输）</Select.Option>
          <Select.Option value="scram">SCRAM（用户名密码）</Select.Option>
          <Select.Option value="kerberos">Kerberos</Select.Option>
        </Select>
      </Form.Item>

      {/* SCRAM 认证配置 */}
      {authType === 'scram' && (
        <>
          <Form.Item name="scram_username" label="用户名" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input placeholder="Kafka 用户名" />
          </Form.Item>
          <Form.Item name="scram_password" label="密码" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password placeholder="Kafka 密码" />
          </Form.Item>
          <Form.Item name="scram_mechanism" label="SCRAM 机制" initialValue="SCRAM-SHA-256">
            <Select>
              <Select.Option value="SCRAM-SHA-256">SCRAM-SHA-256</Select.Option>
              <Select.Option value="SCRAM-SHA-512">SCRAM-SHA-512</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item 
            name="security_protocol" 
            label="安全协议" 
            initialValue="SASL_SSL"
            extra="SASL_SSL: SASL over TLS; SASL_PLAINTEXT: SASL over 明文传输"
          >
            <Select>
              <Select.Option value="SASL_SSL">SASL_SSL（SASL over TLS）</Select.Option>
              <Select.Option value="SASL_PLAINTEXT">SASL_PLAINTEXT（SASL over 明文）</Select.Option>
            </Select>
          </Form.Item>
        </>
      )}

      {/* Kerberos 认证配置 */}
      {authType === 'kerberos' && (
        <>
          <Form.Item name="kerberos_principal" label="Principal" rules={[{ required: true, message: '请输入 Principal' }]}>
            <Input placeholder="例如：kafka-client@EXAMPLE.COM" />
          </Form.Item>
          <Form.Item name="kerberos_keytab_path" label="Keytab 文件路径" rules={[{ required: true, message: '请输入 Keytab 文件路径' }]}>
            <Input placeholder="例如：/etc/security/keytabs/kafka-client.keytab" />
          </Form.Item>
          <Form.Item name="kerberos_realm" label="Realm" rules={[{ required: true, message: '请输入 Realm' }]}>
            <Input placeholder="例如：EXAMPLE.COM" />
          </Form.Item>
          <Form.Item name="kerberos_service_name" label="Service Name" initialValue="kafka">
            <Input placeholder="默认：kafka" />
          </Form.Item>
        </>
      )}

      <Form.Item name="prometheus_url" label="Prometheus URL">
        <Input placeholder="例如：http://prometheus:9090" />
      </Form.Item>

      <Form.Item name="description" label="描述">
        <Input.TextArea rows={3} placeholder="集群用途说明" />
      </Form.Item>
    </Form>
  )

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
        rowKey="cluster_id"
        loading={loading}
        pagination={{
          current: page,
          pageSize,
          total,
          onChange: (p, ps) => { setPage(p); setPageSize(ps) },
        }}
      />

      {/* 创建集群对话框 */}
      <Modal
        title="创建集群"
        open={isModalVisible}
        onOk={handleCreate}
        onCancel={() => { setIsModalVisible(false); form.resetFields(); setAuthType('none') }}
        confirmLoading={testingConnection}
        okText="创建"
        cancelText="取消"
        width={700}
      >
        {renderForm()}
        <div style={{ marginTop: 16, color: '#666' }}>
          提示：创建前会自动测试 Kafka 集群连接，只有连接成功才能创建。
        </div>
      </Modal>

      {/* 编辑集群对话框 */}
      <Modal
        title="编辑集群"
        open={isEditModal}
        onOk={handleUpdate}
        onCancel={() => { setIsEditModal(false); setEditingCluster(null); form.resetFields(); setAuthType('none') }}
        confirmLoading={testingConnection}
        okText="更新"
        cancelText="取消"
        width={700}
      >
        {renderForm()}
        <div style={{ marginTop: 16, color: '#666' }}>
          提示：更新前会自动测试 Kafka 集群连接，只有连接成功才能更新。
        </div>
      </Modal>
    </div>
  )
}

export default ClusterList
