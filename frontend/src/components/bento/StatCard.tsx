import React from 'react'

export interface StatCardProps {
  label: string
  value: string | number | null | undefined
  delta?: { value: string; trend: 'up' | 'down' | 'neutral' }
  unit?: string
  icon?: React.ReactNode
  color?: string
  valueColor?: string
  subtitle?: string
  className?: string
  style?: React.CSSProperties
  onClick?: () => void
}

const StatCard: React.FC<StatCardProps> = ({
  label,
  value,
  delta,
  unit,
  icon,
  color,
  valueColor,
  subtitle,
  className = '',
  style,
  onClick,
}) => {
  const hasData = value !== null && value !== undefined && value !== ''

  return (
    <div
      className={`bento-card ${className}`}
      style={{ cursor: onClick ? 'pointer' : undefined, ...style }}
      onClick={onClick}
    >
      <div className="bento-card-inner" style={{ padding: '18px 20px' }}>
        <div className="bento-stat-label">{label}</div>
        <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, marginTop: 8 }}>
          {icon && <span style={{ marginRight: 4, color: 'var(--text-3)', fontSize: 18 }}>{icon}</span>}
          {hasData ? (
            <span className="bento-stat-value" style={valueColor ? { color: valueColor } : color ? { color } : undefined}>{typeof value === 'number' ? value.toLocaleString() : value}</span>
          ) : (
            <span style={{ color: 'var(--text-3)', fontSize: 14, fontStyle: 'italic' }}>暂无数据</span>
          )}
          {unit && hasData && <span className="bento-stat-delta--unit">{unit}</span>}
        </div>
        {subtitle && (
          <span style={{ fontSize: 12, color: 'var(--text-3)', marginTop: 4, display: 'block' }}>{subtitle}</span>
        )}
        {delta && (
          <span className={`bento-stat-delta bento-stat-delta--${delta.trend}`}>
            {delta.trend === 'up' && '\u25B2 '}
            {delta.trend === 'down' && '\u25BC '}
            {delta.value}
          </span>
        )}
      </div>
    </div>
  )
}

export default StatCard
