import { NavLink } from 'react-router-dom'
import {
  LayoutDashboard,
  Users,
  Laptop,
  HardDrive,
  Settings,
  ScrollText,
  ListChecks,
  Download,
  Store,
  Shield,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { useAuth } from '@/lib/auth-context'
import { VIEWER_UP_ROLES, type Role } from '@/lib/roles'
import { SystemChrome } from '@/components/layout/system-chrome'

const ADMIN_NAV: { to: string; label: string; icon: typeof LayoutDashboard; roles: Role[] }[] = [
  { to: '/admin', label: 'Dashboard', icon: LayoutDashboard, roles: VIEWER_UP_ROLES },
  { to: '/admin/users', label: 'Usuários', icon: Users, roles: VIEWER_UP_ROLES },
  { to: '/admin/rbac', label: 'Papéis', icon: Shield, roles: VIEWER_UP_ROLES },
  { to: '/admin/devices', label: 'Dispositivos', icon: Laptop, roles: VIEWER_UP_ROLES },
  { to: '/admin/shares', label: 'xdriver', icon: HardDrive, roles: VIEWER_UP_ROLES },
  { to: '/admin/waitlist', label: 'Lista de espera', icon: ListChecks, roles: VIEWER_UP_ROLES },
  { to: '/admin/download', label: 'Downloads', icon: Download, roles: VIEWER_UP_ROLES },
  { to: '/admin/marketplace', label: 'Marketplace', icon: Store, roles: VIEWER_UP_ROLES },
  { to: '/admin/settings', label: 'Configurações', icon: Settings, roles: VIEWER_UP_ROLES },
  { to: '/admin/audit', label: 'Auditoria', icon: ScrollText, roles: VIEWER_UP_ROLES },
]

export function AdminShell() {
  const { user } = useAuth()
  const items = user ? ADMIN_NAV.filter((item) => item.roles.includes(user.role)) : []

  return (
    <SystemChrome
      variant="admin"
      subtitle="administração"
      nav={
        <nav className="flex flex-1 flex-col gap-1 overflow-y-auto px-3 py-4">
          <span className="hud-label mb-2 px-3 text-muted-foreground/60">// sistema</span>
          {items.map(({ to, label, icon: Icon }) => (
            <NavLink
              key={to}
              to={to}
              end={to === '/admin'}
              className={({ isActive }) =>
                cn(
                  'group relative flex items-center gap-3 rounded-[10px] px-3 py-2 font-display text-[0.8125rem] tracking-wide transition-all',
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
      }
    />
  )
}
