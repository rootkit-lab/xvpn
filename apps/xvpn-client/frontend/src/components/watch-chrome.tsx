import type { ReactNode } from 'react'
import { ArrowLeft } from 'lucide-react'

/** Fundo + vinheta compartilhados pelas faces do cliente. */
export function WatchShell({
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
      className={`watch-face relative flex h-full flex-col px-5 pb-5 pt-4 ${
        scroll ? 'overflow-y-auto' : 'overflow-hidden'
      } ${className}`}
    >
      <div className="watch-vignette pointer-events-none absolute inset-0" aria-hidden="true" />
      {children}
    </div>
  )
}

/** Botão circular / app icon do chrome watchOS. */
export function WatchIconButton({
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
  /** Fundo sólido estilo ícone de app (grid watch). */
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

/** Header de páginas secundárias: voltar + título + ações. */
export function WatchPageHeader({
  title,
  onBack,
  trailing,
}: {
  title: string
  onBack: () => void
  trailing?: ReactNode
}) {
  return (
    <header className="relative z-10 flex items-center gap-2">
      <WatchIconButton onClick={onBack} label="Voltar" filled>
        <ArrowLeft className="h-4 w-4" strokeWidth={2.25} />
      </WatchIconButton>
      <h1 className="font-display flex-1 text-[17px] font-semibold tracking-tight">{title}</h1>
      {trailing ? <div className="flex items-center gap-1.5">{trailing}</div> : null}
    </header>
  )
}
