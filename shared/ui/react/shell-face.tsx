import type { ReactNode } from 'react'
import { cn } from './cn'

/** Fundo watch-face + vinheta. Use em todo shell de produto (painel, xvpn, xchat). */
export function ShellFace({
  children,
  className = '',
  scroll = false,
  padded = true,
}: {
  children: ReactNode
  className?: string
  scroll?: boolean
  padded?: boolean
}) {
  return (
    <div
      className={cn(
        'watch-face relative flex h-full flex-col',
        padded && 'px-5 pb-5 pt-4',
        scroll ? 'overflow-y-auto' : 'overflow-hidden',
        className,
      )}
    >
      <div className="watch-vignette pointer-events-none absolute inset-0" aria-hidden="true" />
      {children}
    </div>
  )
}
