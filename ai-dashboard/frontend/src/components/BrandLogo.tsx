interface BrandLogoProps {
  size?: number
  collapsed?: boolean
  title?: string
  subtitle?: string
}

/**
 * Brand mark: gradient tile with an orbit/cluster glyph.
 * Pure SVG so it renders crisply at any size and works offline.
 */
export default function BrandLogo({ size = 34, collapsed = false, title, subtitle }: BrandLogoProps) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 12, minWidth: 0 }}>
      <svg
        width={size}
        height={size}
        viewBox="0 0 64 64"
        role="img"
        aria-label={title || 'AI Dashboard'}
        style={{ flexShrink: 0, filter: 'drop-shadow(0 4px 10px rgba(0,113,227,0.28))' }}
      >
        <defs>
          <linearGradient id="brand-g" x1="0" y1="0" x2="1" y2="1">
            <stop offset="0" stopColor="#0071e3" />
            <stop offset="1" stopColor="#5856d6" />
          </linearGradient>
        </defs>
        <rect x="4" y="4" width="56" height="56" rx="16" fill="url(#brand-g)" />
        <circle cx="32" cy="24" r="5" fill="#fff" />
        <circle cx="20" cy="40" r="4" fill="#fff" opacity="0.85" />
        <circle cx="44" cy="40" r="4" fill="#fff" opacity="0.85" />
        <path d="M32 24 L20 40 M32 24 L44 40" stroke="#fff" strokeWidth="3" strokeLinecap="round" opacity="0.7" />
      </svg>
      {!collapsed && title && (
        <div style={{ minWidth: 0, lineHeight: 1.2 }}>
          <div
            style={{
              fontSize: 15,
              fontWeight: 700,
              letterSpacing: '-0.01em',
              color: 'var(--text-1)',
              whiteSpace: 'nowrap',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
            }}
          >
            {title}
          </div>
          {subtitle && (
            <div
              style={{
                fontSize: 11,
                color: 'var(--text-3)',
                whiteSpace: 'nowrap',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
              }}
            >
              {subtitle}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
