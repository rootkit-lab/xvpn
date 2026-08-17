import { ExternalLink, GitBranch, HardDrive, LayoutDashboard, MessageCircle, MessagesSquare, Shield } from 'lucide-react'
import { ProductHeader } from '@xvpn/ui/react/product-header'
import {
  PANEL_ORIGIN,
  XADMIN_CORP_ORIGIN,
  XCHAT_CORP_ORIGIN,
  XDRIVER_CORP_ORIGIN,
  XGIT_CORP_ORIGIN,
  XGROUP_CORP_ORIGIN,
} from '@/lib/product-host'
import { AccountMenu } from '@/components/layout/account-menu'
import { AppLauncher } from '@/components/layout/app-launcher'
import { AppSettingsButton } from '@/components/layout/app-settings-button'
import { useAuth } from '@/lib/auth-context'

const APPS = [
  {
    href: XCHAT_CORP_ORIGIN,
    label: 'XCHAT',
    description: 'Messenger',
    icon: MessagesSquare,
  },
  {
    href: XGROUP_CORP_ORIGIN,
    label: 'XGROUP',
    description: 'Rede social',
    icon: MessageCircle,
  },
  {
    href: XDRIVER_CORP_ORIGIN,
    label: 'XDRIVER',
    description: 'Arquivos',
    icon: HardDrive,
  },
  {
    href: PANEL_ORIGIN,
    label: 'XVPN',
    description: 'Portal e enroll',
    icon: Shield,
  },
  {
    href: `${XADMIN_CORP_ORIGIN}/admin`,
    label: 'XADMIN',
    description: 'Console',
    icon: LayoutDashboard,
  },
  {
    href: XGIT_CORP_ORIGIN,
    label: 'XGIT',
    description: 'Git smart HTTP',
    icon: GitBranch,
  },
] as const

export function CorpHubPage() {
  const { user } = useAuth()

  return (
    <div data-product="xvpn" className="watch-face relative flex min-h-svh flex-col overflow-hidden">
      <div className="watch-vignette pointer-events-none absolute inset-0" aria-hidden="true" />
      <ProductHeader
        product="xvpn"
        href="/"
        trailing={
          user ? (
            <>
              <AppSettingsButton kind="user" />
              <AppLauncher variant="user" />
              <AccountMenu variant="user" />
            </>
          ) : null
        }
      />

      <main className="relative z-10 mx-auto flex w-full max-w-xl flex-1 flex-col justify-center gap-6 px-6 py-16">
        <p className="hud-label text-muted-foreground/70">Intranet</p>
        <h1 className="font-display text-3xl font-semibold tracking-tight">Apps da VPN</h1>
        <p className="text-sm leading-relaxed text-muted-foreground">
          Cada produto abre no próprio host. Administração fica em{' '}
          <code className="font-mono text-xs">xadmin.corp.ihuull.com</code> — não neste endereço.
        </p>
        <ul className="grid gap-3">
          {APPS.map(({ href, label, description, icon: Icon }) => (
            <li key={href}>
              <a
                href={href}
                className="watch-complication flex items-center gap-3 rounded-[18px] p-4 hover:bg-white/6"
              >
                <Icon className="size-5 shrink-0 text-muted-foreground" />
                <span className="min-w-0 flex-1">
                  <span className="font-display block text-sm font-semibold">{label}</span>
                  <span className="text-xs text-muted-foreground">{description}</span>
                </span>
                <ExternalLink className="size-4 shrink-0 text-muted-foreground" />
              </a>
            </li>
          ))}
        </ul>
      </main>
    </div>
  )
}
