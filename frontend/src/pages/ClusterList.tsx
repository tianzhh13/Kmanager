import { useEffect, useState } from 'react'
import { Table, Button, Space, Modal, Form, Input, Select, message, Tag, Upload, Switch, InputNumber } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, CheckCircleOutlined, UploadOutlined } from '@ant-design/icons'
import { useAppDispatch, useAppSelector } from '../store/hooks'
import { fetchClusters, createCluster, testClusterConnection } from '../store/slices/clusterSlice'

const ClusterList: React.FC = () => {
  const dispatch = useAppDispatch()
  const { clusters, total, loading } = useAppSelector((state) => state.clusters)
  const [isModalVisible, setIsModalVisible] = useState(false)
  const [form] = Form.useForm()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [authType, setAuthType] = useState<string>('none')

  useEffect(() => {
    dispatch(fetchClusters({ page, pageSize }))
  }, [dispatch, page, pageSize])

  const handleCreate = async () => {
    try {
      const values = await form.validateFields()
      
      // 构建认证配置
      const authConfig: any = {}
      if (values.auth_type === 'scram') {
        authConfig.username = values.scram_username
        authConfig.password = values.scram_password
        authConfig.mechanism = values.scram_mechanism || 'SCRAM-SHA-256'
      } else if (values.auth_type === 'kerberos') {
        authConfig.principal = values.kerberos_principal
        authConfig.keytab_path = values.kerberos_keytab_path
        authConfig.realm = values.kerberos_realm
        authConfig.service_name = values.kerberos_service_name || 'kafka'
      }
      
      const clusterData = {
        cluster_name: values.cluster_name,
        bootstrap_servers: values.bootstrap_servers,
        auth_type: values.auth_type,
        auth_config: Object.keys(authConfig).length > 0 ? authConfig : undefined,
        jmx_host: values.jmx_host,
        jmx_port: values.jmx_port,
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
    }
  }

  const handleTestConnection = async (id: number) => {
    try {
      await dispatch(testClusterConnection(id)).unwrap()
      message.success('连接测试成功')
    } catch (error: any) {
      message.error(error || '连接测试失败')
    }
  }

  const columns = [
    { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
    { title: '集群名称', dataIndex: 'cluster_name', key: 'cluster_name' },
    { title: 'Bootstrap Servers', dataIndex: 'bootstrap_servers', key: 'bootstrap_servers' },
    { title: '认证类型', dataIndex: 'auth_type', key: 'auth_type', 
      render: (type: string) => (
        <Tag color={type === 'none' ? 'green' : type === 'plaintext' ? 'green' : type === 'scram' ? 'blue' : 'orange'}>
          {type ? type.toUpperCase() : 'NONE'}
        </Tag>
      )
    },
    { title: 'JMX 端口', key: 'jmx', 
      render: (_: any, record: any) => record.jmx_port ? `${record.jmx_host || 'localhost'}:${record.jmx_port}` : '-'
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
      render: (_: any, record: any) => (
        <Space>
          <Button 
            type="link" 
            icon={<CheckCircleOutlined />} 
            onClick={() => handleTestConnection(record.id)}
          >
            测试
          </Button>
          <Button type="link" icon={<EditOutlined />}>编辑</Button>
          <Button type="link" danger icon={<DeleteOutlined />}>删除</Button>
        </Space>
      )
    },
  ]

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
        rowKey="id"
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
        onCancel={() => { setIsModalVisible(false); form.resetFields(); setAuthType('none') }}
        width={700}
      >
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
              <Form.Item 
                name="kerberos_krb5_conf" 
                label="krb5.conf 文件路径"
                extra="Kerberos 配置文件路径，通常为 /etc/krb5.conf"
              >
                <Input placeholder="例如：/etc/krb5.conf" />
              </Form.Item>
            </>
          )}

          {/* JMX 监控配置 */}
          <Form.Item label="JMX 监控配置">
            <Space>
              <Form.Item name="jmx_host" noStyle initialValue="localhost">
                <Input placeholder="JMX 主机" style={{ width: 200 }} />
              </Form.Item>
              <Form.Item name="jmx_port" noStyle>
                <InputNumber placeholder="JMX 端口" min={1} max={65535} style={{ width: 150 }} />
              </Form.Item>
            </Space>
          </Form.Item>

          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} placeholder="集群用途说明" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default ClusterList