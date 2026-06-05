import React from 'react'

export interface DoubleBezelProps {
  children: React.ReactNode
  className?: string
  dark?: boolean
  hover?: boolean
  style?: React.CSSProperties
  onClick?: () => void
}

const DoubleBezel: React.FC<DoubleBezelProps> = ({
  children,
  className = '',
  dark = false,
  hover = true,
  style,
  onClick,
}) => {
  const classes = [
    'bento-card',
    dark && 'bento-card-dark',
    !hover && 'bento-card-no-hover',
    className,
  ].filter(Boolean).join(' ')

  return (
    <div className={classes} style={style} onClick={onClick}>
      <div className="bento-card-inner">
        {children}
      </div>
    </div>
  )
}

export default DoubleBezel
