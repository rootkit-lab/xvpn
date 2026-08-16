import type { ReactNode } from 'react'
import { Outlet, useNavigate } from 'react-router-dom'
import { LogOut } from 'lucide-react'
import { ProductHeader } from '@xvpn/ui/react/product-header'
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

  return (
    <div data-product={kind} className="watch-face relative flex min-h-svh flex-col overflow-hidden">
      <div className="watch-vignette pointer-events-none absolute inset-0" aria-hidden="true" />
      <ProductHeader
        product={kind}
        href="/"
        productHref="/"
        trailing={
          <>
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
          </>
        }
      >
        {search}
      </ProductHeader>
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
