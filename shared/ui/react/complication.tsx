import type { ReactNode } from 'react'
import { cn } from './cn'

/** Card de superfície — rounded 18–22px + watch-complication (IP/servidor do client). */
export function Complication({
  children,
  className = '',
  as: Tag = 'div',
  label,
  value,
  lift = false,
}: {
  children?: ReactNode
  className?: string
  as?: 'div' | 'section' | 'article'
  label?: string
  value?: ReactNode
  lift?: boolean
}) {
  return (
    <Tag
      className={cn(
        'watch-complication rounded-[18px]',
        label && 'px-3.5 py-2.5',
        lift && 'watch-complication-lift',
        className,
      )}
    >
      {label ? (
        <>
          <p className="hud-label text-muted-foreground/75">{label}</p>
          {value != null && (
            <p className="mt-0.5 font-display text-[13px] font-semibold tabular-nums tracking-tight">
              {value}
            </p>
          )}
          {children}
        </>
      ) : (
        children
      )}
    </Tag>
  )
}
