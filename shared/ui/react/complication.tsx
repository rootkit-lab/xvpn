import type { ReactNode } from 'react'
import { cn } from './cn'

/** Card de superfície — rounded 18–22px + watch-complication. */
export function Complication({
  children,
  className = '',
  as: Tag = 'div',
}: {
  children: ReactNode
  className?: string
  as?: 'div' | 'section' | 'article'
}) {
  return (
    <Tag className={cn('watch-complication rounded-[18px]', className)}>{children}</Tag>
  )
}
