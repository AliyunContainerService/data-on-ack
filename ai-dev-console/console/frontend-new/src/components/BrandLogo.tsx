interface BrandLogoProps {
  size?: number;
  collapsed?: boolean;
  title?: string;
  subtitle?: string;
}

/**
 * Brand mark: gradient tile with a lightning glyph.
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
        aria-label={title || 'AI Dev Console'}
        style={{ flexShrink: 0, filter: 'drop-shadow(0 4px 10px rgba(99,102,241,0.32))' }}
      >
        <defs>
          <linearGradient id="brand-g" x1="0" y1="0" x2="1" y2="1">
            <stop offset="0" stopColor="#6366f1" />
            <stop offset="1" stopColor="#a855f7" />
          </linearGradient>
        </defs>
        <rect x="4" y="4" width="56" height="56" rx="16" fill="url(#brand-g)" />
        <path d="M34 12 L22 34 h8 L28 52 L44 28 h-9 z" fill="#fff" />
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
  );
}
