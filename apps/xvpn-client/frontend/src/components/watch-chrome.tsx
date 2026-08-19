import type { ReactNode } from 'react'
import { ArrowLeft } from 'lucide-react'
import { IconButton } from '@xvpn/ui/react/icon-button'
import { ShellFace } from '@xvpn/ui/react/shell-face'

export { IconButton as WatchIconButton }

/** Fundo + vinheta — `ShellFace` do design system (`shared/ui`). */
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
    <ShellFace className={className} scroll={scroll}>
      {children}
    </ShellFace>
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
      <IconButton onClick={onBack} label="Voltar" filled>
        <ArrowLeft className="h-4 w-4" strokeWidth={2.25} />
      </IconButton>
      <h1 className="font-display flex-1 text-[17px] font-semibold tracking-tight">{title}</h1>
      {trailing ? <div className="flex items-center gap-1.5">{trailing}</div> : null}
    </header>
  )
}
