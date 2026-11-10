import React, { useState, useEffect, useCallback } from 'react'
import { Select, InputNumber, Input, Button, Spin, message, Drawer, Radio, Empty, Tag, Pagination } from 'antd'
import { SearchOutlined, ReloadOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import { topicService, Topic } from '../../services/topic'
import { messageService, MessageRecord, MessageQueryType, QueryMessagesRequest } from '../../services/message'
import { SectionTitle } from '../../components/bento'

interface ClusterOption {
  cluster_id: number
  cluster_name: string
}

interface MessagesProps {
  cluster: ClusterOption
  initialTopic?: string | null
}

const QUERY_MODE_OPTIONS: { label: string; value: MessageQueryType }[] = [
  { label: '最新 N 条', value: 'latest' },
  { label: '最旧 N 条', value: 'earliest' },
  { label: '按 Offset 范围', value: 'offset' },
  { label: '按时间戳', value: 'timestamp' },
  { label: '按关键词过滤', value: 'key' },
]

// 后端 Value []byte 经 gin JSON 序列化为 base64 字符串
function decodeValue(b64: string): Uint8Array {
  try {
    const bin = atob(b64)
    const bytes = new Uint8Array(bin.length)
    for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i)
    return bytes
  } catch {
    // 不是 base64（可能是空值或已解码），当作 UTF-8 文本
    return new TextEncoder().encode(b64)
  }
}

function bytesToString(bytes: Uint8Array): string {
  try {
    return new TextDecoder('utf-8', { fatal: false }).decode(bytes)
  } catch {
    return ''
  }
}

function bytesToHex(bytes: Uint8Array, maxBytes = 1024): string {
  const slice = bytes.length > maxBytes ? bytes.slice(0, maxBytes) : bytes
  let hex = ''
  for (let i = 0; i < slice.length; i++) {
    hex += slice[i].toString(16).padStart(2, '0') + ' '
    if ((i + 1) % 16 === 0) hex += '\n'
  }
  if (bytes.length > maxBytes) hex += `\n... (${bytes.length - maxBytes} more bytes)`
  return hex.trim()
}

function tryFormatJson(str: string): { ok: boolean; text: string } {
  try {
    return { ok: true, text: JSON.stringify(JSON.parse(str), null, 2) }
  } catch {
    return { ok: false, text: str }
  }
}

const Messages: React.FC<MessagesProps> = ({ cluster, initialTopic }) => {
  const [topics, setTopics] = useState<Topic[]>([])
  const [topicLoading, setTopicLoading] = useState(false)
  const [selectedTopic, setSelectedTopic] = useState<string | null>(initialTopic || null)

  const [partition, setPartition] = useState<number>(0)
  const [partitionCount, setPartitionCount] = useState<number>(1)
  const [queryMode, setQueryMode] = useState<MessageQueryType>('latest')
  const [maxCount, setMaxCount] = useState<number>(100)
  const [startOffset, setStartOffset] = useState<number>(0)
  const [endOffset, setEndOffset] = useState<number>(100)
  const [timestamp, setTimestamp] = useState<string>(dayjs().subtract(1, 'hour').format('YYYY-MM-DDTHH:mm'))
  const [keyFilter, setKeyFilter] = useState<string>('')

  const [loading, setLoading] = useState(false)
  const [records, setRecords] = useState<MessageRecord[]>([])
  const [partitionOffsets, setPartitionOffsets] = useState<{ start: number; end: number } | null>(null)
  const [scanned, setScanned] = useState(0)

  // 分页（offset / timestamp 模式）：pageBase 是第一页实际起始 offset，翻页在其基础上推进
  const [page, setPage] = useState(1)
  const [pageBase, setPageBase] = useState<number | null>(null)
  const [pageTotal, setPageTotal] = useState(0)

  const [detailRecord, setDetailRecord] = useState<MessageRecord | null>(null)
  const [detailView, setDetailView] = useState<'json' | 'plain' | 'hex'>('json')

  // 加载 topic 列表
  const loadTopics = useCallback(async () => {
    if (!cluster?.cluster_id) return
    setTopicLoading(true)
    try {
      const res = await topicService.list(1, 500, cluster.cluster_id)
      const list = res.data || []
      setTopics(list)
      if (list.length > 0) {
        // 优先用 initialTopic，否则选第一个
        const target = initialTopic && list.find((t: Topic) => t.topic_name === initialTopic)
        setSelectedTopic(target ? target.topic_name : list[0].topic_name)
      }
    } catch {
      message.error('加载 Topic 列表失败')
    } finally {
      setTopicLoading(false)
    }
  }, [cluster?.cluster_id, initialTopic])

  useEffect(() => {
    loadTopics()
  }, [loadTopics])

  // 选中 topic 时，取其分区数
  useEffect(() => {
    const t = topics.find(t => t.topic_name === selectedTopic)
    if (t) {
      setPartitionCount(t.partitions || 1)
      if (partition >= t.partitions) setPartition(0)
    }
  }, [selectedTopic, topics])

  // 执行查询；pageNum > 1 时以第一页基准 offset 翻页（offset/timestamp 模式）
  const handleQuery = async (pageNum: number = 1) => {
    if (!selectedTopic || !cluster?.cluster_id) return
    setLoading(true)
    try {
      const pageSize = Math.min(maxCount, 500)
      const req: QueryMessagesRequest = {
        partition,
        query_type: queryMode,
        max_count: pageSize,
      }

      if (queryMode === 'offset') {
        if (pageNum === 1) {
          req.start_offset = startOffset
        } else {
          req.start_offset = (pageBase ?? startOffset) + (pageNum - 1) * pageSize
        }
        req.end_offset = endOffset
      } else if (queryMode === 'timestamp') {
        if (pageNum === 1) {
          const ts = dayjs(timestamp).valueOf()
          if (!ts || isNaN(ts)) {
            message.warning('请输入有效的时间')
            setLoading(false)
            return
          }
          req.timestamp = ts
        } else {
          // 翻页：时间戳已在第一页解析为 offset，改走 offset 语义往后翻
          req.query_type = 'offset'
          req.start_offset = (pageBase ?? 0) + (pageNum - 1) * pageSize
          req.end_offset = 0 // 0 = 到分区最新
        }
      } else if (queryMode === 'key') {
        req.key = keyFilter
      }

      const res = await messageService.query(selectedTopic, cluster.cluster_id, req)
      setRecords(res.data || [])
      setPartitionOffsets(res.partition_offsets)
      setScanned(res.scanned || 0)
      setPage(pageNum)

      // 第一页记录分页基准和总数
      if (pageNum === 1) {
        const from = res.fetched_from ?? 0
        setPageBase(from)
        if (queryMode === 'offset') {
          const end = res.partition_offsets?.end ?? 0
          const userEnd = endOffset > 0 ? Math.min(endOffset, end) : end
          setPageTotal(Math.max(0, userEnd - from))
        } else if (queryMode === 'timestamp') {
          setPageTotal(Math.max(0, (res.partition_offsets?.end ?? 0) - from))
        } else {
          setPageTotal(0)
        }
      }

      if ((res.data || []).length === 0) {
        if (queryMode === 'key' && (res.scanned || 0) > 0) {
          message.info(`已扫描最早的 ${res.scanned} 条消息，未找到包含该关键词的消息`)
        } else {
          message.info('该范围内没有消息')
        }
      }
    } catch (err: any) {
      const status = err?.response?.status
      const detail = err?.response?.data?.error || err?.message || '查询失败'
      if (status === 403) {
        message.error('无权限查询该 Topic 的消息')
      } else {
        message.error(detail)
      }
      setRecords([])
    } finally {
      setLoading(false)
    }
  }

  // 选中 topic 变化或首次加载时自动查询一次（latest 50 条）
  useEffect(() => {
    if (selectedTopic && cluster?.cluster_id && !loading) {
      handleQuery()
    }
  }, [selectedTopic]) // eslint-disable-line react-hooks/exhaustive-deps

  const gridCols = '100px 120px 160px 200px 1fr'

  const renderValuePreview = (record: MessageRecord): string => {
    const bytes = decodeValue(record.value)
    const str = bytesToString(bytes)
    if (!str) return '(empty)'
    // 尝试 JSON
    const j = tryFormatJson(str)
    if (j.ok) {
      const lines = j.text.split('\n').slice(0, 2).join(' ')
      return lines.length > 80 ? lines.slice(0, 80) + '...' : lines
    }
    return str.length > 80 ? str.slice(0, 80) + '...' : str
  }

  const renderDetailValue = (): string => {
    if (!detailRecord) return ''
    const bytes = decodeValue(detailRecord.value)
    if (detailView === 'hex') return bytesToHex(bytes)
    const str = bytesToString(bytes)
    if (detailView === 'plain') return str || '(empty)'
    // json
    const j = tryFormatJson(str)
    return j.text || '(empty)'
  }

  return (
    <div>
      {/* 查询条件 */}
      <div className="bento-card" style={{ padding: 20, marginBottom: 16 }}>
        <SectionTitle title="消息查询 — 按 Offset / 时间戳 / Key 查询消息，用于排障（只读）" />

        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 12, alignItems: 'center', marginTop: 16 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <span style={{ fontSize: 13, color: 'var(--text-2)', whiteSpace: 'nowrap' }}>Topic</span>
            <Select
              style={{ width: 260 }}
              value={selectedTopic || undefined}
              loading={topicLoading}
              onChange={setSelectedTopic}
              showSearch
              optionFilterProp="label"
              options={topics.map(t => ({ label: t.topic_name, value: t.topic_name }))}
              placeholder="选择 Topic"
            />
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <span style={{ fontSize: 13, color: 'var(--text-2)' }}>分区</span>
            <Select
              style={{ width: 90 }}
              value={partition}
              onChange={setPartition}
              options={Array.from({ length: partitionCount }, (_, i) => ({ label: String(i), value: i }))}
            />
          </div>

          <Select
            style={{ width: 150 }}
            value={queryMode}
            onChange={setQueryMode}
            options={QUERY_MODE_OPTIONS}
          />

          {queryMode === 'offset' && (
            <>
              <InputNumber
                style={{ width: 130 }}
                placeholder="起始 offset"
                value={startOffset}
                onChange={v => setStartOffset(v ?? 0)}
                min={0}
              />
              <span style={{ color: 'var(--text-3)' }}>~</span>
              <InputNumber
                style={{ width: 150 }}
                placeholder="结束 offset(0=最新)"
                value={endOffset}
                onChange={v => setEndOffset(v ?? 0)}
                min={0}
              />
            </>
          )}

          {queryMode === 'timestamp' && (
            <input
              type="datetime-local"
              value={timestamp}
              onChange={e => setTimestamp(e.target.value)}
              style={{
                padding: '6px 10px', borderRadius: 8, fontSize: 13,
                border: '1px solid var(--border)', background: 'var(--card)',
                color: 'var(--text-1)',
              }}
            />
          )}

          {queryMode === 'key' && (
            <Input
              style={{ width: 220 }}
              placeholder="关键词，匹配 Key 或消息内容"
              value={keyFilter}
              onChange={e => setKeyFilter(e.target.value)}
              prefix={<SearchOutlined />}
              allowClear
            />
          )}

          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <span style={{ fontSize: 13, color: 'var(--text-2)' }}>条数</span>
            <InputNumber
              style={{ width: 90 }}
              value={maxCount}
              onChange={v => setMaxCount(v ?? 100)}
              min={1}
              max={500}
            />
          </div>

          <Button type="primary" icon={<SearchOutlined />} loading={loading} onClick={() => handleQuery(1)}>
            查询
          </Button>
          <Button icon={<ReloadOutlined />} onClick={() => handleQuery(page)} disabled={loading}>刷新</Button>
        </div>

        {partitionOffsets && (
          <div style={{ marginTop: 12, fontSize: 12, color: 'var(--text-3)' }}>
            分区 {partition} offset 范围：{partitionOffsets.start} ~ {partitionOffsets.end}
            （可用消息 {Math.max(0, partitionOffsets.end - partitionOffsets.start)} 条）
          </div>
        )}
      </div>

      {/* 结果列表 */}
      <Spin spinning={loading}>
        {records.length > 0 ? (
          <>
            <div className="bento-table-header" style={{ gridTemplateColumns: gridCols }}>
              <span>分区</span>
              <span>Offset</span>
              <span>时间戳</span>
              <span>Key</span>
              <span>Value 预览</span>
            </div>
            <div className="bento-table-body">
              {records.map((r, idx) => (
                <div
                  key={`${r.partition}-${r.offset}-${idx}`}
                  className="bento-table-row"
                  style={{ gridTemplateColumns: gridCols, cursor: 'pointer' }}
                  onClick={() => { setDetailRecord(r); setDetailView('json') }}
                >
                  <span className="text-mono" style={{ fontSize: 12 }}>{r.partition}</span>
                  <span className="text-mono" style={{ fontSize: 12 }}>{r.offset}</span>
                  <span className="text-mono" style={{ fontSize: 12 }}>
                    {r.timestamp ? dayjs(r.timestamp).format('MM-DD HH:mm:ss.SSS') : '-'}
                  </span>
                  <span className="text-mono bento-table-cell-wrap" style={{ fontSize: 12 }} title={r.key}>
                    {r.key || <span style={{ color: 'var(--text-3)' }}>(null)</span>}
                  </span>
                  <span className="text-mono bento-table-cell-wrap" style={{ fontSize: 12 }} title={renderValuePreview(r)}>
                    {renderValuePreview(r)}
                  </span>
                </div>
              ))}
            </div>
            <div style={{ marginTop: 12, display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 8 }}>
              <span style={{ fontSize: 12, color: 'var(--text-3)' }}>
                共 {records.length} 条消息 · 点击任意行查看完整内容
                {queryMode === 'key' ? `（在最早的 ${scanned} 条消息中匹配 Key 和消息内容）` : ''}
              </span>
              {(queryMode === 'offset' || queryMode === 'timestamp') && pageTotal > Math.min(maxCount, 500) && (
                <Pagination
                  current={page}
                  pageSize={Math.min(maxCount, 500)}
                  total={pageTotal}
                  showSizeChanger={false}
                  size="small"
                  onChange={(p) => handleQuery(p)}
                />
              )}
            </div>
          </>
        ) : (
          !loading && <Empty description={selectedTopic ? '暂无消息，请调整查询条件' : '请先选择 Topic'} style={{ marginTop: 40 }} />
        )}
      </Spin>

      {/* 消息详情抽屉 */}
      <Drawer
        title="消息详情"
        placement="right"
        width={720}
        open={!!detailRecord}
        onClose={() => setDetailRecord(null)}
      >
        {detailRecord && (
          <div>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 12, marginBottom: 16 }}>
              <div><span style={{ color: 'var(--text-3)', fontSize: 12 }}>分区</span><br /><span className="text-mono">{detailRecord.partition}</span></div>
              <div><span style={{ color: 'var(--text-3)', fontSize: 12 }}>Offset</span><br /><span className="text-mono">{detailRecord.offset}</span></div>
              <div><span style={{ color: 'var(--text-3)', fontSize: 12 }}>时间戳</span><br /><span className="text-mono">{detailRecord.timestamp ? dayjs(detailRecord.timestamp).format('YYYY-MM-DD HH:mm:ss.SSS') : '-'}</span></div>
              <div style={{ flex: 1, minWidth: 200 }}>
                <span style={{ color: 'var(--text-3)', fontSize: 12 }}>Key</span><br />
                <span className="text-mono" style={{ wordBreak: 'break-all', fontSize: 13 }}>{detailRecord.key || '(null)'}</span>
              </div>
            </div>

            {detailRecord.headers && detailRecord.headers.length > 0 && (
              <div style={{ marginBottom: 16 }}>
                <span style={{ color: 'var(--text-3)', fontSize: 12 }}>Headers</span>
                <div style={{ marginTop: 8, display: 'flex', flexDirection: 'column', gap: 4 }}>
                  {detailRecord.headers.map((h, i) => (
                    <div key={i} style={{ display: 'flex', gap: 8, fontSize: 12 }}>
                      <Tag style={{ margin: 0 }}>{h.key}</Tag>
                      <span className="text-mono" style={{ wordBreak: 'break-all' }}>{h.value}</span>
                    </div>
                  ))}
                </div>
              </div>
            )}

            <div style={{ marginBottom: 8, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <span style={{ fontSize: 13, fontWeight: 600 }}>Value</span>
              <Radio.Group value={detailView} onChange={e => setDetailView(e.target.value)} size="small">
                <Radio.Button value="json">JSON</Radio.Button>
                <Radio.Button value="plain">Plain</Radio.Button>
                <Radio.Button value="hex">Hex</Radio.Button>
              </Radio.Group>
            </div>

            <pre
              style={{
                background: 'var(--bg-1)',
                border: '1px solid var(--border)',
                borderRadius: 8,
                padding: 12,
                fontSize: 12,
                fontFamily: 'var(--font-mono, monospace)',
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-all',
                maxHeight: 480,
                overflow: 'auto',
                margin: 0,
              }}
            >
              {renderDetailValue()}
            </pre>

            {detailView === 'json' && (() => {
              const bytes = decodeValue(detailRecord.value)
              const str = bytesToString(bytes)
              const j = tryFormatJson(str)
              return !j.ok && str ? (
                <div style={{ marginTop: 8, fontSize: 12, color: 'var(--text-3)' }}>
                  非 JSON 格式，已回退为纯文本显示
                </div>
              ) : null
            })()}
          </div>
        )}
      </Drawer>
    </div>
  )
}

export default Messages
