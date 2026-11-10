import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Form, Input, InputNumber, Radio, Select, Upload, message } from 'antd'
import { ArrowLeftOutlined, ClusterOutlined, KeyOutlined, DownloadOutlined, UploadOutlined, DeleteOutlined } from '@ant-design/icons'
import { ticketService, TicketType, TopicCreateForm, PermissionGrantForm, expandACLPreview } from '../services/ticket'
import { topicService } from '../services/topic'
import { clusterAPI, ClusterOption } from '../services/cluster'
import { scramUserService, ScramUser } from '../services/scramUser'
import { TOPIC_NAME_REGEX, MAX_TOPIC_BATCH_ROWS, TopicBatchRow, parseTopicBatchCSV, downloadTopicTemplate, readCSVFileAsText } from '../utils/topicBatch'
import { LabelTag } from '../components/bento'

const TicketCreate: React.FC = () => {
  const navigate = useNavigate()
  const [form] = Form.useForm()

  const [type, setType] = useState<TicketType | null>(null)
  const [clusters, setClusters] = useState<ClusterOption[]>([])
  const [clusterId, setClusterId] = useState<number | null>(null)
  const [scramUsers, setScramUsers] = useState<ScramUser[]>([])
  const [submitting, setSubmitting] = useState(false)

  // topic 工单填写模式：单个申请 / 批量导入
  const [topicMode, setTopicMode] = useState<'single' | 'batch'>('single')
  const [batchRows, setBatchRows] = useState<TopicBatchRow[]>([])
  const [batchChecking, setBatchChecking] = useState(false)

  // 表单相关状态（ACL 预览需要实时值）
  const [mode, setMode] = useState<'new_user' | 'existing_user'>('new_user')
  const [permission, setPermission] = useState<'read' | 'write' | 'read_write'>('read_write')
  const [username, setUsername] = useState('')
  const [topics, setTopics] = useState<string[]>([])
  const [topicPattern, setTopicPattern] = useState<'literal' | 'prefixed'>('literal')
  const [groupPrefix, setGroupPrefix] = useState('')
  const [groupPrefixTouched, setGroupPrefixTouched] = useState(false)
  const [extras, setExtras] = useState<string[]>([])

  // 拉取集群列表（options 接口不做权限过滤，所有登录用户可见）
  useEffect(() => {
    clusterAPI.options().then(setClusters).catch(() => message.error('获取集群列表失败'))
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
    setTopicMode('single')
    setBatchRows([])
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

  // ============ 批量导入 ============
  // 集群校验：调后端一次 ListTopics，返回 topic_name -> 错误信息 映射
  //（已存在 / '.'与'_' 命名冲突）。接口失败不阻断流程，提交时后端会再校验一次
  const fetchClusterErrors = async (rows: TopicBatchRow[], cid: number): Promise<Record<string, string>> => {
    const names = rows.filter(r => r.errors.length === 0).map(r => r.topic_name)
    if (names.length === 0) return {}
    try {
      const results = await topicService.checkNames(cid, names)
      const errorMap: Record<string, string> = {}
      for (const [name, check] of Object.entries(results)) {
        if (check.exists) {
          errorMap[name] = '集群中已存在同名 topic'
        } else if (check.conflict_with) {
          errorMap[name] = `与 ${check.conflict_with} 仅 '.'/'_' 之差（指标命名冲突）`
        }
      }
      return errorMap
    } catch {
      message.error('集群 topic 名称校验失败，请手动确认或稍后重试')
      return {}
    }
  }

  const handleCSVFile = async (file: File) => {
    const text = await readCSVFileAsText(file)
    const { rows, error } = parseTopicBatchCSV(text)
    if (error) {
      message.error(error)
      return
    }
    setBatchRows(rows)
    if (!clusterId) {
      return
    }
    // 函数式合并 cluster_error：校验期间用户删除的行不会被复活
    setBatchChecking(true)
    const errorMap = await fetchClusterErrors(rows, clusterId)
    setBatchRows(prev => prev.map(r => ({ ...r, cluster_error: errorMap[r.topic_name] })))
    setBatchChecking(false)
    const invalid = rows.filter(r => r.errors.length > 0 || errorMap[r.topic_name]).length
    if (invalid > 0) {
      message.warning(`解析完成：${rows.length} 行，其中 ${invalid} 行校验不通过（见预览标红行）`)
    } else {
      message.success(`解析完成：${rows.length} 行，全部校验通过`)
    }
  }

  // 切换集群后重新校验已导入的行（格式校验与集群无关，只更新 cluster_error）
  const recheckBatchRows = async (cid: number) => {
    if (batchRows.length === 0) return
    setBatchChecking(true)
    const errorMap = await fetchClusterErrors(batchRows, cid)
    setBatchRows(prev => prev.map(r => ({ ...r, cluster_error: errorMap[r.topic_name] })))
    setBatchChecking(false)
  }

  // 删除预览中的行（写多了 / 已存在时直接页面删除，无需改 CSV 重传）
  const removeBatchRow = (line: number) => {
    setBatchRows(prev => prev.filter(r => r.line !== line))
  }

  const batchInvalidCount = batchRows.filter(r => r.errors.length > 0 || r.cluster_error).length

  // 批量导入合并为一张工单提交（后端整单校验重名/命名冲突，审批通过后逐个幂等创建）
  const handleBatchSubmit = async () => {
    if (!clusterId || batchInvalidCount > 0) return
    const validRows = batchRows.filter(r => r.errors.length === 0 && !r.cluster_error)
    setSubmitting(true)
    try {
      const t = await ticketService.submit({
        type: 'topic_create',
        cluster_id: clusterId,
        form: {
          items: validRows.map(r => ({
            topic_name: r.topic_name,
            partitions: r.partitions,
            replication_factor: r.replication_factor,
            retention_days: r.retention_days,
            business_desc: r.business_desc,
          })),
        },
      })
      message.success(`工单 ${t.ticket_no} 提交成功（共 ${validRows.length} 个 Topic），等待审批`)
      navigate('/tickets')
    } catch (error: any) {
      message.error(error?.response?.data?.error || '提交失败')
    } finally {
      setSubmitting(false)
    }
  }

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
          retention_days: values.retention_days ?? 3,
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
            在目标集群创建新 Topic（支持批量导入），申请人信息将自动写入 Topic 描述
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
    <Form form={form} layout="vertical" style={{ maxWidth: 860 }}>
      <Form.Item label="填写方式">
        <Radio.Group value={topicMode} onChange={e => setTopicMode(e.target.value)}>
          <Radio.Button value="single">单个申请</Radio.Button>
          <Radio.Button value="batch">批量导入</Radio.Button>
        </Radio.Group>
      </Form.Item>

      {topicMode === 'single' ? (
        <>
          <Form.Item
            name="topic_name"
            label="Topic 名称"
            rules={[
              { required: true, message: '请输入 topic 名称' },
              { pattern: TOPIC_NAME_REGEX, message: '仅允许字母、数字、.、_、-' },
            ]}
            extra="建议按 团队.应用.事件 分层命名；提交时将校验集群内是否重名及 '.' 与 '_' 的命名冲突"
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
            <Form.Item name="retention_days" label="保留天数" initialValue={3} extra="0 = 集群默认，默认 3 天">
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
        </>
      ) : (
        <div>
          <div style={{ display: 'flex', gap: 12, marginBottom: 12 }}>
            <Button icon={<DownloadOutlined />} onClick={downloadTopicTemplate}>下载 CSV 模板</Button>
            <Upload accept=".csv" showUploadList={false} beforeUpload={(file) => { handleCSVFile(file); return false }}>
              <Button icon={<UploadOutlined />}>上传 CSV 文件</Button>
            </Upload>
          </div>
          <div style={{ fontSize: 13, color: 'var(--text-muted)', marginBottom: 12 }}>
            固定 5 列：topic_name, partitions, replication_factor, retention_days, business_desc；最多 {MAX_TOPIC_BATCH_ROWS} 行；
            保留天数默认 3（0 = 集群默认）；业务说明含逗号时请用英文引号包裹。全部行将合并为一张工单，审批通过后一次性创建。
            上传后将自动校验集群内重名与命名冲突，写多的行可直接在预览中删除。
          </div>

          {batchRows.length > 0 && (
            <div className="bento-card" style={{ marginBottom: 16 }}>
              <div className="bento-card-inner">
                <div style={{ fontWeight: 600, marginBottom: 8 }}>
                  预览（共 {batchRows.length} 行，{batchInvalidCount > 0 ? <span style={{ color: 'var(--status-danger, #dc2626)' }}>{batchInvalidCount} 行校验不通过</span> : '全部校验通过'}{batchChecking ? '，正在校验集群...' : ''}）
                </div>
                <div style={{ maxHeight: 320, overflow: 'auto' }}>
                  <table className="bento-table" style={{ width: '100%', tableLayout: 'fixed' }}>
                    <thead>
                      <tr>
                        <th style={{ width: 52 }}>行号</th>
                        <th style={{ width: 190 }}>Topic 名称</th>
                        <th style={{ width: 56 }}>分区</th>
                        <th style={{ width: 56 }}>副本</th>
                        <th style={{ width: 72 }}>保留天数</th>
                        <th>业务说明</th>
                        <th style={{ width: 170 }}>校验</th>
                        <th style={{ width: 56 }}>操作</th>
                      </tr>
                    </thead>
                    <tbody>
                      {batchRows.map(r => (
                        <tr key={r.line} style={r.errors.length > 0 || r.cluster_error ? { background: 'rgba(220, 38, 38, 0.05)' } : undefined}>
                          <td>{r.line}</td>
                          <td style={{ wordBreak: 'break-all' }}><code>{r.topic_name}</code></td>
                          <td>{r.partitions}</td>
                          <td>{r.replication_factor}</td>
                          <td>{r.retention_days}</td>
                          <td style={{ wordBreak: 'break-word' }}>{r.business_desc}</td>
                          <td style={{ wordBreak: 'break-word', color: r.errors.length > 0 || r.cluster_error ? 'var(--status-danger, #dc2626)' : 'var(--status-success, #16a34a)', fontSize: 13 }}>
                            {r.errors.length > 0 ? r.errors.join('；') : r.cluster_error || '通过'}
                          </td>
                          <td>
                            <Button type="text" size="small" danger icon={<DeleteOutlined />} onClick={() => removeBatchRow(r.line)} />
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            </div>
          )}
        </div>
      )}
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
                onChange={(v) => { setClusterId(v); form.resetFields(['mechanism']); recheckBatchRows(v) }}
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
          {type === 'topic_create' && topicMode === 'batch' ? (
            <Button
              type="primary"
              size="large"
              loading={submitting}
              disabled={batchRows.length === 0 || batchInvalidCount > 0 || batchChecking}
              onClick={handleBatchSubmit}
            >
              提交工单（共 {batchRows.length - batchInvalidCount} 个 Topic，合并为一张工单）
            </Button>
          ) : (
            <Button type="primary" size="large" loading={submitting} onClick={handleSubmit}>
              提交工单
            </Button>
          )}
          <Button size="large" onClick={() => navigate('/tickets')}>取消</Button>
        </div>
      )}
    </div>
  )
}

export default TicketCreate
