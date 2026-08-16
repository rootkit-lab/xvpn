import { useEffect, useRef, type ReactNode } from 'react'
import { cn } from '@chat/lib/utils'

/** Fundo + vinheta iguais ao WatchShell do xvpn-client. */
export function ChatShell({
  children,
  className = '',
  scroll = false,
}: {
  children: ReactNode
  className?: string
  scroll?: boolean
}) {
  return (
    <div
      className={cn(
        'watch-face relative flex h-full flex-col px-5 pb-5 pt-4',
        scroll ? 'overflow-y-auto' : 'overflow-hidden',
        className,
      )}
    >
      <div className="watch-vignette pointer-events-none absolute inset-0" aria-hidden="true" />
      {children}
    </div>
  )
}

/** Botão circular / ícone de app do chrome watchOS. */
export function ChatIconButton({
  children,
  onClick,
  label,
  title,
  disabled,
  filled = false,
}: {
  children: ReactNode
  onClick?: () => void
  label: string
  title?: string
  disabled?: boolean
  filled?: boolean
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      aria-label={label}
      title={title ?? label}
      className={
        filled
          ? 'flex size-8 items-center justify-center rounded-[10px] bg-gradient-to-b from-white/16 to-white/6 text-foreground shadow-[inset_0_1px_0_color-mix(in_oklch,white_18%,transparent)] transition-transform hover:scale-105 active:scale-95 disabled:cursor-not-allowed disabled:opacity-40'
          : 'flex size-8 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-white/10 hover:text-foreground disabled:cursor-not-allowed disabled:opacity-40'
      }
    >
      {children}
    </button>
  )
}

/** Menu dropdown ancorado num ícone (status, tema, ações). */
export function ChatDropdown({
  open,
  onClose,
  align = 'right',
  children,
}: {
  open: boolean
  onClose: () => void
  align?: 'left' | 'right'
  children: ReactNode
}) {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!open) return
    function onDoc(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose()
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('mousedown', onDoc)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDoc)
      document.removeEventListener('keydown', onKey)
    }
  }, [open, onClose])
  if (!open) return null
  return (
    <div
      ref={ref}
      role="menu"
      className={cn(
        'absolute top-[calc(100%+8px)] z-30 min-w-[11rem] rounded-[16px] p-1.5 watch-complication',
        align === 'right' ? 'right-0' : 'left-0',
      )}
    >
      {children}
    </div>
  )
}

export function ChatMenuItem({
  children,
  active,
  onClick,
}: {
  children: ReactNode
  active?: boolean
  onClick: () => void
}) {
  return (
    <button
      type="button"
      role="menuitem"
      onClick={onClick}
      className={cn(
        'flex w-full items-center gap-2 rounded-[12px] px-3 py-2 text-left font-display text-[13px] hover:bg-white/8',
        active && 'bg-white/10 text-[var(--safe)]',
      )}
    >
      {children}
    </button>
  )
}
