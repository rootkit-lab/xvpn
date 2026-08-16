import { Outlet } from 'react-router-dom'
import { ProductHeader } from '@xvpn/ui/react/product-header'
import { useAuth } from '@/lib/auth-context'
import { PANEL_ORIGIN } from '@/lib/product-host'
import { AccountMenu } from '@/components/layout/account-menu'
import { AppLauncher } from '@/components/layout/app-launcher'
import { AppSettingsButton } from '@/components/layout/app-settings-button'
import { cn } from '@/lib/utils'

export function StoreShell({ kind }: { kind: 'marketplace' | 'xdriver' }) {
  const { user } = useAuth()

  return (
    <div data-product={kind} className="watch-face relative flex min-h-svh flex-col overflow-hidden">
      <div className="watch-vignette pointer-events-none absolute inset-0" aria-hidden="true" />
      <ProductHeader
        product={kind}
        href="/"
        trailing={
          user ? (
            <>
              <AppSettingsButton kind={kind} />
              <AppLauncher variant={kind} />
              <AccountMenu variant="user" />
            </>
          ) : null
        }
      />
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
