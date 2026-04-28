import { useEffect, useState } from 'react'
import { Table, Button, Space, Modal, Form, Input, Select, message, Tag, Upload } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, UploadOutlined } from '@ant-design/icons'
import { useAppDispatch, useAppSelector } from '../store/hooks'
import { fetchClusters, createCluster, updateCluster, deleteCluster, testConnectionForCreate } from '../store/slices/clusterSlice'
import { Cluster } from '../services/cluster'
import { clusterAPI } from '../services/cluster'

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
  const [keytabFile, setKeytabFile] = useState<File | null>(null)
  const [keytabTempId, setKeytabTempId] = useState<string>('')
  const [uploadingKeytab, setUploadingKeytab] = useState(false)

  useEffect(() => {
    dispatch(fetchClusters({ page, pageSize }))
  }, [dispatch, page, pageSize])

  // 构建认证配置
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
      // 使用已上传的 keytab temp ID
      if (keytabTempId) {
        authConfig.keytab_temp_id = keytabTempId
      }
    }
    return Object.keys(authConfig).length > 0 ? authConfig : undefined
  }

  // 上传 keytab 文件
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
    return false // 阻止默认上传行为
  }

  // 创建集群
  const handleCreate = async () => {
    try {
      const values = await form.validateFields()
      
      // Kerberos 认证需要先上传 keytab
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
        jmx_exporter_url: values.jmx_exporter_url,
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

  // 编辑集群
  const handleEdit = (record: Cluster) => {
    setEditingCluster(record)
    setIsEditModal(true)
    
    // 设置表单值（只有可修改的字段）
    const formValues: any = {
      cluster_name: record.cluster_name,
      jmx_exporter_url: record.jmx_exporter_url,
      description: record.description,
    }
    
    form.setFieldsValue(formValues)
  }

  // 更新集群
  const handleUpdate = async () => {
    if (!editingCluster) return
    
    try {
      const values = await form.validateFields()
      
      // 更新集群（只有可修改的字段）
      const updateData = {
        cluster_name: values.cluster_name,
        jmx_exporter_url: values.jmx_exporter_url,
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
  const renderForm = () => {
    if (isEditModal && editingCluster) {
      // 编辑模式：只显示可修改的字段，其他信息只读显示
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
            name="jmx_exporter_url" 
            label="JMX Exporter URL"
            extra="JMX Exporter 的 HTTP 地址，用于获取 Broker 内部指标"
          >
            <Input placeholder="例如：http://broker1:7071/metrics" />
          </Form.Item>

          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} placeholder="集群用途说明" />
          </Form.Item>
        </Form>
      )
    }
    
    // 创建模式：显示所有字段
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

      {/* SASL 认证配置 */}
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

      {/* Kerberos 认证配置 */}
      {authType === 'kerberos' && (
        <>
          <Form.Item 
            name="kerberos_principal" 
            label="Principal" 
            rules={[{ required: true, message: '请输入 Principal' }]}
            extra="格式：user@REALM 或 user/hostname@REALM，Realm 会自动从 Principal 中提取"
          >
            <Input placeholder="例如：kafka-client/node01@EXAMPLE.COM" />
          </Form.Item>
          <Form.Item 
            name="kerberos_service_name" 
            label="Service Name" 
            initialValue="kafka"
            extra="Kafka 服务名称，通常为 kafka"
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
  }

[domain_realm]
  .example.com = EXAMPLE.COM
  example.com = EXAMPLE.COM`}
            />
          </Form.Item>
          <Form.Item 
            label="Keytab 文件" 
            required 
            extra={isEditModal ? "如需更新 Keytab，请上传新文件；否则保持现有文件" : "请上传 Keytab 文件"}
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
            {keytabTempId && <span style={{ marginLeft: 8, color: '#52c41a' }}>已上传</span>}
            {isEditModal && !keytabTempId && !keytabFile && (
              <span style={{ marginLeft: 8, color: '#999' }}>保持现有文件</span>
            )}
          </Form.Item>
        </>
      )}

      <Form.Item 
        name="jmx_exporter_url" 
        label="JMX Exporter URL"
        extra="JMX Exporter 的 HTTP 地址，用于获取 Broker 内部指标（吞吐量、ISR、GC 等）"
      >
        <Input placeholder="例如：http://broker1:7071/metrics" />
      </Form.Item>

      <Form.Item name="description" label="描述">
        <Input.TextArea rows={3} placeholder="集群用途说明" />
      </Form.Item>
    </Form>
    )
  }

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
        onCancel={() => { 
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
        <div style={{ marginTop: 16, color: '#666' }}>
          提示：创建前会自动测试 Kafka 集群连接，只有连接成功才能创建。
        </div>
      </Modal>

      {/* 编辑集群对话框 */}
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
        <div style={{ marginTop: 16, color: '#666' }}>
          提示：Bootstrap Servers 和认证配置创建后不可修改。
        </div>
      </Modal>
    </div>
  )
}

export default ClusterList
