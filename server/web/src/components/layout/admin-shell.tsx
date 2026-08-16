import { NavLink } from 'react-router-dom'
import {
  LayoutDashboard,
  Users,
  Laptop,
  HardDrive,
  Settings,
  ScrollText,
  ListChecks,
  Store,
  Shield,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { useAuth } from '@/lib/auth-context'
import { VIEWER_UP_ROLES, type Role } from '@/lib/roles'
import { SystemChrome } from '@/components/layout/system-chrome'

type AdminNavItem = { to: string; label: string; icon: typeof LayoutDashboard; roles: Role[]; end?: boolean }

const ADMIN_NAV: { id: string; label: string; items: AdminNavItem[] }[] = [
  {
    id: 'core',
    label: 'Core',
    items: [
      { to: '/admin', label: 'Dashboard', icon: LayoutDashboard, roles: VIEWER_UP_ROLES, end: true },
      { to: '/admin/users', label: 'Usuários', icon: Users, roles: VIEWER_UP_ROLES },
      { to: '/admin/rbac', label: 'Papéis', icon: Shield, roles: VIEWER_UP_ROLES },
      { to: '/admin/devices', label: 'Dispositivos', icon: Laptop, roles: VIEWER_UP_ROLES },
      { to: '/admin/waitlist', label: 'Lista de espera', icon: ListChecks, roles: VIEWER_UP_ROLES },
    ],
  },
  {
    id: 'apps',
    label: 'Apps',
    items: [
      { to: '/admin/marketplace', label: 'Marketplace', icon: Store, roles: VIEWER_UP_ROLES },
      { to: '/admin/shares', label: 'XDriver', icon: HardDrive, roles: VIEWER_UP_ROLES },
    ],
  },
  {
    id: 'settings',
    label: 'Settings',
    items: [
      { to: '/admin/settings', label: 'Gerais', icon: Settings, roles: VIEWER_UP_ROLES },
      { to: '/admin/audit', label: 'Auditoria', icon: ScrollText, roles: VIEWER_UP_ROLES },
    ],
  },
]

export function AdminShell() {
  const { user } = useAuth()

  return (
    <SystemChrome
      variant="admin"
      subtitle="administração"
      nav={
        <nav className="flex flex-1 flex-col gap-4 overflow-y-auto px-3 py-4">
          {ADMIN_NAV.map((group) => {
            const items = user ? group.items.filter((item) => item.roles.includes(user.role)) : []
            if (items.length === 0) return null
            return (
              <div key={group.id} className="flex flex-col gap-1">
                <span className="hud-label mb-1 px-3 text-muted-foreground/60">{group.label}</span>
                {items.map(({ to, label, icon: Icon, end }) => (
                  <NavLink
                    key={to}
                    to={to}
                    end={end}
                    className={({ isActive }) => cn('nav-link relative', isActive && 'nav-link-active')}
                  >
                    {({ isActive }) => (
                      <>
                        <Icon className="size-4" />
                        <span>{label}</span>
                        {isActive && (
                          <span className="status-safe-dot absolute right-2 top-1/2 size-1.5 -translate-y-1/2 rounded-full" />
                        )}
                      </>
                    )}
                  </NavLink>
                ))}
              </div>
            )
          })}
        </nav>
      }
    />
  )
}
