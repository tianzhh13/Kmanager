import { useState, useEffect } from 'react'
import { Card, Table, Select, Input, DatePicker, Button, Space, Tag, message, Modal } from 'antd'
import { DownloadOutlined, EyeOutlined } from '@ant-design/icons'
import { auditLogAPI } from '../services/auditLog'
import type { ColumnsType } from 'antd/es/table'

const { RangePicker } = DatePicker
const { Search } = Input

interface AuditLog {
  id: number
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
  
  // 筛选条件
  const [username, setUsername] = useState<string>('')
  const [action, setAction] = useState<string>('')
  const [resourceType, setResourceType] = useState<string>('')
  const [status, setStatus] = useState<string>('')
  const [dateRange, setDateRange] = useState<[string, string] | null>(null)
  
  // 详情弹窗
  const [detailVisible, setDetailVisible] = useState(false)
  const [currentLog, setCurrentLog] = useState<AuditLog | null>(null)

  useEffect(() => {
    loadLogs()
  }, [page, pageSize, username, action, resourceType, status, dateRange])

  const loadLogs = async () => {
    setLoading(true)
    try {
      const params: any = {
        page,
        page_size: pageSize
      }
      
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
      
      // 下载文件
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
    return status === 'success' ? 'green' : 'red'
  }

  const getActionColor = (action: string) => {
    const colorMap: Record<string, string> = {
      login: 'blue',
      logout: 'default',
      create: 'green',
      update: 'orange',
      delete: 'red',
      grant: 'purple',
      revoke: 'magenta'
    }
    for (const key of Object.keys(colorMap)) {
      if (action.includes(key)) {
        return colorMap[key]
      }
    }
    return 'default'
  }

  const columns: ColumnsType<AuditLog> = [
    {
      title: '时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 180,
      render: (text: string) => new Date(text).toLocaleString('zh-CN')
    },
    {
      title: '用户',
      dataIndex: 'username',
      key: 'username',
      width: 120
    },
    {
      title: '操作',
      dataIndex: 'action',
      key: 'action',
      width: 150,
      render: (text: string) => <Tag color={getActionColor(text)}>{text}</Tag>
    },
    {
      title: '资源类型',
      dataIndex: 'resource',
      key: 'resource',
      width: 100
    },
    {
      title: '资源ID',
      dataIndex: 'resource_id',
      key: 'resource_id',
      width: 100
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
      width: 140
    },
    {
      title: '操作',
      key: 'action',
      width: 80,
      render: (_, record) => (
        <Button 
          type="link" 
          icon={<EyeOutlined />} 
          onClick={() => showDetail(record)}
        >
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
    { label: '创建Topic', value: 'create_topic' },
    { label: '删除Topic', value: 'delete_topic' },
    { label: '创建ACL', value: 'create_acl' },
    { label: '删除ACL', value: 'delete_acl' },
    { label: '创建用户', value: 'create_user' },
    { label: '删除用户', value: 'delete_user' }
  ]

  const resourceOptions = [
    { label: '用户', value: 'user' },
    { label: '集群', value: 'cluster' },
    { label: 'Topic', value: 'topic' },
    { label: 'ACL', value: 'acl' },
    { label: '系统', value: 'system' }
  ]

  const statusOptions = [
    { label: '成功', value: 'success' },
    { label: '失败', value: 'failed' }
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card title="审计日志">
        {/* 筛选条件 */}
        <Space wrap style={{ marginBottom: 16 }}>
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
          <RangePicker
            showTime
            onChange={handleDateChange}
          />
          <Button onClick={handleReset}>重置</Button>
        </Space>

        {/* 导出按钮 */}
        <Space style={{ marginBottom: 16, float: 'right' }}>
          <Button 
            icon={<DownloadOutlined />} 
            onClick={() => handleExport('csv')}
          >
            导出 CSV
          </Button>
          <Button 
            icon={<DownloadOutlined />} 
            onClick={() => handleExport('json')}
          >
            导出 JSON
          </Button>
        </Space>

        {/* 日志表格 */}
        <Table
          columns={columns}
          dataSource={logs}
          rowKey="id"
          loading={loading}
          pagination={{
            current: page,
            pageSize: pageSize,
            total: total,
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (total) => `共 ${total} 条`,
            onChange: (p, ps) => {
              setPage(p)
              setPageSize(ps)
            }
          }}
        />
      </Card>

      {/* 详情弹窗 */}
      <Modal
        title="审计日志详情"
        open={detailVisible}
        onCancel={() => setDetailVisible(false)}
        footer={null}
        width={600}
      >
        {currentLog && (
          <div>
            <p><strong>时间：</strong>{new Date(currentLog.created_at).toLocaleString('zh-CN')}</p>
            <p><strong>用户：</strong>{currentLog.username}</p>
            <p><strong>操作：</strong>{currentLog.action}</p>
            <p><strong>资源类型：</strong>{currentLog.resource}</p>
            <p><strong>资源ID：</strong>{currentLog.resource_id}</p>
            <p><strong>状态：</strong><Tag color={getStatusColor(currentLog.status)}>{currentLog.status}</Tag></p>
            <p><strong>IP地址：</strong>{currentLog.ip_address}</p>
            <p><strong>用户代理：</strong>{currentLog.user_agent}</p>
            <p><strong>详情：</strong>{currentLog.details || '-'}</p>
            {currentLog.error_msg && (
              <p><strong>错误信息：</strong><span style={{ color: 'red' }}>{currentLog.error_msg}</span></p>
            )}
          </div>
        )}
      </Modal>
    </div>
  )
}

export default AuditLogPage