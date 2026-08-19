import { cn } from '@/lib/utils'
import type { PresenceStatus } from '@/lib/api'
import { presenceLabel } from '@/lib/social-presence'
import { useSocialMediaUrl } from '@/hooks/use-social-media-url'

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
  src,
}: {
  name: string
  className?: string
  textClassName?: string
  presence?: PresenceStatus
  src?: string
}) {
  const letter = (name.trim() || '?').slice(0, 1).toUpperCase()
  const photo = useSocialMediaUrl(src)
  return (
    <span className="relative inline-flex shrink-0">
      <span
        className={cn(
          'icon-well flex items-center justify-center overflow-hidden rounded-full font-display font-semibold',
          className,
        )}
      >
        {photo ? (
          <img src={photo} alt="" className="size-full object-cover" />
        ) : (
          <span className={textClassName}>{letter}</span>
        )}
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
