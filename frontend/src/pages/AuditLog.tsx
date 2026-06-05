import { useState, useEffect } from 'react'
import { Modal, Select, Button, message, DatePicker } from 'antd'
import { DownloadOutlined, EyeOutlined } from '@ant-design/icons'
import { auditLogAPI } from '../services/auditLog'
import { SearchBar, LabelTag } from '../components/bento'

const { RangePicker } = DatePicker

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
  const [pageSize] = useState(20)

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

  const handleDateChange = (_dates: any, dateStrings: [string, string]) => {
    setDateRange(dateStrings[0] ? dateStrings : null)
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

  const getActionColor = (act: string): 'green' | 'orange' | 'red' | 'purple' | 'pink' | 'blue' | 'neutral' => {
    if (act.includes('login')) return 'green'
    if (act.includes('logout')) return 'neutral'
    if (act.includes('create')) return 'green'
    if (act.includes('update')) return 'orange'
    if (act.includes('delete')) return 'red'
    if (act.includes('grant')) return 'purple'
    if (act.includes('revoke')) return 'pink'
    if (act.includes('sync')) return 'blue'
    return 'neutral'
  }

  const gridCols = '150px 80px 280px 70px 60px 65px 1fr 70px'

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
      {/* Header */}
      <div className="page-header">
        <div className="page-header-row">
          <div>
            <h1>审计日志</h1>
            <div className="page-accent-line" />
          </div>
          <div style={{ display: 'flex', gap: 8 }}>
            <button className="bento-action-btn" style={{ padding: '8px 16px', fontSize: 13 }} onClick={() => handleExport('csv')}>
              <DownloadOutlined /> CSV
            </button>
            <button className="bento-action-btn" style={{ padding: '8px 16px', fontSize: 13 }} onClick={() => handleExport('json')}>
              <DownloadOutlined /> JSON
            </button>
          </div>
        </div>
      </div>

      {/* Filters */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 20, flexWrap: 'wrap' }}>
        <SearchBar value={username} onChange={handleSearch} placeholder="搜索用户名..." style={{ flex: '0 0 220px' }} />
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
      </div>

      {/* Table header */}
      <div className="bento-table-header" style={{ gridTemplateColumns: gridCols }}>
        <div>Time</div>
        <div>User</div>
        <div>Action</div>
        <div>Resource</div>
        <div>Res. ID</div>
        <div>Status</div>
        <div>IP Address</div>
        <div style={{ textAlign: 'right' }}>Detail</div>
      </div>

      {/* Table body */}
      <div className="bento-table-body">
        {loading && <div style={{ textAlign: 'center', padding: 48, color: 'var(--text-3)' }}>加载中...</div>}
        {!loading && logs.map(log => (
          <div key={log.log_id} className={`bento-table-row${log.status === 'failed' ? ' bento-table-row--failed' : ''}`} style={{ gridTemplateColumns: gridCols }}>
            <div style={{ fontSize: 12, color: 'var(--text-3)' }}>{new Date(log.created_at).toLocaleString('zh-CN')}</div>
            <div style={{ fontWeight: 700, fontSize: 13 }}>{log.username}</div>
            <div style={{ overflow: 'hidden', maxWidth: '100%' }}>
              <LabelTag text={log.action} color={getActionColor(log.action)} />
            </div>
            <div style={{ fontSize: 12 }}>{log.resource}</div>
            <div style={{ fontSize: 12, fontFamily: 'var(--font-mono)', color: 'var(--text-3)' }}>{log.resource_id}</div>
            <div>
              <LabelTag text={log.status === 'success' ? '成功' : '失败'} color={log.status === 'success' ? 'green' : 'red'} />
            </div>
            <div style={{ fontSize: 12, fontFamily: 'var(--font-mono)', color: 'var(--text-3)' }}>{log.ip_address}</div>
            <div style={{ textAlign: 'right' }}>
              <button className="bento-action-btn" onClick={() => showDetail(log)}>
                <EyeOutlined /> 详情
              </button>
            </div>
          </div>
        ))}
        {!loading && logs.length === 0 && (
          <div style={{ textAlign: 'center', padding: 48, color: 'var(--text-3)' }}>暂无审计日志数据</div>
        )}
      </div>

      {/* Pagination */}
      {total > pageSize && (
        <div className="bento-pagination">
          <span className="bento-pagination-info">
            Showing {(page - 1) * pageSize + 1}-{Math.min(page * pageSize, total)} of {total}
          </span>
          <div className="bento-pagination-buttons">
            <button className="bento-pagination-btn" disabled={page <= 1} onClick={() => setPage(page - 1)}>&larr;</button>
            {Array.from({ length: Math.ceil(total / pageSize) }, (_, i) => i + 1)
              .filter(p => Math.abs(p - page) <= 2)
              .map(p => (
                <button
                  key={p}
                  className={`bento-pagination-btn${p === page ? ' bento-pagination-btn--active' : ''}`}
                  onClick={() => setPage(p)}
                >{p}</button>
              ))}
            <button className="bento-pagination-btn" disabled={page >= Math.ceil(total / pageSize)} onClick={() => setPage(page + 1)}>&rarr;</button>
          </div>
        </div>
      )}

      {/* Detail Modal */}
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
              <span style={{ color: 'var(--text-3)' }}>时间</span>
              <span>{new Date(currentLog.created_at).toLocaleString('zh-CN')}</span>
              <span style={{ color: 'var(--text-3)' }}>用户</span>
              <span><strong>{currentLog.username}</strong></span>
              <span style={{ color: 'var(--text-3)' }}>操作</span>
              <span><LabelTag text={currentLog.action} color={getActionColor(currentLog.action)} /></span>
              <span style={{ color: 'var(--text-3)' }}>资源类型</span>
              <span>{currentLog.resource}</span>
              <span style={{ color: 'var(--text-3)' }}>资源ID</span>
              <span style={{ fontFamily: 'var(--font-mono)' }}>{currentLog.resource_id}</span>
              <span style={{ color: 'var(--text-3)' }}>状态</span>
              <span><LabelTag text={currentLog.status === 'success' ? '成功' : '失败'} color={currentLog.status === 'success' ? 'green' : 'red'} /></span>
              <span style={{ color: 'var(--text-3)' }}>IP地址</span>
              <span style={{ fontFamily: 'var(--font-mono)' }}>{currentLog.ip_address}</span>
              <span style={{ color: 'var(--text-3)' }}>用户代理</span>
              <span style={{ wordBreak: 'break-all' }}>{currentLog.user_agent}</span>
              <span style={{ color: 'var(--text-3)' }}>详情</span>
              <span>{currentLog.details || '-'}</span>
            </div>
            {currentLog.error_msg && (
              <div style={{ padding: '10px 12px', background: 'rgba(239,68,68,0.06)', borderRadius: 'var(--radius-md)', fontSize: 13, border: '1px solid rgba(239,68,68,0.15)' }}>
                <span style={{ color: 'var(--text-3)', marginRight: 8 }}>错误信息：</span>
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
