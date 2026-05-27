import { useState, useEffect, useCallback, useRef } from 'react'
import { Drawer, Table, Input, Button, Space, Tag, Typography, Tooltip, message } from 'antd'
import { BugOutlined, EditOutlined, CheckOutlined, CloseOutlined, UndoOutlined, ClearOutlined, QuestionCircleOutlined } from '@ant-design/icons'

/**
 * usePromqlOverrides - 管理 PromQL 覆写（state + localStorage）
 * @param storageKey localStorage 存储键
 */
export function usePromqlOverrides(storageKey: string) {
  const [overrides, setOverrides] = useState<Record<string, string>>({})

  useEffect(() => {
    try {
      const saved = localStorage.getItem(storageKey)
      if (saved) setOverrides(JSON.parse(saved))
    } catch { /* ignore */ }
  }, [storageKey])

  useEffect(() => {
    localStorage.setItem(storageKey, JSON.stringify(overrides))
  }, [storageKey, overrides])

  const getQ = useCallback((id: string, defaultQ: string) => {
    return overrides[id] || defaultQ
  }, [overrides])

  const setOverride = useCallback((id: string, value: string) => {
    setOverrides(prev => ({ ...prev, [id]: value }))
  }, [])

  const resetOverride = useCallback((id: string) => {
    setOverrides(prev => {
      const next = { ...prev }
      delete next[id]
      return next
    })
  }, [])

  const resetAll = useCallback(() => {
    setOverrides({})
    localStorage.removeItem(storageKey)
  }, [storageKey])

  return { overrides, getQ, setOverride, resetOverride, resetAll }
}

/**
 * useDefaultPromqls - 收集默认 PromQL（在 query 构建时通过 q() 收集）
 * 返回 q() 函数和 ref，传给 PromqlDebugger 展示
 */
export function useDefaultPromqls(getQ: (id: string, defaultQ: string) => string) {
  const ref = useRef<Record<string, string>>({})

  const q = useCallback((id: string, defaultQ: string) => {
    ref.current[id] = defaultQ
    return getQ(id, defaultQ)
  }, [getQ])

  return { q, defaultPromqls: ref }
}

interface PromqlDebuggerProps {
  open: boolean
  onClose: () => void
  defaultPromqls: Record<string, string>
  overrides: Record<string, string>
  onSetOverride: (id: string, value: string) => void
  onResetOverride: (id: string) => void
  onResetAll: () => void
  onApplied?: () => void
  /** 查询 ID → 对应图表/卡片名称 */
  labelMap?: Record<string, string>
}

export const PromqlDebugger: React.FC<PromqlDebuggerProps> = ({
  open, onClose, defaultPromqls, overrides, onSetOverride, onResetOverride, onResetAll, onApplied, labelMap
}) => {
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editingValue, setEditingValue] = useState('')
  const [searchText, setSearchText] = useState('')

  const dataSource = Object.entries(defaultPromqls)
    .filter(([id]) => !searchText || id.toLowerCase().includes(searchText.toLowerCase()))
    .map(([id, defaultQ]) => ({
      key: id,
      id,
      default: defaultQ,
      effective: overrides[id] || defaultQ,
      overridden: !!overrides[id],
    }))

  const handleApply = (id: string) => {
    onSetOverride(id, editingValue)
    setEditingId(null)
    message.success(`已更新: ${id}`)
  }

  const handleReset = (id: string) => {
    onResetOverride(id)
    message.info(`已重置: ${id}`)
  }

  const handleResetAll = () => {
    onResetAll()
    message.info('已重置所有 PromQL')
  }

  const overriddenCount = Object.keys(overrides).filter(id => defaultPromqls[id]).length

  return (
    <Drawer
      title={
        <Space>
          <BugOutlined />
          <span>PromQL 调试</span>
          {overriddenCount > 0 && <Tag color="orange">{overriddenCount} 项已自定义</Tag>}
        </Space>
      }
      open={open}
      onClose={onClose}
      width={780}
      extra={
        <Space>
          {overriddenCount > 0 && (
            <Tooltip title="重置所有自定义 PromQL">
              <Button size="small" icon={<ClearOutlined />} onClick={handleResetAll}>全部重置</Button>
            </Tooltip>
          )}
          {onApplied && (
            <Button size="small" type="primary" onClick={() => { onApplied(); onClose() }}>
              应用并刷新
            </Button>
          )}
        </Space>
      }
    >
      <Input
        placeholder="搜索查询 ID..."
        value={searchText}
        onChange={e => setSearchText(e.target.value)}
        style={{ marginBottom: 12, width: 300 }}
        allowClear
      />

      <Table
        size="small"
        dataSource={dataSource}
        pagination={false}
        scroll={{ y: 'calc(100vh - 240px)' }}
        columns={[
          {
            title: 'ID',
            dataIndex: 'id',
            width: 120,
            render: (id: string) => <Typography.Text strong style={{ fontSize: 11 }}>{id}</Typography.Text>,
          },
          {
            title: 'PromQL',
            render: (_: unknown, record: any) =>
              editingId === record.id ? (
                <Input.TextArea
                  value={editingValue}
                  onChange={e => setEditingValue(e.target.value)}
                  autoSize={{ minRows: 2, maxRows: 8 }}
                  style={{ fontFamily: 'monospace', fontSize: 11 }}
                  onKeyDown={e => { if (e.key === 'Enter' && e.ctrlKey) { handleApply(record.id) } }}
                />
              ) : (
                <Typography.Paragraph
                  copyable
                  code
                  style={{
                    fontFamily: 'monospace',
                    fontSize: 11,
                    margin: 0,
                    cursor: 'pointer',
                    background: record.overridden ? '#fff7e6' : undefined,
                    padding: '4px 8px',
                    borderRadius: 4,
                  }}
                  onClick={() => { setEditingId(record.id); setEditingValue(record.effective) }}
                >
                  {record.effective}
                </Typography.Paragraph>
              ),
          },
          {
            title: '状态',
            width: 80,
            render: (_: unknown, record: any) =>
              record.overridden ? <Tag color="orange" style={{ fontSize: 10 }}>自定义</Tag> : <Tag style={{ fontSize: 10 }}>默认</Tag>,
          },
          {
            title: '操作',
            width: 160,
            render: (_: unknown, record: any) =>
              editingId === record.id ? (
                <Space size={4}>
                  <Tooltip title="Ctrl+Enter 快速应用">
                    <Button size="small" type="primary" icon={<CheckOutlined />} onClick={() => handleApply(record.id)} />
                  </Tooltip>
                  <Button size="small" icon={<CloseOutlined />} onClick={() => setEditingId(null)} />
                </Space>
              ) : (
                <Space size={4}>
                  <Tooltip title="编辑 PromQL">
                    <Button size="small" icon={<EditOutlined />} onClick={() => { setEditingId(record.id); setEditingValue(record.effective) }} />
                  </Tooltip>
                  {record.overridden && (
                    <Tooltip title="恢复默认">
                      <Button size="small" icon={<UndoOutlined />} onClick={() => handleReset(record.id)} />
                    </Tooltip>
                  )}
                  {labelMap?.[record.id] && (
                    <Tooltip title={labelMap[record.id]}>
                      <QuestionCircleOutlined style={{ color: '#999', cursor: 'help' }} />
                    </Tooltip>
                  )}
                </Space>
              ),
          },
        ]}
      />

      <div style={{ marginTop: 16, color: '#999', fontSize: 12 }}>
        提示：点击 PromQL 文本即可编辑，Ctrl+Enter 快速应用。修改后的 PromQL 会持久化到 localStorage。
      </div>
    </Drawer>
  )
}

/**
 * DebugButton - 页面上的调试按钮
 */
export const PromqlDebugButton: React.FC<{
  onClick: () => void
  overrideCount?: number
}> = ({ onClick, overrideCount }) => (
  <Tooltip title={overrideCount ? `${overrideCount} 项 PromQL 已自定义` : 'PromQL 调试'}>
    <Button
      size="small"
      icon={<BugOutlined />}
      onClick={onClick}
      type={overrideCount ? 'primary' : 'default'}
      ghost={!!overrideCount}
    >
      PromQL 调试
    </Button>
  </Tooltip>
)
