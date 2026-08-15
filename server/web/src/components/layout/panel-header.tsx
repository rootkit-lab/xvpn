import { Link, useLocation } from 'react-router-dom'
import { ExternalLink, Pencil, Shield, Users } from 'lucide-react'
import { pageMetaForPath } from '@/lib/page-meta'
import { useAuth } from '@/lib/auth-context'
import { ROLE_BADGE_VARIANT, ROLE_LABELS } from '@/lib/roles'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

const RELEASES_URL = 'https://github.com/rootkit-lab/xvpn/releases'

/** Header fixo — título da rota + ações contextuais + identidade. */
export function PanelHeader({ variant }: { variant: 'user' | 'admin' }) {
  const { user } = useAuth()
  const location = useLocation()
  const meta = pageMetaForPath(location.pathname)
  const title = location.pathname === '/app' && user ? `Olá, ${user.username}` : meta.title

  return (
    <header
      className={cn(
        'flex shrink-0 items-center gap-4 border-b px-6 py-3',
        variant === 'admin'
          ? 'border-white/5 bg-card/80 backdrop-blur'
          : 'border-white/8 bg-card/60 backdrop-blur-xl',
      )}
    >
      <div className="min-w-0 flex-1">
        <p
          className={cn(
            'text-[11px] font-medium uppercase tracking-[0.14em] text-muted-foreground/70',
            variant === 'admin' && 'hud-label',
          )}
        >
          {variant === 'admin' ? `// ${meta.kicker.toLowerCase()}` : meta.kicker}
        </p>
        <div className="flex flex-wrap items-baseline gap-x-3 gap-y-0.5">
          <h1 className="truncate text-lg font-semibold tracking-tight">{title}</h1>
          {meta.description && (
            <p className="hidden truncate text-sm text-muted-foreground lg:block">{meta.description}</p>
          )}
        </div>
      </div>
      <div className="flex shrink-0 items-center gap-2">
        <HeaderActions pathname={location.pathname} />
        {user && (
          <Link
            to="/app/profile"
            className="hidden items-center gap-2 rounded-full border border-white/8 bg-background/40 px-3 py-1.5 sm:flex"
            title="Abrir perfil"
          >
            <span className="max-w-28 truncate text-sm font-medium">{user.username}</span>
            <Badge variant={ROLE_BADGE_VARIANT[user.role]}>{ROLE_LABELS[user.role]}</Badge>
          </Link>
        )}
      </div>
    </header>
  )
}

function HeaderActions({ pathname }: { pathname: string }) {
  if (pathname === '/app/profile') {
    return (
      <Button size="sm" asChild>
        <Link to="/app/account">
          <Pencil className="size-4" />
          Editar minha conta
        </Link>
      </Button>
    )
  }
  if (pathname === '/app/download' || pathname === '/admin/download') {
    return (
      <Button size="sm" variant="outline" asChild>
        <a href={RELEASES_URL} target="_blank" rel="noreferrer">
          GitHub Releases
          <ExternalLink className="size-4" />
        </a>
      </Button>
    )
  }
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
  if (pathname === '/admin/users') {
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
