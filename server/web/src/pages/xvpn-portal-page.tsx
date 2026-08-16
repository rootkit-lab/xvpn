import { useCallback } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Download, ExternalLink, HardDrive, Laptop, LogOut, Shield, Store, UserRound } from 'lucide-react'
import { useAuth } from '@/lib/auth-context'
import { api } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { isViewerUpRole } from '@/lib/roles'
import { MARKETPLACE_ORIGIN, XDRIVER_ORIGIN, XGROUP_ORIGIN } from '@/lib/product-host'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

const SHORTCUTS = [
  {
    href: MARKETPLACE_ORIGIN,
    label: 'Marketplace',
    description: 'Baixe o cliente e os apps da intranet',
    icon: Store,
  },
  {
    href: XDRIVER_ORIGIN,
    label: 'XDriver',
    description: 'Arquivos só com a VPN ligada',
    icon: HardDrive,
  },
  {
    href: XGROUP_ORIGIN,
    label: 'xgroup',
    description: 'Rede social — app em xgroup.corp',
    icon: UserRound,
  },
] as const

export function XvpnProductPortal() {
  const { user, isAuthenticated, logout } = useAuth()
  const navigate = useNavigate()
  const fetchStatus = useCallback(() => api.status(), [])
  const { data: status, error } = usePollingData(fetchStatus, 10_000)
  const online = Boolean(status) && !error
  const showAdmin = isViewerUpRole(user?.role)

  return (
    <div className="watch-face relative flex min-h-svh flex-col overflow-hidden">
      <div className="watch-vignette pointer-events-none absolute inset-0" aria-hidden="true" />

      <header className="chrome-bar relative z-20 flex items-center gap-3 px-4 py-3 md:px-6">
        <Link to="/" className="flex min-w-0 items-center gap-2.5">
          <span className="icon-well flex size-10 items-center justify-center rounded-[12px]">
            <Shield className="size-5" />
          </span>
          <span className="min-w-0">
            <span className="font-display block text-[15px] font-semibold tracking-tight">XVPN</span>
            <span className="hud-label text-muted-foreground/70">sua rede</span>
          </span>
        </Link>
        <div className="ml-auto flex shrink-0 items-center gap-2">
          {isAuthenticated && user ? (
            <>
              <span className="hidden max-w-32 truncate text-sm text-muted-foreground sm:inline">
                {user.username}
              </span>
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                title="Sair"
                onClick={() => {
                  logout()
                  navigate('/my/login', { replace: true })
                }}
              >
                <LogOut className="size-4" />
              </Button>
            </>
          ) : (
            <Button variant="ghost" className="rounded-full" asChild>
              <Link to="/my/login">Entrar</Link>
            </Button>
          )}
        </div>
      </header>

      <main className="relative z-10 mx-auto flex w-full max-w-3xl flex-1 flex-col gap-8 px-4 py-10 md:px-8">
        <section className="flex flex-col gap-3">
          <p className="hud-label text-muted-foreground/70">Portal</p>
          <h1 className="font-display text-3xl font-semibold tracking-tight">Sua VPN privada</h1>
          <p className="max-w-xl text-sm leading-relaxed text-muted-foreground">
            Status do túnel, download do cliente e atalhos. A operação de peers fica em Administração —
            esta home não lista dispositivos.
          </p>
        </section>

        <section className="watch-complication flex items-start gap-3 rounded-[18px] p-4">
          <span
            className={cn('mt-1.5 size-2 shrink-0 rounded-full', online ? 'status-safe-dot' : 'bg-destructive')}
            aria-hidden
          />
          <div className="min-w-0">
            <p className="font-display text-sm font-semibold">{online ? 'VPN no ar' : 'API offline'}</p>
            {status ? (
              <p className="mt-1 text-sm text-muted-foreground">
                {status.connected_peers} de {status.total_peers} peers com handshake recente · api v
                {status.api_version}
              </p>
            ) : (
              <p className="mt-1 text-sm text-muted-foreground">Consultando o control-plane…</p>
            )}
          </div>
        </section>

        <section className="flex flex-wrap gap-3">
          <Button size="lg" className="rounded-full" asChild>
            <a href={MARKETPLACE_ORIGIN}>
              <Download className="size-4" />
              Baixar o cliente
            </a>
          </Button>
          {!isAuthenticated && (
            <Button size="lg" variant="outline" className="rounded-full" asChild>
              <Link to="/my/login">Já tenho acesso</Link>
            </Button>
          )}
          {isAuthenticated && (
            <Button size="lg" variant="outline" className="rounded-full" asChild>
              <Link to="/my/account">Minha conta</Link>
            </Button>
          )}
        </section>

        <section>
          <h2 className="hud-label mb-3 text-muted-foreground/70">Atalhos</h2>
          <div className="grid gap-3 sm:grid-cols-3">
            {isAuthenticated && (
              <Link to="/my/devices" className="watch-complication-lift watch-complication block rounded-[18px] p-4">
                <Laptop className="mb-2 size-5 text-muted-foreground" />
                <p className="font-display text-sm font-semibold">Dispositivos</p>
                <p className="mt-1 text-xs text-muted-foreground">Revogar os seus peers — não é o dashboard de admin</p>
              </Link>
            )}
            {SHORTCUTS.map(({ href, label, description, icon: Icon }) => (
              <a
                key={href}
                href={href}
                className="watch-complication-lift watch-complication block rounded-[18px] p-4"
              >
                <Icon className="mb-2 size-5 text-muted-foreground" />
                <p className="font-display text-sm font-semibold">{label}</p>
                <p className="mt-1 text-xs text-muted-foreground">{description}</p>
              </a>
            ))}
          </div>
        </section>
      </main>

      <footer className="chrome-bar relative z-10 flex flex-wrap items-center justify-center gap-x-3 gap-y-1 px-4 py-3 text-center text-[11px] text-muted-foreground/70">
        {showAdmin && (
          <Link to="/admin" className="underline-offset-4 hover:text-foreground hover:underline">
            Administração
          </Link>
        )}
        <a
          href={MARKETPLACE_ORIGIN}
          className="inline-flex items-center gap-1 underline-offset-4 hover:text-foreground hover:underline"
        >
          Loja
          <ExternalLink className="size-3" />
        </a>
      </footer>
    </div>
  )
}
