import React from 'react'

export interface AvatarInitialsProps {
  name: string
  size?: number
  color?: string
  className?: string
}

/** Deterministic hash-based color palette */
const COLORS = [
  '#f97316', '#3b82f6', '#10b981', '#8b5cf6',
  '#ec4899', '#f59e0b', '#06b6d4', '#6366f1',
]

function hashColor(name: string): string {
  let hash = 0
  for (let i = 0; i < name.length; i++) {
    hash = name.charCodeAt(i) + ((hash << 5) - hash)
  }
  return COLORS[Math.abs(hash) % COLORS.length]
}

const AvatarInitials: React.FC<AvatarInitialsProps> = ({
  name,
  size = 36,
  color,
  className = '',
}) => {
  const initial = name.charAt(0).toUpperCase()
  const bg = color || hashColor(name)

  return (
    <span
      className={`bento-avatar-initials ${className}`}
      style={{
        width: size,
        height: size,
        background: bg,
        fontSize: size * 0.39,
      }}
    >
      {initial}
    </span>
  )
}

export default AvatarInitials
