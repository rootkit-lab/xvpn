import type { ReactNode } from 'react'

/** Botão de ícone canônico — filled = rounded-[10px] vidro; senão círculo. */
export function IconButton({
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
