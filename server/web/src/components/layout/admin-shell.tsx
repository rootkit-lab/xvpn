import { NavLink, Outlet, useLocation, Link, useNavigate } from 'react-router-dom'
import { AnimatePresence, motion } from 'framer-motion'
import {
  LayoutDashboard,
  Users,
  Laptop,
  HardDrive,
  Settings,
  ScrollText,
  LogOut,
  ListChecks,
  Download,
  Store,
  UserRound,
  Shield,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { useAuth } from '@/lib/auth-context'
import { ROLE_BADGE_VARIANT, ROLE_LABELS, VIEWER_UP_ROLES, type Role } from '@/lib/roles'

const ADMIN_NAV: { to: string; label: string; icon: typeof LayoutDashboard; roles: Role[] }[] = [
  { to: '/admin', label: 'Dashboard', icon: LayoutDashboard, roles: VIEWER_UP_ROLES },
  { to: '/admin/users', label: 'Usuários', icon: Users, roles: VIEWER_UP_ROLES },
  { to: '/admin/rbac', label: 'Papéis', icon: Shield, roles: VIEWER_UP_ROLES },
  { to: '/admin/devices', label: 'Dispositivos', icon: Laptop, roles: VIEWER_UP_ROLES },
  { to: '/admin/shares', label: 'Compartilhamentos', icon: HardDrive, roles: VIEWER_UP_ROLES },
  { to: '/admin/waitlist', label: 'Lista de espera', icon: ListChecks, roles: VIEWER_UP_ROLES },
  { to: '/admin/download', label: 'Downloads', icon: Download, roles: VIEWER_UP_ROLES },
  { to: '/admin/marketplace', label: 'Marketplace', icon: Store, roles: VIEWER_UP_ROLES },
  { to: '/admin/settings', label: 'Configurações', icon: Settings, roles: VIEWER_UP_ROLES },
  { to: '/admin/audit', label: 'Auditoria', icon: ScrollText, roles: VIEWER_UP_ROLES },
]

/** Shell da administração do sistema — chrome operacional (viewer+). */
export function AdminShell() {
  const { user, logout } = useAuth()
  const location = useLocation()
  const navigate = useNavigate()
  const items = user ? ADMIN_NAV.filter((item) => item.roles.includes(user.role)) : []

  return (
    <div className="relative flex min-h-svh w-full bg-background">
      <div className="dot-grid pointer-events-none fixed inset-0 opacity-60" />
      <aside className="cyber-frame relative z-10 flex w-64 shrink-0 flex-col border-r border-white/5 bg-card/70 backdrop-blur">
        <div className="flex items-center gap-2 px-6 py-5">
          <img src="/logo-192.png" alt="XVPN" className="size-8 drop-shadow-[0_0_12px_var(--color-glow)]" />
          <div className="min-w-0">
            <span className="block text-lg font-semibold tracking-tight">XVPN</span>
            <span className="hud-label text-muted-foreground/70">administração</span>
          </div>
        </div>
        <div className="scanline mx-3" />
        <nav className="flex flex-1 flex-col gap-1 px-3 py-4">
          <span className="hud-label mb-2 px-3 text-muted-foreground/60">// sistema</span>
          {items.map(({ to, label, icon: Icon }) => (
            <NavLink
              key={to}
              to={to}
              end={to === '/admin'}
              className={({ isActive }) =>
                cn(
                  'group relative flex items-center gap-3 rounded-md px-3 py-2 font-mono text-[0.8125rem] tracking-wide transition-all',
                  isActive
                    ? 'bg-primary/15 text-primary shadow-[inset_1px_0_0_var(--color-primary)]'
                    : 'text-muted-foreground hover:bg-white/5 hover:text-foreground',
                )
              }
            >
              {({ isActive }) => (
                <>
                  <Icon className="size-4" />
                  <span>{label}</span>
                  {isActive && (
                    <span className="absolute right-2 top-1/2 size-1.5 -translate-y-1/2 rounded-full bg-primary shadow-[0_0_8px_var(--color-glow)]" />
                  )}
                </>
              )}
            </NavLink>
          ))}
        </nav>
        <div className="border-t border-white/5 p-3">
          {user && (
            <div className="flex items-center justify-between gap-2 px-3 py-2">
              <span className="truncate font-mono text-sm" title={user.username}>
                {user.username}
              </span>
              <Badge variant={ROLE_BADGE_VARIANT[user.role]}>{ROLE_LABELS[user.role]}</Badge>
            </div>
          )}
          <Button variant="ghost" className="w-full justify-start gap-3 rounded-md font-mono" asChild>
            <Link to="/app">
              <UserRound className="size-4" />
              Meu espaço
            </Link>
          </Button>
          <Button
            variant="ghost"
            className="w-full justify-start gap-3 rounded-md font-mono"
            onClick={() => {
              logout()
              navigate('/admin/login', { replace: true })
            }}
          >
            <LogOut className="size-4" />
            Sair
          </Button>
        </div>
      </aside>
      <main className="relative z-10 flex-1 overflow-y-auto p-8">
        <AnimatePresence mode="wait">
          <motion.div
            key={location.pathname}
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -8 }}
            transition={{ duration: 0.2, ease: 'easeOut' }}
          >
            <Outlet />
          </motion.div>
        </AnimatePresence>
      </main>
    </div>
  )
}
