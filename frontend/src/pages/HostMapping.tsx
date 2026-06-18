import { useEffect, useState } from 'react'
import { Button, Modal, Form, Input, message } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import { hostMappingAPI, HostMapping } from '../services/hostMapping'
import { StatCard, SearchBar } from '../components/bento'

const HostMappingPage: React.FC = () => {
  const [mappings, setMappings] = useState<HostMapping[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [page, setPage] = useState(1)
  const [pageSize] = useState(20)
  const [searchText, setSearchText] = useState('')
  const [isModalVisible, setIsModalVisible] = useState(false)
  const [isEditModal, setIsEditModal] = useState(false)
  const [editingMapping, setEditingMapping] = useState<HostMapping | null>(null)
  const [form] = Form.useForm()

  const fetchMappings = async () => {
    setLoading(true)
    try {
      const res = await hostMappingAPI.list(page, pageSize, searchText)
      setMappings(res.data || [])
      setTotal(res.total || 0)
    } catch {
      message.error('获取主机映射列表失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchMappings()
  }, [page, pageSize, searchText])

  const handleCreate = async () => {
    try {
      const values = await form.validateFields()
      await hostMappingAPI.create(values)
      message.success('创建成功')
      setIsModalVisible(false)
      form.resetFields()
      fetchMappings()
    } catch (error: any) {
      message.error(error?.response?.data?.error || '创建失败')
    }
  }

  const handleEdit = (record: HostMapping) => {
    setEditingMapping(record)
    setIsEditModal(true)
    form.setFieldsValue({
      hostname: record.hostname,
      cluster_name: record.cluster_name,
      ip_address: record.ip_address,
      description: record.description,
    })
  }

  const handleUpdate = async () => {
    if (!editingMapping) return
    try {
      const values = await form.validateFields()
      await hostMappingAPI.update(editingMapping.id, values)
      message.success('更新成功')
      setIsEditModal(false)
      setEditingMapping(null)
      form.resetFields()
      fetchMappings()
    } catch (error: any) {
      message.error(error?.response?.data?.error || '更新失败')
    }
  }

  const handleDelete = (id: number) => {
    Modal.confirm({
      title: '确认删除',
      content: '确定要删除这个主机映射吗？',
      okText: '确认',
      cancelText: '取消',
      onOk: async () => {
        try {
          await hostMappingAPI.delete(id)
          message.success('删除成功')
          fetchMappings()
        } catch {
          message.error('删除失败')
        }
      },
    })
  }

  const renderForm = () => (
    <Form form={form} layout="vertical">
      <Form.Item name="hostname" label="主机名" rules={[{ required: true, message: '请输入主机名' }]} extra="Kafka Broker 使用的主机名，如 broker1.example.com">
        <Input placeholder="例如：broker1.example.com" />
      </Form.Item>
      <Form.Item name="cluster_name" label="集群名称" extra="留空表示全局映射，填写集群名表示该映射仅对指定集群生效">
        <Input placeholder="例如：kafka-public-prod（可选）" />
      </Form.Item>
      <Form.Item name="ip_address" label="IP 地址" rules={[{ required: true, message: '请输入 IP 地址' }]}>
        <Input placeholder="例如：192.168.1.100" />
      </Form.Item>
      <Form.Item name="description" label="描述">
        <Input.TextArea rows={3} placeholder="可选，用途说明" />
      </Form.Item>
    </Form>
  )

  return (
    <div>
      <div className="page-header">
        <div className="page-header-row">
          <div>
            <h1>主机映射</h1>
            <div className="page-accent-line" />
          </div>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => { setIsModalVisible(true); setIsEditModal(false); form.resetFields() }}>
            添加映射
          </Button>
        </div>
      </div>

      {/* Stat cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 16, marginBottom: 20 }}>
        <StatCard label="TOTAL MAPPINGS" value={total} />
      </div>

      {/* Search */}
      <div style={{ display: 'flex', gap: 12, marginBottom: 20 }}>
        <SearchBar value={searchText} onChange={setSearchText} placeholder="搜索主机名、集群名或 IP..." />
      </div>

      {/* Table */}
      <div className="bento-card" style={{ marginBottom: 20 }}>
        <div className="bento-card-inner" style={{ padding: 0 }}>
          <table className="bento-table">
            <thead>
              <tr>
                <th style={{ textAlign: 'center' }}>HOSTNAME</th>
                <th style={{ textAlign: 'center' }}>CLUSTER</th>
                <th style={{ textAlign: 'center' }}>IP ADDRESS</th>
                <th style={{ textAlign: 'center' }}>DESCRIPTION</th>
                <th style={{ textAlign: 'center' }}>CREATED</th>
                <th style={{ textAlign: 'right' }}>ACTIONS</th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                <tr><td colSpan={6} style={{ textAlign: 'center', padding: 40 }}>加载中...</td></tr>
              ) : mappings.length === 0 ? (
                <tr><td colSpan={6} style={{ textAlign: 'center', padding: 40 }}>暂无数据</td></tr>
              ) : (
                mappings.map(m => (
                  <tr key={m.id}>
                    <td style={{ textAlign: 'center' }}><code style={{ fontFamily: 'var(--font-mono)', fontSize: 13 }}>{m.hostname}</code></td>
                    <td style={{ textAlign: 'center' }}><span style={{ fontSize: 13, color: m.cluster_name ? 'var(--brand)' : 'var(--text-muted)' }}>{m.cluster_name || '全局'}</span></td>
                    <td style={{ textAlign: 'center' }}><code style={{ fontFamily: 'var(--font-mono)', fontSize: 13 }}>{m.ip_address}</code></td>
                    <td style={{ textAlign: 'center' }}>{m.description || '-'}</td>
                    <td style={{ textAlign: 'center' }}>{new Date(m.created_at).toLocaleDateString()}</td>
                    <td style={{ textAlign: 'right' }}>
                      <div style={{ display: 'flex', gap: 6, justifyContent: 'flex-end' }}>
                        <Button size="small" icon={<EditOutlined />} onClick={() => handleEdit(m)}>编辑</Button>
                        <Button size="small" danger icon={<DeleteOutlined />} onClick={() => handleDelete(m.id)}>删除</Button>
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Pagination */}
      {total > pageSize && (
        <div style={{ display: 'flex', justifyContent: 'center', gap: 8 }}>
          <Button disabled={page <= 1} onClick={() => setPage(p => p - 1)}>上一页</Button>
          <span style={{ lineHeight: '32px', fontFamily: 'var(--font-mono)' }}>{page} / {Math.ceil(total / pageSize)}</span>
          <Button disabled={page * pageSize >= total} onClick={() => setPage(p => p + 1)}>下一页</Button>
        </div>
      )}

      {/* Create Modal */}
      <Modal
        title="添加主机映射"
        open={isModalVisible && !isEditModal}
        onOk={handleCreate}
        onCancel={() => { setIsModalVisible(false); form.resetFields() }}
        okText="创建"
        cancelText="取消"
      >
        {renderForm()}
      </Modal>

      {/* Edit Modal */}
      <Modal
        title="编辑主机映射"
        open={isEditModal}
        onOk={handleUpdate}
        onCancel={() => { setIsEditModal(false); setEditingMapping(null); form.resetFields() }}
        okText="保存"
        cancelText="取消"
      >
        {renderForm()}
      </Modal>
    </div>
  )
}

export default HostMappingPage
