import { cn } from '@/lib/utils'

export function SocialAvatar({
  name,
  className,
  textClassName,
}: {
  name: string
  className?: string
  textClassName?: string
}) {
  const letter = (name.trim() || '?').slice(0, 1).toUpperCase()
  return (
    <span
      className={cn(
        'icon-well flex items-center justify-center rounded-full font-display font-semibold',
        className,
      )}
    >
      <span className={textClassName}>{letter}</span>
    </span>
  )
}
