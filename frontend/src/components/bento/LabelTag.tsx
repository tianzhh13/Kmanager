import React from 'react'

export type LabelColor = 'green' | 'blue' | 'purple' | 'red' | 'pink' | 'orange' | 'neutral' | 'warning'

export interface LabelTagProps {
  text: string
  color: LabelColor
  className?: string
}

const LabelTag: React.FC<LabelTagProps> = ({ text, color, className = '' }) => {
  return (
    <span className={`bento-label-tag bento-label-tag--${color} ${className}`}>
      {text}
    </span>
  )
}

export default LabelTag
