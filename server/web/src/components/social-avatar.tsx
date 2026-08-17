import { cn } from '@/lib/utils'
import type { PresenceStatus } from '@/lib/api'
import { presenceLabel } from '@/lib/social-presence'

const DOT: Record<PresenceStatus, string> = {
  online: 'status-safe-dot',
  away: 'bg-[var(--status-away)]',
  dnd: 'bg-[var(--status-dnd)]',
  invisible: 'bg-[var(--status-offline)]',
  offline: 'bg-[var(--status-offline)]',
}

export function SocialAvatar({
  name,
  className,
  textClassName,
  presence,
}: {
  name: string
  className?: string
  textClassName?: string
  presence?: PresenceStatus
}) {
  const letter = (name.trim() || '?').slice(0, 1).toUpperCase()
  return (
    <span className="relative inline-flex shrink-0">
      <span
        className={cn(
          'icon-well flex items-center justify-center rounded-full font-display font-semibold',
          className,
        )}
      >
        <span className={textClassName}>{letter}</span>
      </span>
      {presence && (
        <span
          className={cn(
            'absolute bottom-0.5 right-0.5 size-3 rounded-full ring-2 ring-background',
            DOT[presence],
          )}
          title={presenceLabel(presence)}
        />
      )}
    </span>
  )
}
