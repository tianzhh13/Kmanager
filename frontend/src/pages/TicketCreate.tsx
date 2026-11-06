import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Form, Input, InputNumber, Radio, Select, message } from 'antd'
import { ArrowLeftOutlined, ClusterOutlined, KeyOutlined } from '@ant-design/icons'
import { ticketService, TicketType, TopicCreateForm, PermissionGrantForm, expandACLPreview } from '../services/ticket'
import { clusterAPI, Cluster } from '../services/cluster'
import { scramUserService, ScramUser } from '../services/scramUser'
import { LabelTag } from '../components/bento'

const TicketCreate: React.FC = () => {
  const navigate = useNavigate()
  const [form] = Form.useForm()

  const [type, setType] = useState<TicketType | null>(null)
  const [clusters, setClusters] = useState<Cluster[]>([])
  const [clusterId, setClusterId] = useState<number | null>(null)
  const [scramUsers, setScramUsers] = useState<ScramUser[]>([])
  const [submitting, setSubmitting] = useState(false)

  // 表单相关状态（ACL 预览需要实时值）
  const [mode, setMode] = useState<'new_user' | 'existing_user'>('new_user')
  const [permission, setPermission] = useState<'read' | 'write' | 'read_write'>('read_write')
  const [username, setUsername] = useState('')
  const [topics, setTopics] = useState<string[]>([])
  const [topicPattern, setTopicPattern] = useState<'literal' | 'prefixed'>('literal')
  const [groupPrefix, setGroupPrefix] = useState('')
  const [groupPrefixTouched, setGroupPrefixTouched] = useState(false)
  const [extras, setExtras] = useState<string[]>([])

  // 拉取集群列表
  useEffect(() => {
    clusterAPI.list(1, 200).then((res: { data: Cluster[] }) => setClusters(res.data || [])).catch(() => message.error('获取集群列表失败'))
  }, [])

  const selectedCluster = clusters.find(c => c.cluster_id === clusterId)
  // 权限工单仅适用于 SCRAM 集群（PLAINTEXT 无认证、Kerberos 用户在 KDC 管理）
  const availableClusters = type === 'permission_grant' ? clusters.filter(c => c.auth_type === 'scram') : clusters

  // 权限工单：加载该集群已有 SCRAM 用户
  useEffect(() => {
    if (type !== 'permission_grant' || !clusterId) {
      setScramUsers([])
      return
    }
    scramUserService.list(clusterId, 1, 200)
      .then((res: { data: ScramUser[] }) => setScramUsers(res.data || []))
      .catch(() => setScramUsers([]))
  }, [type, clusterId])

  // 权限表单：消费组前缀默认 = 用户名-（用户手动修改后不再跟随）
  useEffect(() => {
    if (groupPrefixTouched) return
    const next = username ? `${username}-` : ''
    form.setFieldValue('group_prefix', next)
    setGroupPrefix(next)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [username])

  // 切换类型时重置表单，避免字段串值
  const handleTypeChange = (t: TicketType) => {
    setType(t)
    setClusterId(null)
    form.resetFields()
    setMode('new_user')
    setPermission('read_write')
    setUsername('')
    setTopics([])
    setTopicPattern('literal')
    setGroupPrefix('')
    setGroupPrefixTouched(false)
    setExtras([])
  }

  // 提交前 ACL 预览
  const aclPreview = useMemo(() => {
    if (type !== 'permission_grant' || topics.length === 0) return []
    return expandACLPreview({
      mode,
      username: username || 'user',
      mechanism: 'SCRAM-SHA-256',
      permission,
      topics,
      topic_pattern: topicPattern,
      group_prefix: groupPrefix || 'group-',
      extras,
    })
  }, [type, mode, username, permission, topics, topicPattern, groupPrefix, extras])

  // ============ 提交 ============
  const handleSubmit = async () => {
    if (!type || !clusterId) {
      message.warning('请选择工单类型和目标集群')
      return
    }
    let formPayload: TopicCreateForm | PermissionGrantForm
    try {
      const values = await form.validateFields()
      if (type === 'topic_create') {
        formPayload = {
          topic_name: values.topic_name,
          partitions: values.partitions,
          replication_factor: values.replication_factor,
          retention_days: values.retention_days || 0,
          business_desc: values.business_desc,
        }
      } else {
        formPayload = {
          mode,
          username: values.username,
          mechanism: mode === 'new_user' ? values.mechanism : '',
          permission,
          topics,
          topic_pattern: topicPattern,
          group_prefix: permission !== 'write' ? (values.group_prefix || '*') : '',
          extras,
        }
      }
    } catch {
      return
    }

    setSubmitting(true)
    try {
      const t = await ticketService.submit({ type, cluster_id: clusterId, form: formPayload })
      message.success(`工单 ${t.ticket_no} 提交成功，等待审批`)
      navigate('/tickets')
    } catch (error: any) {
      message.error(error?.response?.data?.error || '提交失败')
    } finally {
      setSubmitting(false)
    }
  }

  // ============ 类型选择卡片 ============
  const renderTypeSelection = () => (
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: 16, maxWidth: 720 }}>
      <div
        className="bento-card"
        style={{ cursor: 'pointer', border: type === 'topic_create' ? '2px solid var(--brand, #f97316)' : undefined }}
        onClick={() => handleTypeChange('topic_create')}
      >
        <div className="bento-card-inner">
          <ClusterOutlined style={{ fontSize: 28, color: 'var(--brand, #f97316)' }} />
          <div style={{ fontWeight: 600, fontSize: 15, marginTop: 12 }}>申请 Topic</div>
          <div style={{ fontSize: 13, color: 'var(--text-muted)', marginTop: 4 }}>
            在目标集群创建新 Topic，申请人信息将自动写入 Topic 描述
          </div>
        </div>
      </div>
      <div
        className="bento-card"
        style={{ cursor: 'pointer', border: type === 'permission_grant' ? '2px solid var(--brand, #f97316)' : undefined }}
        onClick={() => handleTypeChange('permission_grant')}
      >
        <div className="bento-card-inner">
          <KeyOutlined style={{ fontSize: 28, color: 'var(--brand, #f97316)' }} />
          <div style={{ fontWeight: 600, fontSize: 15, marginTop: 12 }}>申请权限</div>
          <div style={{ fontSize: 13, color: 'var(--text-muted)', marginTop: 4 }}>
            新建 SCRAM 用户或为已有用户开通 Topic / 消费组读写权限（仅 SCRAM 集群）
          </div>
        </div>
      </div>
    </div>
  )

  // ============ 申请 Topic 表单 ============
  const renderTopicForm = () => (
    <Form form={form} layout="vertical" style={{ maxWidth: 720 }}>
      <Form.Item
        name="topic_name"
        label="Topic 名称"
        rules={[
          { required: true, message: '请输入 topic 名称' },
          { pattern: /^[a-zA-Z0-9._-]{1,249}$/, message: '仅允许字母、数字、.、_、-' },
        ]}
        extra="建议按 团队.应用.事件 分层命名，如 trade.ods.order-created"
      >
        <Input placeholder="例如：trade.ods.order-created" />
      </Form.Item>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 12 }}>
        <Form.Item name="partitions" label="分区数" initialValue={6} rules={[{ required: true, message: '请输入分区数' }]}>
          <InputNumber min={1} max={200} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item name="replication_factor" label="副本数" initialValue={3} rules={[{ required: true, message: '请输入副本数' }]}>
          <InputNumber min={1} max={5} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item name="retention_days" label="保留天数" initialValue={0} extra="0 = 集群默认">
          <InputNumber min={0} max={3650} style={{ width: '100%' }} />
        </Form.Item>
      </div>
      <Form.Item
        name="business_desc"
        label="业务说明"
        rules={[{ required: true, message: '请填写业务说明' }]}
        extra="申请人信息由平台自动拼接进 Topic 描述，无需手写"
      >
        <Input.TextArea rows={3} placeholder="例如：订单创建事件流，供风控和实时大盘消费" />
      </Form.Item>
    </Form>
  )

  // ============ 申请权限表单 ============
  const renderPermissionForm = () => (
    <Form form={form} layout="vertical" style={{ maxWidth: 720 }}>
      <Form.Item label="授权对象" required>
        <Radio.Group
          value={mode}
          onChange={e => {
            setMode(e.target.value)
            form.setFieldValue('username', undefined)
            setUsername('')
          }}
        >
          <Radio.Button value="new_user">新建 SCRAM 用户</Radio.Button>
          <Radio.Button value="existing_user">已有用户</Radio.Button>
        </Radio.Group>
      </Form.Item>

      {mode === 'new_user' ? (
        <>
          <Form.Item
            name="username"
            label="用户名"
            rules={[
              { required: true, message: '请输入用户名' },
              { pattern: /^[a-zA-Z0-9_-]{1,128}$/, message: '仅允许字母、数字、_、-' },
            ]}
            extra="密码将由平台自动生成，工单完成后可查看一次"
          >
            <Input placeholder="例如：risk-engine" onChange={e => setUsername(e.target.value)} />
          </Form.Item>
          <Form.Item
            name="mechanism"
            label="认证机制"
            initialValue={(selectedCluster?.sasl_mechanism as 'SCRAM-SHA-256' | 'SCRAM-SHA-512') || 'SCRAM-SHA-256'}
          >
            <Select
              options={[
                { value: 'SCRAM-SHA-256', label: 'SCRAM-SHA-256' },
                { value: 'SCRAM-SHA-512', label: 'SCRAM-SHA-512' },
              ]}
            />
          </Form.Item>
        </>
      ) : (
        <Form.Item name="username" label="选择用户" rules={[{ required: true, message: '请选择用户' }]}>
          <Select
            showSearch
            optionFilterProp="label"
            placeholder="选择该集群已有的 SCRAM 用户"
            options={scramUsers.map(u => ({ value: u.username, label: u.username }))}
            onChange={(v) => setUsername(v)}
          />
        </Form.Item>
      )}

      <Form.Item label="权限模板" required extra="describe 权限将随读写自动授予；消费场景需同时授权消费组">
        <Radio.Group value={permission} onChange={e => setPermission(e.target.value)}>
          <Radio.Button value="read">只读（消费者）</Radio.Button>
          <Radio.Button value="write">只写（生产者）</Radio.Button>
          <Radio.Button value="read_write">读写</Radio.Button>
        </Radio.Group>
      </Form.Item>

      <Form.Item label="Topic" required extra="输入 topic 名称后回车添加，可多个">
        <Select
          mode="tags"
          value={topics}
          onChange={setTopics}
          placeholder={topicPattern === 'prefixed' ? '例如：trade.ods. （前缀）' : '例如：trade.ods.order-created'}
          tokenSeparators={[',', ' ']}
        />
      </Form.Item>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: 12 }}>
        <Form.Item label="Topic 匹配方式">
          <Radio.Group value={topicPattern} onChange={e => setTopicPattern(e.target.value)}>
            <Radio.Button value="literal">精确匹配</Radio.Button>
            <Radio.Button value="prefixed">前缀匹配</Radio.Button>
          </Radio.Group>
        </Form.Item>
        {permission !== 'write' && (
          <Form.Item
            name="group_prefix"
            label="消费组前缀"
            rules={[{ required: true, message: '请输入消费组前缀（可填 * 通配）' }]}
            extra="该前缀下的组名均可使用，如 team-a-"
          >
            <Input
              placeholder="例如：team-a-（或 * 通配所有组）"
              onChange={e => { setGroupPrefix(e.target.value); setGroupPrefixTouched(true) }}
            />
          </Form.Item>
        )}
      </div>

      <Form.Item label="可选增强">
        <Select
          mode="multiple"
          value={extras}
          onChange={setExtras}
          placeholder="默认不勾选（最小权限）"
          options={[
            { value: 'describeconfigs', label: '允许查看 Topic 配置（describeconfigs）' },
            { value: 'group_describe', label: '允许自查消费位点（group describe）' },
          ]}
        />
      </Form.Item>

      {/* ACL 预览 */}
      {aclPreview.length > 0 && (
        <div className="bento-card">
          <div className="bento-card-inner">
            <div style={{ fontWeight: 600, marginBottom: 8 }}>将创建的 ACL 预览（{aclPreview.length} 条）</div>
            <table className="bento-table">
              <thead>
                <tr><th>资源类型</th><th>资源名称</th><th>匹配</th><th>操作</th><th>权限</th><th>HOST</th></tr>
              </thead>
              <tbody>
                {aclPreview.map((r, i) => (
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
        </div>
      )}
    </Form>
  )

  return (
    <div>
      <div className="page-header">
        <div className="page-header-row">
          <div>
            <h1>新建工单</h1>
            <div className="page-accent-line" />
          </div>
          <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/tickets')}>返回工单中心</Button>
        </div>
      </div>

      <div className="bento-card" style={{ marginBottom: 20 }}>
        <div className="bento-card-inner">
          {/* 第一步：类型 */}
          <div style={{ fontWeight: 600, marginBottom: 12 }}>
            {type ? <span>工单类型：<LabelTag text={type === 'topic_create' ? '申请 Topic' : '申请权限'} color="orange" /></span> : '① 选择工单类型'}
          </div>
          {renderTypeSelection()}

          {/* 第二步：集群 */}
          {type && (
            <div style={{ marginTop: 24 }}>
              <div style={{ fontWeight: 600, marginBottom: 12 }}>② 选择目标集群</div>
              {type === 'permission_grant' && clusters.length > 0 && availableClusters.length === 0 && (
                <div style={{ fontSize: 13, color: 'var(--status-danger, #dc2626)', marginBottom: 8 }}>
                  当前纳管的集群中没有 SCRAM 认证的集群，无法申请权限
                </div>
              )}
              <Select
                style={{ width: 360 }}
                placeholder="选择集群"
                value={clusterId}
                onChange={(v) => { setClusterId(v); form.resetFields(['mechanism']) }}
                options={availableClusters.map(c => ({
                  value: c.cluster_id,
                  label: `${c.cluster_name}（${c.auth_type}${c.sasl_mechanism ? ` / ${c.sasl_mechanism}` : ''}）`,
                }))}
              />
            </div>
          )}

          {/* 第三步：表单 */}
          {type && clusterId && (
            <div style={{ marginTop: 24 }}>
              <div style={{ fontWeight: 600, marginBottom: 12 }}>③ 填写申请内容</div>
              {type === 'topic_create' ? renderTopicForm() : renderPermissionForm()}
            </div>
          )}
        </div>
      </div>

      {/* 提交 */}
      {type && clusterId && (
        <div style={{ display: 'flex', gap: 12 }}>
          <Button type="primary" size="large" loading={submitting} onClick={handleSubmit}>
            提交工单
          </Button>
          <Button size="large" onClick={() => navigate('/tickets')}>取消</Button>
        </div>
      )}
    </div>
  )
}

export default TicketCreate
