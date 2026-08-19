import { useEffect, useRef, type ReactNode } from 'react'
import { IconButton } from '@xvpn/ui/react/icon-button'
import { ShellFace } from '@xvpn/ui/react/shell-face'
import { cn } from '@chat/lib/utils'

export { IconButton as ChatIconButton }

/** Fundo + vinheta — `ShellFace` do design system (`shared/ui`). */
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
    <ShellFace className={className} scroll={scroll}>
      {children}
    </ShellFace>
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
