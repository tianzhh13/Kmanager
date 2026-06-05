import React from 'react'

export interface SearchBarProps {
  value: string
  onChange: (value: string) => void
  placeholder?: string
  className?: string
  style?: React.CSSProperties
}

const SearchBar: React.FC<SearchBarProps> = ({
  value,
  onChange,
  placeholder = '搜索...',
  className = '',
  style,
}) => {
  return (
    <div className={`bento-search-wrap ${className}`} style={style}>
      <span className="bento-search-icon">
        <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <circle cx="11" cy="11" r="8" />
          <line x1="21" y1="21" x2="16.65" y2="16.65" />
        </svg>
      </span>
      <input
        type="text"
        value={value}
        onChange={e => onChange(e.target.value)}
        placeholder={placeholder}
      />
    </div>
  )
}

export default SearchBar
