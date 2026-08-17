import { useCallback } from 'react'
import { Link } from 'react-router-dom'
import { Download, ExternalLink, HardDrive, Laptop, Shield, Store, UserRound } from 'lucide-react'
import { ProductHeader } from '@xvpn/ui/react/product-header'
import { useAuth } from '@/lib/auth-context'
import { api } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { isViewerUpRole } from '@/lib/roles'
import {
  MARKETPLACE_ORIGIN,
  XCHAT_CORP_ORIGIN,
  XDRIVER_CORP_ORIGIN,
  XGROUP_CORP_ORIGIN,
  ssoLoginURL,
} from '@/lib/product-host'
import { AccountMenu } from '@/components/layout/account-menu'
import { AppLauncher } from '@/components/layout/app-launcher'
import { AppSettingsButton } from '@/components/layout/app-settings-button'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

const SHORTCUTS = [
  {
    href: MARKETPLACE_ORIGIN,
    label: 'Marketplace',
    description: 'Cliente e apps da intranet',
    icon: Store,
  },
  {
    href: XDRIVER_CORP_ORIGIN,
    label: 'XDRIVER',
    description: 'Arquivos — só com a VPN',
    icon: HardDrive,
  },
  {
    href: XCHAT_CORP_ORIGIN,
    label: 'XCHAT',
    description: 'Messenger na VPN',
    icon: Laptop,
  },
  {
    href: XGROUP_CORP_ORIGIN,
    label: 'XGROUP',
    description: 'Rede social na VPN',
    icon: UserRound,
  },
] as const

export function XvpnProductPortal() {
  const { user, isAuthenticated, isLoadingUser } = useAuth()
  const fetchStatus = useCallback(() => api.status(), [])
  const { data: status, error, loading } = usePollingData(fetchStatus, 10_000, isAuthenticated)
  const online = Boolean(status) && !error
  const checking = isAuthenticated && loading && !status && !error
  const showAdmin = isViewerUpRole(user?.role)
  const loginHref = ssoLoginURL()

  return (
    <div data-product="xvpn" className="watch-face relative flex min-h-svh flex-col overflow-hidden">
      <div className="watch-vignette pointer-events-none absolute inset-0" aria-hidden="true" />

      <ProductHeader
        product="xvpn"
        href="/"
        trailing={
          isLoadingUser ? (
            <span className="hud-label text-muted-foreground/50">Sessão…</span>
          ) : isAuthenticated && user ? (
            <>
              <AppSettingsButton kind="user" />
              <AppLauncher variant="user" />
              <AccountMenu variant="user" />
            </>
          ) : (
            <Button size="sm" className="rounded-full" asChild>
              <a href={loginHref}>Entrar</a>
            </Button>
          )
        }
      />

      <main className="relative z-10 mx-auto flex w-full max-w-6xl flex-1 flex-col justify-center gap-8 px-4 py-10 md:px-8">
        <section className="flex flex-col gap-3">
          <p className="hud-label text-muted-foreground/70">Portal</p>
          <h1 className="font-display text-3xl font-semibold tracking-tight md:text-4xl">
            {isAuthenticated && user ? `Olá, ${user.username}` : 'Sua VPN privada'}
          </h1>
          <p className="max-w-xl text-sm leading-relaxed text-muted-foreground">
            {isAuthenticated
              ? 'Status do túnel, cliente e atalhos da intranet. Peers e contas ficam em Administração.'
              : 'Baixe o cliente e entre com a conta ihuull. O estado da VPN e os peers só aparecem depois do login.'}
          </p>
        </section>

        <section className="watch-complication flex items-center gap-4 rounded-[22px] p-5">
          <span
            className={cn(
              'size-2.5 shrink-0 rounded-full',
              !isAuthenticated || checking ? 'bg-muted-foreground/50' : online ? 'status-safe-dot' : 'bg-destructive',
            )}
            aria-hidden
          />
          <div className="min-w-0 flex-1">
            <p className="font-display text-sm font-semibold">
              {!isAuthenticated
                ? 'Entre para ver o status'
                : checking
                  ? 'Consultando…'
                  : online
                    ? 'VPN no ar'
                    : 'API offline'}
            </p>
            {isAuthenticated && status ? (
              <p className="mt-1 text-sm text-muted-foreground">
                {status.connected_peers} de {status.total_peers} peers com handshake recente · api v
                {status.api_version}
              </p>
            ) : (
              <p className="mt-1 text-sm text-muted-foreground">
                {!isAuthenticated
                  ? 'O control-plane não publica peers para quem está deslogado.'
                  : error
                    ? 'Não foi possível falar com o control-plane.'
                    : 'Consultando o control-plane…'}
              </p>
            )}
          </div>
          {showAdmin && (
            <Button size="sm" variant="outline" className="hidden rounded-full sm:inline-flex" asChild>
              <Link to="/admin">
                <Shield className="size-4" />
                Admin
              </Link>
            </Button>
          )}
        </section>

        <section className="flex flex-wrap gap-3">
          <Button size="lg" className="rounded-full" asChild>
            <a href={MARKETPLACE_ORIGIN}>
              <Download className="size-4" />
              Baixar o cliente
            </a>
          </Button>
          {!isLoadingUser && !isAuthenticated && (
            <Button size="lg" variant="outline" className="rounded-full" asChild>
              <a href={loginHref}>Já tenho acesso</a>
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
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {isAuthenticated && (
              <Link to="/my/devices" className="watch-complication-lift watch-complication block rounded-[18px] p-4">
                <Laptop className="mb-3 size-5 text-muted-foreground" />
                <p className="font-display text-sm font-semibold">Dispositivos</p>
                <p className="mt-1 text-xs leading-relaxed text-muted-foreground">
                  Revogar os seus peers — não é o dashboard de admin
                </p>
              </Link>
            )}
            {SHORTCUTS.map(({ href, label, description, icon: Icon }) => (
              <a
                key={href}
                href={href}
                className="watch-complication-lift watch-complication block rounded-[18px] p-4"
              >
                <Icon className="mb-3 size-5 text-muted-foreground" />
                <p className="font-display text-sm font-semibold">{label}</p>
                <p className="mt-1 text-xs leading-relaxed text-muted-foreground">{description}</p>
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
