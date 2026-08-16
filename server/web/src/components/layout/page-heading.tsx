import { Link, useLocation } from 'react-router-dom'
import { Shield, Users } from 'lucide-react'
import { pageMetaForPath } from '@/lib/page-meta'
import { useAuth } from '@/lib/auth-context'
import { Button } from '@/components/ui/button'

/** Título da rota — vive no template do app, não no chrome de sistema. */
export function PageHeading({ variant }: { variant: 'user' | 'admin' | 'social' }) {
  const { user } = useAuth()
  const location = useLocation()
  const meta = pageMetaForPath(location.pathname)
  const title = location.pathname === '/my' && user ? `Olá, ${user.username}` : meta.title

  return (
    <div className="mb-6 flex flex-wrap items-start justify-between gap-3">
      <div className="min-w-0">
        <p className="hud-label text-muted-foreground/65">
          {variant === 'admin' ? `// ${meta.kicker.toLowerCase()}` : meta.kicker}
        </p>
        <h1 className="font-display mt-0.5 text-2xl font-semibold tracking-tight">{title}</h1>
        {meta.description && (
          <p className="mt-1 max-w-xl text-sm leading-relaxed text-muted-foreground">{meta.description}</p>
        )}
      </div>
      <PageActions pathname={location.pathname} />
    </div>
  )
}

function PageActions({ pathname }: { pathname: string }) {
  if (pathname === '/admin/rbac') {
    return (
      <Button size="sm" variant="outline" className="rounded-full" asChild>
        <Link to="/admin/users">
          <Users className="size-4" />
          Usuários
        </Link>
      </Button>
    )
  }
  if (pathname === '/admin/users' || pathname.startsWith('/admin/users/')) {
    return (
      <Button size="sm" variant="outline" className="rounded-full" asChild>
        <Link to="/admin/rbac">
          <Shield className="size-4" />
          Papéis
        </Link>
      </Button>
    )
  }
  return null
}
