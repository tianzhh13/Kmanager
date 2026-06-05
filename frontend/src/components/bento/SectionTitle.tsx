import React from 'react'

export interface SectionTitleProps {
  title: string
  className?: string
  style?: React.CSSProperties
}

const SectionTitle: React.FC<SectionTitleProps> = ({
  title,
  className = '',
  style,
}) => {
  return (
    <div className={`bento-section-title ${className}`} style={style}>
      {title}
    </div>
  )
}

export default SectionTitle
