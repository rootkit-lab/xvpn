import type { ReactNode } from 'react'
import { Link, Outlet, useNavigate } from 'react-router-dom'
import { HardDrive, LogOut, Store } from 'lucide-react'
import { useAuth } from '@/lib/auth-context'
import { PANEL_ORIGIN } from '@/lib/product-host'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

export function StoreShell({
  kind,
  search,
}: {
  kind: 'marketplace' | 'xdriver'
  search?: ReactNode
}) {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const Icon = kind === 'marketplace' ? Store : HardDrive
  const title = kind === 'marketplace' ? 'Marketplace' : 'XDriver'
  const kicker = kind === 'marketplace' ? 'ihuull store' : 'seus arquivos'

  return (
    <div className="watch-face relative flex min-h-svh flex-col overflow-hidden">
      <div className="watch-vignette pointer-events-none absolute inset-0" aria-hidden="true" />
      <header className="chrome-bar relative z-20 flex items-center gap-3 px-4 py-3 md:px-6">
        <Link to="/" className="flex min-w-0 items-center gap-2.5">
          <span className="icon-well flex size-10 items-center justify-center rounded-[12px]">
            <Icon className="size-5" />
          </span>
          <span className="min-w-0">
            <span className="font-display block text-[15px] font-semibold tracking-tight">{title}</span>
            <span className="hud-label text-muted-foreground/70">{kicker}</span>
          </span>
        </Link>
        <div className="min-w-0 flex-1">{search}</div>
        <div className="flex shrink-0 items-center gap-2">
          {user && (
            <span className="hidden max-w-32 truncate text-sm text-muted-foreground sm:inline">{user.username}</span>
          )}
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            title="Sair"
            onClick={() => {
              logout()
              navigate('/login', { replace: true })
            }}
          >
            <LogOut className="size-4" />
          </Button>
        </div>
      </header>
      <main className="relative z-10 min-h-0 flex-1 overflow-y-auto">
        <Outlet />
      </main>
      <footer className="relative z-10 px-4 py-3 text-center text-[11px] text-muted-foreground/70">
        <a href={PANEL_ORIGIN} className={cn('underline-offset-4 hover:text-foreground hover:underline')}>
          Painel XVPN
        </a>
      </footer>
    </div>
  )
}
