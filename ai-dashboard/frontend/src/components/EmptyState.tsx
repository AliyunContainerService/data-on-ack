import type { ReactNode } from 'react'
import { InboxOutlined } from '@ant-design/icons'

interface EmptyStateProps {
  icon?: ReactNode
  title?: ReactNode
  description?: ReactNode
  action?: ReactNode
}

/** Friendly empty state with soft icon tile and optional CTA. */
export default function EmptyState({ icon, title, description, action }: EmptyStateProps) {
  return (
    <div className="empty-state anim-fade-in">
      <div className="empty-icon">{icon || <InboxOutlined />}</div>
      {title && <div style={{ fontSize: 14, fontWeight: 600, color: 'var(--text-1)', marginBottom: 6 }}>{title}</div>}
      {description && (
        <div style={{ fontSize: 12, color: 'var(--text-3)', maxWidth: 320, lineHeight: 1.6 }}>{description}</div>
      )}
      {action && <div style={{ marginTop: 18 }}>{action}</div>}
    </div>
  )
}
