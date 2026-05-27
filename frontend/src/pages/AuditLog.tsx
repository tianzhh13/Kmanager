import { useState, useEffect } from 'react'
import { Card, Table, Select, Input, DatePicker, Button, Space, Tag, message, Modal } from 'antd'
import { DownloadOutlined, EyeOutlined } from '@ant-design/icons'
import { auditLogAPI } from '../services/auditLog'
import type { ColumnsType } from 'antd/es/table'

const { RangePicker } = DatePicker
const { Search } = Input

interface AuditLog {
  log_id: number
  user_id: number
  username: string
  action: string
  resource: string
  resource_id: string
  details: string
  ip_address: string
  user_agent: string
  status: string
  error_msg?: string
  created_at: string
}

const AuditLogPage: React.FC = () => {
  const [loading, setLoading] = useState(false)
  const [logs, setLogs] = useState<AuditLog[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  
  const [username, setUsername] = useState<string>('')
  const [action, setAction] = useState<string>('')
  const [resourceType, setResourceType] = useState<string>('')
  const [status, setStatus] = useState<string>('')
  const [dateRange, setDateRange] = useState<[string, string] | null>(null)
  
  const [detailVisible, setDetailVisible] = useState(false)
  const [currentLog, setCurrentLog] = useState<AuditLog | null>(null)

  useEffect(() => {
    loadLogs()
  }, [page, pageSize, username, action, resourceType, status, dateRange])

  const loadLogs = async () => {
    setLoading(true)
    try {
      const params: any = { page, page_size: pageSize }
      if (username) params.username = username
      if (action) params.operation = action
      if (resourceType) params.resource_type = resourceType
      if (status) params.status = status
      if (dateRange) {
        params.start_time = dateRange[0]
        params.end_time = dateRange[1]
      }
      const res = await auditLogAPI.list(params)
      setLogs(res.data.data || [])
      setTotal(res.data.total || 0)
    } catch (error) {
      message.error('加载审计日志失败')
    } finally {
      setLoading(false)
    }
  }

  const handleSearch = (value: string) => {
    setUsername(value)
    setPage(1)
  }

  const handleReset = () => {
    setUsername('')
    setAction('')
    setResourceType('')
    setStatus('')
    setDateRange(null)
    setPage(1)
  }

  const handleDateChange = (dates: any, dateStrings: [string, string]) => {
    setDateRange(dates ? dateStrings : null)
    setPage(1)
  }

  const handleExport = async (format: 'csv' | 'json') => {
    try {
      const params: any = { format }
      if (username) params.username = username
      if (action) params.operation = action
      if (resourceType) params.resource_type = resourceType
      if (status) params.status = status
      if (dateRange) {
        params.start_time = dateRange[0]
        params.end_time = dateRange[1]
      }
      const res = await auditLogAPI.export(params)
      const blob = new Blob([format === 'json' ? JSON.stringify(res.data, null, 2) : res.data], {
        type: format === 'json' ? 'application/json' : 'text/csv'
      })
      const url = window.URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `audit_logs_${new Date().toISOString()}.${format}`
      link.click()
      window.URL.revokeObjectURL(url)
      message.success('导出成功')
    } catch (error) {
      message.error('导出失败')
    }
  }

  const showDetail = (log: AuditLog) => {
    setCurrentLog(log)
    setDetailVisible(true)
  }

  const getStatusColor = (status: string) => {
    return status === 'success' ? 'success' : 'error'
  }

  const getActionColor = (action: string) => {
    const colorMap: Record<string, string> = {
      login: 'processing', logout: 'default', create: 'success', update: 'warning',
      delete: 'error', grant: 'purple', revoke: 'magenta'
    }
    for (const key of Object.keys(colorMap)) {
      if (action.includes(key)) return colorMap[key]
    }
    return 'default'
  }

  const columns: ColumnsType<AuditLog> = [
    {
      title: '时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 180,
      render: (text: string) => <span style={{ color: 'var(--text-tertiary)', fontSize: 13 }}>{new Date(text).toLocaleString('zh-CN')}</span>,
    },
    {
      title: '用户',
      dataIndex: 'username',
      key: 'username',
      width: 120,
      render: (text: string) => <strong style={{ color: 'var(--text-heading)' }}>{text}</strong>,
    },
    {
      title: '操作',
      dataIndex: 'action',
      key: 'action',
      width: 150,
      render: (text: string) => <Tag color={getActionColor(text)}>{text}</Tag>
    },
    { title: '资源类型', dataIndex: 'resource', key: 'resource', width: 100 },
    { title: '资源ID', dataIndex: 'resource_id', key: 'resource_id', width: 100,
      render: (text: string) => <span className="text-mono" style={{ fontSize: 12 }}>{text}</span>,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 80,
      render: (text: string) => (
        <Tag color={getStatusColor(text)}>{text === 'success' ? '成功' : '失败'}</Tag>
      )
    },
    {
      title: 'IP地址',
      dataIndex: 'ip_address',
      key: 'ip_address',
      width: 140,
      render: (text: string) => <span className="text-mono" style={{ fontSize: 12 }}>{text}</span>,
    },
    {
      title: '操作',
      key: 'action',
      width: 80,
      render: (_, record) => (
        <Button type="link" icon={<EyeOutlined />} onClick={() => showDetail(record)}>
          详情
        </Button>
      )
    }
  ]

  const actionOptions = [
    { label: '登录', value: 'login' },
    { label: '登出', value: 'logout' },
    { label: '创建集群', value: 'create_cluster' },
    { label: '更新集群', value: 'update_cluster' },
    { label: '删除集群', value: 'delete_cluster' },
    { label: '测试连接', value: 'test_connection' },
    { label: '授权集群访问', value: 'grant_cluster_access' },
    { label: '撤销集群访问', value: 'revoke_cluster_access' },
    { label: '创建Topic', value: 'create_topic' },
    { label: '更新Topic', value: 'update_topic' },
    { label: '删除Topic', value: 'delete_topic' },
    { label: '同步Topic', value: 'sync_topics' },
    { label: '创建ACL', value: 'create_acl' },
    { label: '删除ACL', value: 'delete_acl' },
    { label: '批量删除ACL', value: 'batch_delete_acl' },
    { label: '同步ACL', value: 'sync_acls' },
    { label: '创建用户', value: 'create_user' },
    { label: '更新用户', value: 'update_user' },
    { label: '删除用户', value: 'delete_user' },
    { label: '禁用用户', value: 'disable_user' },
    { label: '启用用户', value: 'enable_user' },
    { label: '修改密码', value: 'update_password' },
    { label: '创建SCRAM用户', value: 'create_scram_user' },
    { label: '删除SCRAM用户', value: 'delete_scram_user' },
    { label: '同步SCRAM用户', value: 'sync_scram_users' },
    { label: '分配Topic权限', value: 'assign_topic_permission' },
    { label: '撤销Topic权限', value: 'revoke_topic_permission' },
    { label: '导出日志', value: 'export_logs' },
    { label: '清理日志', value: 'clean_logs' },
  ]

  const resourceOptions = [
    { label: '用户', value: 'user' },
    { label: '集群', value: 'cluster' },
    { label: 'Topic', value: 'topic' },
    { label: 'ACL', value: 'acl' },
    { label: 'SCRAM用户', value: 'scram_user' },
    { label: 'Topic权限', value: 'topic_permission' },
    { label: '认证', value: 'auth' },
    { label: '监控', value: 'monitor' },
    { label: '审计日志', value: 'audit_log' },
    { label: '系统', value: 'system' },
  ]

  const statusOptions = [
    { label: '成功', value: 'success' },
    { label: '失败', value: 'failed' }
  ]

  return (
    <div>
      <div className="page-header">
        <h1>审计日志</h1>
        <div className="page-accent-line" />
      </div>

      <Card>
        {/* 筛选条件 */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16, flexWrap: 'wrap', gap: 12 }}>
          <Space wrap>
            <Search
              placeholder="搜索用户名"
              onSearch={handleSearch}
              style={{ width: 200 }}
              allowClear
            />
            <Select
              placeholder="操作类型"
              value={action || undefined}
              onChange={(val) => { setAction(val || ''); setPage(1) }}
              style={{ width: 150 }}
              options={actionOptions}
              allowClear
            />
            <Select
              placeholder="资源类型"
              value={resourceType || undefined}
              onChange={(val) => { setResourceType(val || ''); setPage(1) }}
              style={{ width: 120 }}
              options={resourceOptions}
              allowClear
            />
            <Select
              placeholder="状态"
              value={status || undefined}
              onChange={(val) => { setStatus(val || ''); setPage(1) }}
              style={{ width: 100 }}
              options={statusOptions}
              allowClear
            />
            <RangePicker showTime onChange={handleDateChange} />
            <Button onClick={handleReset}>重置</Button>
          </Space>
          <Space>
            <Button icon={<DownloadOutlined />} onClick={() => handleExport('csv')}>
              导出 CSV
            </Button>
            <Button icon={<DownloadOutlined />} onClick={() => handleExport('json')}>
              导出 JSON
            </Button>
          </Space>
        </div>

        <Table
          columns={columns}
          dataSource={logs}
          rowKey="log_id"
          loading={loading}
          pagination={{
            current: page,
            pageSize: pageSize,
            total: total,
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (total) => `共 ${total} 条`,
            onChange: (p, ps) => { setPage(p); setPageSize(ps) }
          }}
        />
      </Card>

      <Modal
        title="审计日志详情"
        open={detailVisible}
        onCancel={() => setDetailVisible(false)}
        footer={null}
        width={600}
      >
        {currentLog && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            <div style={{ display: 'grid', gridTemplateColumns: '80px 1fr', gap: '8px 16px', fontSize: 13 }}>
              <span style={{ color: 'var(--text-tertiary)' }}>时间</span>
              <span>{new Date(currentLog.created_at).toLocaleString('zh-CN')}</span>
              <span style={{ color: 'var(--text-tertiary)' }}>用户</span>
              <span><strong>{currentLog.username}</strong></span>
              <span style={{ color: 'var(--text-tertiary)' }}>操作</span>
              <span><Tag color={getActionColor(currentLog.action)}>{currentLog.action}</Tag></span>
              <span style={{ color: 'var(--text-tertiary)' }}>资源类型</span>
              <span>{currentLog.resource}</span>
              <span style={{ color: 'var(--text-tertiary)' }}>资源ID</span>
              <span className="text-mono">{currentLog.resource_id}</span>
              <span style={{ color: 'var(--text-tertiary)' }}>状态</span>
              <span><Tag color={getStatusColor(currentLog.status)}>{currentLog.status === 'success' ? '成功' : '失败'}</Tag></span>
              <span style={{ color: 'var(--text-tertiary)' }}>IP地址</span>
              <span className="text-mono">{currentLog.ip_address}</span>
              <span style={{ color: 'var(--text-tertiary)' }}>用户代理</span>
              <span style={{ wordBreak: 'break-all' }}>{currentLog.user_agent}</span>
              <span style={{ color: 'var(--text-tertiary)' }}>详情</span>
              <span>{currentLog.details || '-'}</span>
            </div>
            {currentLog.error_msg && (
              <div style={{ padding: '10px 12px', background: 'var(--color-error-bg)', borderRadius: 'var(--radius-md)', fontSize: 13 }}>
                <span style={{ color: 'var(--text-tertiary)', marginRight: 8 }}>错误信息：</span>
                <span style={{ color: 'var(--color-error)' }}>{currentLog.error_msg}</span>
              </div>
            )}
          </div>
        )}
      </Modal>
    </div>
  )
}

export default AuditLogPage
