import { NavLink, Outlet } from 'react-router-dom'
import { Home, MessageSquare, Search, UsersRound } from 'lucide-react'
import { ProductHeader } from '@xvpn/ui/react/product-header'
import { cn } from '@/lib/utils'
import { useAuth } from '@/lib/auth-context'
import { XCHAT_CORP_ORIGIN, XGROUP_CORP_ORIGIN, productKind } from '@/lib/product-host'
import { AccountMenu } from '@/components/layout/account-menu'
import { AppLauncher } from '@/components/layout/app-launcher'
import { SystemChrome } from '@/components/layout/system-chrome'

const SOCIAL_NAV = [
  { to: '/social', label: 'Início', icon: Home, end: true, product: 'xgroup' as const },
  { to: '/social/explore', label: 'Explorar', icon: Search, end: false, product: 'xgroup' as const },
  { to: '/social/messages', label: 'Mensagens', icon: MessageSquare, end: false, product: 'xchat' as const },
  { to: '/social/groups', label: 'Grupos', icon: UsersRound, end: false, product: 'xgroup' as const },
]

/** Perfil em xgroup.ihuull.com — sem ChatHost (messenger fica no corp). */
export function PublicProfileShell() {
  const { user } = useAuth()
  return (
    <div data-product="xgroup" className="watch-face relative flex min-h-svh flex-col overflow-hidden">
      <div className="watch-vignette pointer-events-none absolute inset-0" aria-hidden="true" />
      <ProductHeader
        product="xgroup"
        href={`${XGROUP_CORP_ORIGIN}/social`}
        trailing={
          user ? (
            <>
              <AppLauncher variant="social" />
              <AccountMenu variant="social" />
            </>
          ) : null
        }
      />
      <main className="relative z-10 min-h-0 flex-1 overflow-y-auto p-6 md:p-8">
        <Outlet />
      </main>
    </div>
  )
}

export function SocialShell() {
  const kind = productKind()
  const subtitle = kind === 'xchat-corp' ? 'XCHAT' : 'XGROUP'

  return (
    <SystemChrome
      variant="social"
      subtitle={subtitle}
      nav={
        <nav className="flex flex-1 flex-col gap-1 overflow-y-auto px-3 py-2">
          {SOCIAL_NAV.map(({ to, label, icon: Icon, end, product }) => {
            const href =
              product === 'xchat' && kind === 'xgroup-corp'
                ? `${XCHAT_CORP_ORIGIN}${to}`
                : product === 'xgroup' && kind === 'xchat-corp'
                  ? `${XGROUP_CORP_ORIGIN}${to}`
                  : null
            const className = 'nav-link'
            if (href) {
              return (
                <a key={to} href={href} className={className}>
                  <Icon className="size-4" />
                  {label}
                </a>
              )
            }
            return (
              <NavLink
                key={to}
                to={to}
                end={end}
                className={({ isActive }) => cn(className, isActive && 'nav-link-active')}
              >
                <Icon className="size-4" />
                {label}
              </NavLink>
            )
          })}
        </nav>
      }
    />
  )
}
