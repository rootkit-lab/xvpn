import { cn } from '@chat/lib/utils'
import type { PresenceStatus } from '@chat/chatapi/types'

const COLOR: Record<PresenceStatus, string> = {
  online: 'bg-[var(--status-online)]',
  away: 'bg-[var(--status-away)]',
  dnd: 'bg-[var(--status-dnd)]',
  invisible: 'bg-[var(--status-offline)]',
  offline: 'bg-[var(--status-offline)]',
}

export function StatusDot({ status, className }: { status: PresenceStatus; className?: string }) {
  return (
    <span
      className={cn('inline-block size-2.5 shrink-0 rounded-full ring-2 ring-[var(--card)]', COLOR[status], className)}
      title={status}
    />
  )
}

export function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean)
  if (parts.length === 0) return '?'
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase()
  return (parts[0][0] + parts[1][0]).toUpperCase()
}
