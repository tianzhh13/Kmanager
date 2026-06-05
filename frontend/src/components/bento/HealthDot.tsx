import React from 'react'

export type HealthStatus = 'healthy' | 'warning' | 'error' | 'unknown'

export interface HealthDotProps {
  status: HealthStatus
  pulse?: boolean
  className?: string
  style?: React.CSSProperties
}

const HealthDot: React.FC<HealthDotProps> = ({
  status,
  pulse = true,
  className = '',
  style,
}) => {
  const classes = [
    'bento-health-dot',
    `bento-health-dot--${status}`,
    !pulse && 'bento-health-dot--no-pulse',
    className,
  ].filter(Boolean).join(' ')

  return <span className={classes} style={style} />
}

export default HealthDot
