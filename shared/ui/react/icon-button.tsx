import type { ReactNode } from 'react'
import { cn } from './cn'

/** Botão de ícone canônico — filled = poço de vidro (header do client); senão círculo. */
export function IconButton({
  children,
  onClick,
  label,
  title,
  disabled,
  filled = false,
  size = 'md',
  className = '',
}: {
  children: ReactNode
  onClick?: () => void
  label: string
  title?: string
  disabled?: boolean
  filled?: boolean
  size?: 'md' | 'lg'
  className?: string
}) {
  const well =
    size === 'lg'
      ? 'size-12 rounded-[16px] icon-well-lg'
      : 'size-8 rounded-[10px] icon-well'

  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      aria-label={label}
      title={title ?? label}
      className={cn(
        'flex items-center justify-center text-foreground transition-transform hover:scale-105 active:scale-95 disabled:cursor-not-allowed disabled:opacity-40',
        filled
          ? well
          : 'size-8 rounded-full text-muted-foreground hover:bg-white/10 hover:text-foreground',
        className,
      )}
    >
      {children}
    </button>
  )
}
