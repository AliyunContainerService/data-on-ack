import type { ReactNode } from 'react';

interface PageHeaderProps {
  title: ReactNode;
  description?: ReactNode;
  extra?: ReactNode;
}

/**
 * Consistent page heading: bold title, muted description, right-aligned actions.
 * Animates in with the shared fade-up entrance.
 */
export default function PageHeader({ title, description, extra }: PageHeaderProps) {
  return (
    <div
      className="anim-fade-up"
      style={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'flex-start',
        gap: 16,
        flexWrap: 'wrap',
        marginBottom: 24,
      }}
    >
      <div style={{ minWidth: 0 }}>
        <h1
          style={{
            margin: 0,
            fontSize: 22,
            fontWeight: 700,
            letterSpacing: '-0.02em',
            color: 'var(--text-1)',
            lineHeight: 1.25,
          }}
        >
          {title}
        </h1>
        {description && (
          <p style={{ margin: '6px 0 0', fontSize: 13, color: 'var(--text-2)', lineHeight: 1.5, maxWidth: 640 }}>
            {description}
          </p>
        )}
      </div>
      {extra && <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexShrink: 0 }}>{extra}</div>}
    </div>
  );
}
