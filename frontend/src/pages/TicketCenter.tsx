import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Tabs, Drawer, Modal, Input, Select, message } from 'antd'
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import { ticketService, Ticket, TicketStatus, TicketType, TicketExecuteResult, expandACLPreview, PermissionGrantForm, TopicCreateForm, TopicCreateItem } from '../services/ticket'
import { clusterAPI, ClusterOption } from '../services/cluster'
import { StatCard, LabelTag, LabelColor } from '../components/bento'
import { useAppSelector } from '../store/hooks'

// 状态中文与标签色映射
const statusMeta: Record<TicketStatus, { label: string; color: LabelColor }> = {
  pending: { label: '待审批', color: 'warning' },
  executing: { label: '执行中', color: 'blue' },
  succeeded: { label: '已完成', color: 'green' },
  failed: { label: '执行失败', color: 'red' },
  rejected: { label: '已驳回', color: 'pink' },
  cancelled: { label: '已撤回', color: 'neutral' },
}

const typeMeta: Record<TicketType, { label: string; color: LabelColor }> = {
  topic_create: { label: '申请 Topic', color: 'purple' },
  permission_grant: { label: '申请权限', color: 'orange' },
}

const permissionLabels: Record<string, string> = {
  read: '只读',
  write: '只写',
  read_write: '读写',
}

const TicketCenter: React.FC = () => {
  const navigate = useNavigate()
  const { user } = useAppSelector((state) => state.auth)
  const userRole = user?.role || ''

  const isAdmin = userRole === 'super_admin' || userRole === 'cluster_admin'

  const [tickets, setTickets] = useState<Ticket[]>([])
  const [total, setTotal] = useState(0)
  const [stats, setStats] = useState<Record<string, number>>({})
  const [loading, setLoading] = useState(false)
  const [page, setPage] = useState(1)
  const [pageSize] = useState(20)
  const [scope, setScope] = useState<'mine' | 'pending' | 'all'>('mine')
  const [statusFilter, setStatusFilter] = useState('')
  const [clusters, setClusters] = useState<ClusterOption[]>([])
  const [detail, setDetail] = useState<Ticket | null>(null)
  const [actionLoading, setActionLoading] = useState(false)
  const [rejectModal, setRejectModal] = useState<{ open: boolean; remark: string }>({ open: false, remark: '' })
  const [credentialModal, setCredentialModal] = useState<{ open: boolean; data: { username: string; password: string; mechanism: string } | null }>({ open: false, data: null })

  // 拉取集群基础信息（options 接口对所有用户可见，用于展示集群名）
  useEffect(() => {
    clusterAPI.options().then(setClusters).catch(() => { /* 集群列表加载失败不影响工单展示 */ })
  }, [])

  const fetchTickets = useCallback(async () => {
    setLoading(true)
    try {
      const res = await ticketService.list({ scope, status: statusFilter || undefined, page, page_size: pageSize })
      setTickets(res.data || [])
      setTotal(res.total || 0)
    } catch {
      message.error('获取工单列表失败')
    } finally {
      setLoading(false)
    }
  }, [scope, statusFilter, page, pageSize])

  const fetchStats = useCallback(async () => {
    try {
      const res = await ticketService.stats()
      setStats(res || {})
    } catch { /* 统计加载失败静默 */ }
  }, [])

  useEffect(() => {
    fetchTickets()
  }, [fetchTickets])

  useEffect(() => {
    fetchStats()
  }, [fetchStats])

  const clusterName = (clusterId: number) => clusters.find(c => c.cluster_id === clusterId)?.cluster_name || `集群#${clusterId}`

  const refreshAll = () => {
    fetchTickets()
    fetchStats()
  }

  // ============ 工单操作 ============
  const openDetail = async (id: number) => {
    try {
      const t = await ticketService.get(id)
      setDetail(t)
    } catch (error: any) {
      message.error(error?.response?.data?.error || '获取工单详情失败')
    }
  }

  const handleApprove = async (id: number) => {
    setActionLoading(true)
    try {
      const t = await ticketService.approve(id)
      message.success(t.status === 'succeeded' ? '审批通过，已执行完成' : `审批通过，工单状态: ${statusMeta[t.status].label}`)
      setDetail(t)
      refreshAll()
    } catch (error: any) {
      message.error(error?.response?.data?.error || '审批失败')
    } finally {
      setActionLoading(false)
    }
  }

  const handleReject = async (id: number) => {
    if (!rejectModal.remark.trim()) {
      message.warning('请填写驳回原因')
      return
    }
    setActionLoading(true)
    try {
      const t = await ticketService.reject(id, rejectModal.remark.trim())
      message.success('已驳回')
      setDetail(t)
      setRejectModal({ open: false, remark: '' })
      refreshAll()
    } catch (error: any) {
      message.error(error?.response?.data?.error || '驳回失败')
    } finally {
      setActionLoading(false)
    }
  }

  const handleCancel = async (id: number) => {
    setActionLoading(true)
    try {
      const t = await ticketService.cancel(id)
      message.success('已撤回')
      setDetail(t)
      refreshAll()
    } catch (error: any) {
      message.error(error?.response?.data?.error || '撤回失败')
    } finally {
      setActionLoading(false)
    }
  }

  const handleRetry = async (id: number) => {
    setActionLoading(true)
    try {
      const t = await ticketService.retry(id)
      message.success(t.status === 'succeeded' ? '重试成功' : `重试后状态: ${statusMeta[t.status].label}`)
      setDetail(t)
      refreshAll()
    } catch (error: any) {
      message.error(error?.response?.data?.error || '重试失败')
    } finally {
      setActionLoading(false)
    }
  }

  const handleTakeCredential = async (id: number) => {
    setActionLoading(true)
    try {
      const cred = await ticketService.takeCredential(id)
      setCredentialModal({ open: true, data: cred })
      // 凭证一次性展示，同时刷新详情（credential_viewed_at 已更新）
      const t = await ticketService.get(id)
      setDetail(t)
    } catch (error: any) {
      message.error(error?.response?.data?.error || '获取凭证失败')
    } finally {
      setActionLoading(false)
    }
  }

  // ============ 权限判断 ============
  const isApplicant = detail ? detail.applicant_id === user?.user_id : false
  const canApprove = detail && isAdmin && detail.status === 'pending'
  const canCancel = detail && isApplicant && detail.status === 'pending'
  const canRetry = detail && isAdmin && detail.status === 'failed'

  // 权限工单的表单与凭证可用性
  const detailForm = detail ? JSON.parse(detail.form || '{}') : null
  const isNewUserTicket = detail?.type === 'permission_grant' && detailForm?.mode === 'new_user'
  const canViewCredential = detail && isApplicant && detail.status === 'succeeded' && isNewUserTicket && !detail.credential_viewed_at

  // ============ 渲染 ============
  const renderStatusTag = (status: TicketStatus) => {
    const meta = statusMeta[status]
    return <LabelTag text={meta.label} color={meta.color} />
  }

  const renderTypeTag = (type: TicketType) => {
    const meta = typeMeta[type]
    return <LabelTag text={meta.label} color={meta.color} />
  }

  const renderDetailForm = () => {
    if (!detail || !detailForm) return null
    if (detail.type === 'topic_create') {
      // 批量导入工单：items 里是全部申请的 topic
      const items = (detailForm as { items?: TopicCreateItem[] }).items
      if (items?.length) {
        return (
          <table className="bento-table">
            <thead>
              <tr>
                <th style={{ width: 48 }}>#</th>
                <th>Topic 名称</th>
                <th style={{ width: 56 }}>分区</th>
                <th style={{ width: 56 }}>副本</th>
                <th style={{ width: 72 }}>保留天数</th>
                <th>业务说明</th>
              </tr>
            </thead>
            <tbody>
              {items.map((it, i) => (
                <tr key={i}>
                  <td>{i + 1}</td>
                  <td style={{ wordBreak: 'break-all' }}><code>{it.topic_name}</code></td>
                  <td>{it.partitions}</td>
                  <td>{it.replication_factor}</td>
                  <td>{it.retention_days > 0 ? `${it.retention_days} 天` : '默认'}</td>
                  <td style={{ wordBreak: 'break-word' }}>{it.business_desc}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )
      }
      const form = detailForm as TopicCreateForm
      return (
        <table className="bento-table">
          <tbody>
            <tr><td style={{ width: 140, color: 'var(--text-muted)' }}>Topic 名称</td><td><code>{form.topic_name}</code></td></tr>
            <tr><td style={{ color: 'var(--text-muted)' }}>分区 / 副本</td><td>{form.partitions} / {form.replication_factor}</td></tr>
            <tr><td style={{ color: 'var(--text-muted)' }}>保留天数</td><td>{form.retention_days > 0 ? `${form.retention_days} 天` : '集群默认'}</td></tr>
            <tr><td style={{ color: 'var(--text-muted)' }}>业务说明</td><td>{form.business_desc}</td></tr>
          </tbody>
        </table>
      )
    }
    const form = detailForm as PermissionGrantForm
    const aclRules = expandACLPreview(form)
    return (
      <div>
        <table className="bento-table" style={{ marginBottom: 16 }}>
          <tbody>
            <tr><td style={{ width: 140, color: 'var(--text-muted)' }}>SCRAM 用户</td><td><code>{form.username}</code>（{form.mode === 'new_user' ? '新建用户' : '已有用户'}{form.mode === 'new_user' ? `，${form.mechanism}` : ''}）</td></tr>
            <tr><td style={{ color: 'var(--text-muted)' }}>权限模板</td><td>{permissionLabels[form.permission] || form.permission}</td></tr>
            <tr><td style={{ color: 'var(--text-muted)' }}>Topic 匹配</td><td>{form.topic_pattern === 'prefixed' ? '前缀匹配' : '精确匹配'}：{form.topics?.join('、')}</td></tr>
            {form.permission !== 'write' && (
              <tr><td style={{ color: 'var(--text-muted)' }}>消费组前缀</td><td><code>{form.group_prefix}</code>{form.group_prefix === '*' ? '（通配所有组）' : '（前缀匹配）'}</td></tr>
            )}
            {form.extras?.length > 0 && (
              <tr><td style={{ color: 'var(--text-muted)' }}>增强选项</td><td>{form.extras.includes('describeconfigs') ? '允许查看配置；' : ''}{form.extras.includes('group_describe') ? '允许自查消费位点' : ''}</td></tr>
            )}
          </tbody>
        </table>
        <div style={{ fontSize: 13, color: 'var(--text-muted)', marginBottom: 8 }}>将创建的 ACL（{aclRules.length} 条）：</div>
        <table className="bento-table">
          <thead>
            <tr><th>资源类型</th><th>资源名称</th><th>匹配</th><th>操作</th><th>权限</th><th>HOST</th></tr>
          </thead>
          <tbody>
            {aclRules.map((r, i) => (
              <tr key={i}>
                <td style={{ textAlign: 'center' }}>{r.resource_type.toUpperCase()}</td>
                <td><code>{r.resource_name}</code></td>
                <td style={{ textAlign: 'center' }}>{r.resource_pattern.toUpperCase()}</td>
                <td style={{ textAlign: 'center' }}>{r.operation.toUpperCase()}</td>
                <td style={{ textAlign: 'center' }}>ALLOW</td>
                <td style={{ textAlign: 'center' }}>*</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    )
  }

  const renderResult = () => {
    if (!detail) return null
    if (detail.status === 'failed' && detail.error_msg) {
      return (
        <div className="bento-card" style={{ marginBottom: 16 }}>
          <div className="bento-card-inner">
            <div style={{ fontWeight: 600, marginBottom: 8, color: 'var(--status-danger, #dc2626)' }}>执行失败</div>
            <div style={{ fontSize: 13, color: 'var(--text-muted)' }}>{detail.error_msg}</div>
          </div>
        </div>
      )
    }
    if ((detail.status === 'succeeded' || detail.status === 'failed') && detail.result) {
      try {
        const result = JSON.parse(detail.result) as TicketExecuteResult
        if (!result.created?.length && !result.skipped?.length) return null
        return (
          <div className="bento-card" style={{ marginBottom: 16 }}>
            <div className="bento-card-inner">
              <div style={{ fontWeight: 600, marginBottom: 8 }}>
                {detail.status === 'succeeded' ? '执行结果' : '部分执行结果（失败项可重试，已创建的自动跳过）'}
              </div>
              {result.created?.map((item, i) => (
                <div key={i} style={{ fontSize: 13, color: 'var(--text-secondary)' }}>✓ 已创建：{item}</div>
              ))}
              {result.skipped?.map((item, i) => (
                <div key={i} style={{ fontSize: 13, color: 'var(--text-muted)' }}>○ 跳过：{item}</div>
              ))}
            </div>
          </div>
        )
      } catch { /* 结果解析失败忽略 */ }
    }
    return null
  }

  return (
    <div>
      <div className="page-header">
        <div className="page-header-row">
          <div>
            <h1>工单中心</h1>
            <div className="page-accent-line" />
          </div>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => navigate('/tickets/create')}>
            新建工单
          </Button>
        </div>
      </div>

      {/* 统计卡片 */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 16, marginBottom: 20 }}>
        <StatCard label="PENDING" value={stats.pending || 0} subtitle="待审批" />
        <StatCard label="SUCCEEDED" value={stats.succeeded || 0} subtitle="已完成" />
        <StatCard label="FAILED" value={stats.failed || 0} subtitle="执行失败" />
        <StatCard label="REJECTED" value={stats.rejected || 0} subtitle="已驳回" />
      </div>

      {/* 范围 Tab + 筛选 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12, flexWrap: 'wrap', gap: 12 }}>
        <Tabs
          activeKey={scope}
          onChange={(key) => { setScope(key as 'mine' | 'pending' | 'all'); setPage(1) }}
          items={[
            { key: 'mine', label: '我的工单' },
            ...(isAdmin ? [{ key: 'pending', label: '待我审批' }] : []),
            ...(isAdmin ? [{ key: 'all', label: '全部工单' }] : []),
          ]}
          style={{ marginBottom: 0 }}
        />
        <div style={{ display: 'flex', gap: 12 }}>
          <Select
            style={{ width: 160 }}
            allowClear
            placeholder="按状态筛选"
            value={statusFilter || undefined}
            onChange={(v) => { setStatusFilter(v || ''); setPage(1) }}
            options={Object.entries(statusMeta).map(([value, meta]) => ({ value, label: meta.label }))}
          />
          <Button icon={<ReloadOutlined />} onClick={refreshAll}>刷新</Button>
        </div>
      </div>

      {/* 工单表 */}
      <div className="bento-card" style={{ marginBottom: 20 }}>
        <div className="bento-card-inner" style={{ padding: 0 }}>
          <table className="bento-table">
            <thead>
              <tr>
                <th style={{ textAlign: 'center' }}>工单号</th>
                <th style={{ textAlign: 'center' }}>类型</th>
                <th>标题</th>
                <th style={{ textAlign: 'center' }}>集群</th>
                <th style={{ textAlign: 'center' }}>状态</th>
                <th style={{ textAlign: 'center' }}>申请时间</th>
                <th style={{ textAlign: 'right' }}>操作</th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                <tr><td colSpan={7} style={{ textAlign: 'center', padding: 40 }}>加载中...</td></tr>
              ) : tickets.length === 0 ? (
                <tr><td colSpan={7} style={{ textAlign: 'center', padding: 40 }}>暂无工单</td></tr>
              ) : (
                tickets.map(t => (
                  <tr key={t.ticket_id}>
                    <td style={{ textAlign: 'center' }}><code>{t.ticket_no}</code></td>
                    <td style={{ textAlign: 'center' }}>{renderTypeTag(t.type)}</td>
                    <td>{t.title}</td>
                    <td style={{ textAlign: 'center' }}>{clusterName(t.cluster_id)}</td>
                    <td style={{ textAlign: 'center' }}>{renderStatusTag(t.status)}</td>
                    <td style={{ textAlign: 'center' }}>{new Date(t.applied_at).toLocaleString()}</td>
                    <td style={{ textAlign: 'right' }}>
                      <Button size="small" onClick={() => openDetail(t.ticket_id)}>详情</Button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* 分页 */}
      {total > pageSize && (
        <div style={{ display: 'flex', justifyContent: 'center', gap: 8 }}>
          <Button disabled={page <= 1} onClick={() => setPage(p => p - 1)}>上一页</Button>
          <span style={{ lineHeight: '32px', fontFamily: 'var(--font-mono)' }}>{page} / {Math.ceil(total / pageSize)}</span>
          <Button disabled={page * pageSize >= total} onClick={() => setPage(p => p + 1)}>下一页</Button>
        </div>
      )}

      {/* 工单详情 Drawer */}
      <Drawer
        title={detail ? <span>{detail.ticket_no} <span style={{ marginLeft: 8 }}>{renderStatusTag(detail.status)}</span></span> : '工单详情'}
        open={!!detail}
        onClose={() => setDetail(null)}
        width={640}
        footer={detail && (canApprove || canCancel || canRetry || canViewCredential) ? (
          <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
            {canCancel && <Button onClick={() => handleCancel(detail.ticket_id)} loading={actionLoading}>撤回</Button>}
            {canRetry && <Button onClick={() => handleRetry(detail.ticket_id)} loading={actionLoading}>重试</Button>}
            {canViewCredential && <Button type="primary" ghost onClick={() => handleTakeCredential(detail.ticket_id)} loading={actionLoading}>查看凭证</Button>}
            {canApprove && <Button danger onClick={() => setRejectModal({ open: true, remark: '' })} loading={actionLoading}>驳回</Button>}
            {canApprove && <Button type="primary" onClick={() => handleApprove(detail.ticket_id)} loading={actionLoading}>审批通过</Button>}
          </div>
        ) : null}
      >
        {detail && (
          <div>
            {/* 基本信息 */}
            <table className="bento-table" style={{ marginBottom: 16 }}>
              <tbody>
                <tr><td style={{ width: 100, color: 'var(--text-muted)' }}>类型</td><td>{renderTypeTag(detail.type)}</td></tr>
                <tr><td style={{ color: 'var(--text-muted)' }}>集群</td><td>{clusterName(detail.cluster_id)}</td></tr>
                <tr><td style={{ color: 'var(--text-muted)' }}>标题</td><td>{detail.title}</td></tr>
                <tr>
                  <td style={{ color: 'var(--text-muted)' }}>指定审批人</td>
                  <td>
                    {detail.designated_approver_name || '-'}
                    <span style={{ fontSize: 12, color: 'var(--text-muted)', marginLeft: 8 }}>
                      （规则：优先该集群的集群管理员，无则超级管理员兜底；超级管理员随时可审）
                    </span>
                  </td>
                </tr>
                <tr><td style={{ color: 'var(--text-muted)' }}>申请人</td><td>{detail.applicant_name || `用户#${detail.applicant_id}`}</td></tr>
                <tr><td style={{ color: 'var(--text-muted)' }}>申请时间</td><td>{new Date(detail.applied_at).toLocaleString()}</td></tr>
                {detail.approved_at && (
                  <tr><td style={{ color: 'var(--text-muted)' }}>审批时间</td><td>{new Date(detail.approved_at).toLocaleString()}{detail.approver_name ? `（${detail.approver_name}）` : ''}</td></tr>
                )}
                {detail.completed_at && (
                  <tr><td style={{ color: 'var(--text-muted)' }}>完成时间</td><td>{new Date(detail.completed_at).toLocaleString()}</td></tr>
                )}
                {detail.approve_remark && (
                  <tr><td style={{ color: 'var(--text-muted)' }}>审批备注</td><td>{detail.approve_remark}</td></tr>
                )}
              </tbody>
            </table>

            {/* 表单内容 */}
            <div style={{ fontWeight: 600, marginBottom: 8 }}>申请内容</div>
            {renderDetailForm()}

            {/* 执行结果 / 失败信息 */}
            {renderResult()}

            {/* 凭证提示 */}
            {detail.status === 'succeeded' && isNewUserTicket && (
              <div className="bento-card">
                <div className="bento-card-inner">
                  {detail.credential_viewed_at ? (
                    <span style={{ fontSize: 13, color: 'var(--text-muted)' }}>新建用户的密码已查看过，出于安全考虑仅展示一次，如需重置请联系管理员。</span>
                  ) : (
                    isApplicant
                      ? <span style={{ fontSize: 13 }}>新建 SCRAM 用户的密码可查看一次，请点击底部「查看凭证」按钮并妥善保存。</span>
                      : <span style={{ fontSize: 13, color: 'var(--text-muted)' }}>该工单包含新建用户凭证，仅申请人本人可查看。</span>
                  )}
                </div>
              </div>
            )}
          </div>
        )}
      </Drawer>

      {/* 驳回弹窗 */}
      <Modal
        title="驳回工单"
        open={rejectModal.open}
        onCancel={() => setRejectModal({ open: false, remark: '' })}
        onOk={() => detail && handleReject(detail.ticket_id)}
        confirmLoading={actionLoading}
        okText="确认驳回"
        okButtonProps={{ danger: true }}
        cancelText="取消"
      >
        <Input.TextArea
          rows={3}
          placeholder="请填写驳回原因（必填）"
          value={rejectModal.remark}
          onChange={e => setRejectModal({ open: true, remark: e.target.value })}
        />
      </Modal>

      {/* 凭证弹窗（一次性） */}
      <Modal
        title="SCRAM 用户凭证（仅显示一次）"
        open={credentialModal.open}
        onCancel={() => setCredentialModal({ open: false, data: null })}
        footer={<Button type="primary" onClick={() => setCredentialModal({ open: false, data: null })}>我已保存</Button>}
      >
        {credentialModal.data && (
          <div>
            <div style={{ marginBottom: 12, color: 'var(--status-danger, #dc2626)', fontSize: 13 }}>
              密码关闭弹窗后无法再次查看，请立即保存到安全的地方。
            </div>
            <table className="bento-table">
              <tbody>
                <tr><td style={{ width: 100, color: 'var(--text-muted)' }}>用户名</td><td><code>{credentialModal.data.username}</code></td></tr>
                <tr><td style={{ color: 'var(--text-muted)' }}>密码</td><td><code style={{ userSelect: 'all' }}>{credentialModal.data.password}</code></td></tr>
                <tr><td style={{ color: 'var(--text-muted)' }}>机制</td><td>{credentialModal.data.mechanism}</td></tr>
              </tbody>
            </table>
          </div>
        )}
      </Modal>
    </div>
  )
}

export default TicketCenter
