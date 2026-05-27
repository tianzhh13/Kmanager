import React from 'react'
import { Select, Button, Space, DatePicker } from 'antd'
import { CalendarOutlined } from '@ant-design/icons'
import dayjs, { Dayjs } from 'dayjs'

interface MonitorControlsProps {
  selectedCluster: number | undefined
  onClusterChange: (clusterId: number) => void
  clusters: Array<{ cluster_id: number; cluster_name: string }>
  timeRange: 'quick' | 'custom'
  onTimeRangeChange: (range: 'quick' | 'custom') => void
  quickRange: string
  onQuickRangeChange: (range: string) => void
  customRange: [Dayjs, Dayjs] | null
  onCustomRangeChange: (range: [Dayjs, Dayjs] | null) => void
}

const quickRangeOptions = [
  { label: '5m', value: '5m' },
  { label: '15m', value: '15m' },
  { label: '30m', value: '30m' },
  { label: '1h', value: '1h' },
  { label: '3h', value: '3h' },
  { label: '6h', value: '6h' },
  { label: '12h', value: '12h' },
  { label: '24h', value: '24h' },
  { label: '2d', value: '2d' },
  { label: '7d', value: '7d' },
  { label: '30d', value: '30d' },
]

const MonitorControls: React.FC<MonitorControlsProps> = ({
  selectedCluster,
  onClusterChange,
  clusters,
  timeRange,
  onTimeRangeChange,
  quickRange,
  onQuickRangeChange,
  customRange,
  onCustomRangeChange,
}) => {
  return (
    <Space>
      <Select
        placeholder="选择集群"
        value={selectedCluster}
        onChange={onClusterChange}
        style={{ width: 200 }}
        options={clusters.map(c => ({ label: c.cluster_name, value: c.cluster_id }))}
      />
      {timeRange === 'quick' ? (
        <>
          <Button.Group>
            {quickRangeOptions.map(opt => (
              <Button
                key={opt.value}
                size="small"
                type={quickRange === opt.value ? 'primary' : 'default'}
                onClick={() => onQuickRangeChange(opt.value)}
              >
                {opt.label}
              </Button>
            ))}
          </Button.Group>
          <Button
            size="small"
            icon={<CalendarOutlined />}
            onClick={() => onTimeRangeChange('custom')}
          >
            自定义
          </Button>
        </>
      ) : (
        <>
          <DatePicker.RangePicker
            size="small"
            showTime
            value={customRange}
            onChange={(dates) => {
              if (dates && dates[0] && dates[1]) {
                onCustomRangeChange([dates[0], dates[1]])
              }
            }}
            presets={[
              { label: '最近1小时', value: [dayjs().subtract(1, 'hour'), dayjs()] },
              { label: '最近6小时', value: [dayjs().subtract(6, 'hour'), dayjs()] },
              { label: '最近24小时', value: [dayjs().subtract(24, 'hour'), dayjs()] },
              { label: '最近7天', value: [dayjs().subtract(7, 'day'), dayjs()] },
              { label: '最近30天', value: [dayjs().subtract(30, 'day'), dayjs()] },
            ]}
            style={{ width: 360 }}
          />
          <Button
            size="small"
            onClick={() => { onTimeRangeChange('quick'); onCustomRangeChange(null) }}
          >
            快速选择
          </Button>
        </>
      )}
    </Space>
  )
}

export default React.memo(MonitorControls)
