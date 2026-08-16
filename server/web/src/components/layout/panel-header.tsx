import { Link, useLocation } from 'react-router-dom'
import { Shield, Users } from 'lucide-react'
import { ProductHeader } from '@xvpn/ui/react/product-header'
import { pageMetaForPath } from '@/lib/page-meta'
import { useAuth } from '@/lib/auth-context'
import { Button } from '@/components/ui/button'
import { AccountMenu, ProductSwitcher } from '@/components/layout/account-menu'

/** Header fixo — marca ihuull + produto + título da rota + menu da conta. */
export function PanelHeader({ variant }: { variant: 'user' | 'admin' | 'social' }) {
  const { user } = useAuth()
  const location = useLocation()
  const meta = pageMetaForPath(location.pathname)
  const title = location.pathname === '/my' && user ? `Olá, ${user.username}` : meta.title
  const product = variant === 'social' ? 'xgroup' : 'xvpn'
  const productHref = variant === 'social' ? '/social' : variant === 'admin' ? '/admin' : '/my'

  return (
    <ProductHeader
      product={product}
      href="/"
      productHref={productHref}
      trailing={
        <>
          <HeaderActions pathname={location.pathname} />
          <ProductSwitcher variant={variant} />
          <AccountMenu variant={variant} />
        </>
      }
    >
      <div className="min-w-0">
        <p className="hud-label text-muted-foreground/70">
          {variant === 'admin' ? `// ${meta.kicker.toLowerCase()}` : meta.kicker}
        </p>
        <div className="flex flex-wrap items-baseline gap-x-3 gap-y-0.5">
          <h1 className="font-display truncate text-lg font-semibold tracking-tight">{title}</h1>
          {meta.description && (
            <p className="hidden truncate font-display text-sm text-muted-foreground xl:block">{meta.description}</p>
          )}
        </div>
      </div>
    </ProductHeader>
  )
}

function HeaderActions({ pathname }: { pathname: string }) {
  if (pathname === '/admin/rbac') {
    return (
      <Button size="sm" variant="outline" asChild>
        <Link to="/admin/users">
          <Users className="size-4" />
          Usuários
        </Link>
      </Button>
    )
  }
  if (pathname === '/admin/users' || pathname.startsWith('/admin/users/')) {
    return (
      <Button size="sm" variant="outline" asChild>
        <Link to="/admin/rbac">
          <Shield className="size-4" />
          Papéis
        </Link>
      </Button>
    )
  }
  return null
}
