import React from 'react'

export interface BentoGridProps {
  children: React.ReactNode
  columns?: number
  gap?: number
  className?: string
  style?: React.CSSProperties
}

const BentoGrid: React.FC<BentoGridProps> = ({
  children,
  columns = 12,
  gap = 16,
  className = '',
  style,
}) => {
  return (
    <div
      className={`bento-grid ${className}`}
      style={{
        gridTemplateColumns: `repeat(${columns}, 1fr)`,
        gap,
        ...style,
      }}
    >
      {children}
    </div>
  )
}

export default BentoGrid
