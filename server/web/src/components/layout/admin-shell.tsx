import { NavLink } from 'react-router-dom'
import {
  LayoutDashboard,
  Users,
  Laptop,
  HardDrive,
  Settings,
  Globe,
  ScrollText,
  ListChecks,
  Store,
  Shield,
  AtSign,
  GitBranch,
  Server,
  Boxes,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { useAuth } from '@/lib/auth-context'
import { hasAdminProduct, VIEWER_UP_ROLES, type Product, type Role } from '@/lib/roles'
import { SystemChrome } from '@/components/layout/system-chrome'

type AdminNavItem = {
  to: string
  label: string
  icon: typeof LayoutDashboard
  roles: Role[]
  product?: Product
  always?: boolean
  end?: boolean
}

const ADMIN_NAV: { id: string; label: string; product?: Product; items: AdminNavItem[] }[] = [
  {
    id: 'core',
    label: 'Core VPN',
    product: 'core',
    items: [
      { to: '/admin', label: 'Dashboard', icon: LayoutDashboard, roles: VIEWER_UP_ROLES, always: true, end: true },
      { to: '/admin/devices', label: 'Dispositivos', icon: Laptop, roles: VIEWER_UP_ROLES, product: 'core' },
      { to: '/admin/waitlist', label: 'Lista de espera', icon: ListChecks, roles: VIEWER_UP_ROLES, product: 'core' },
      { to: '/admin/settings', label: 'Gerais', icon: Settings, roles: VIEWER_UP_ROLES, product: 'core' },
    ],
  },
  {
    id: 'dns',
    label: 'DNS',
    product: 'dns',
    items: [
      { to: '/admin/dns', label: 'Intranet', icon: Globe, roles: VIEWER_UP_ROLES, product: 'dns' },
      { to: '/admin/dns/public', label: 'Zonas', icon: Globe, roles: VIEWER_UP_ROLES, product: 'dns' },
      { to: '/admin/dns/settings', label: 'Configurações', icon: Settings, roles: VIEWER_UP_ROLES, product: 'dns' },
    ],
  },
  {
    id: 'forge',
    label: 'XGIT',
    product: 'forge',
    items: [
      { to: '/admin/xgit', label: 'Repositórios', icon: GitBranch, roles: VIEWER_UP_ROLES, product: 'forge', end: true },
      { to: '/admin/xgit/settings', label: 'Configurações', icon: Settings, roles: VIEWER_UP_ROLES, product: 'forge' },
    ],
  },
  {
    id: 'compute',
    label: 'Compute',
    product: 'compute',
    items: [
      { to: '/admin/servers', label: 'Servidores', icon: Server, roles: VIEWER_UP_ROLES, product: 'compute' },
      { to: '/admin/compute/settings', label: 'Configurações', icon: Settings, roles: VIEWER_UP_ROLES, product: 'compute' },
    ],
  },
  {
    id: 'managed',
    label: 'Serviços',
    product: 'managed',
    items: [{ to: '/admin/services', label: 'Instâncias', icon: Boxes, roles: VIEWER_UP_ROLES, product: 'managed' }],
  },
  {
    id: 'marketplace',
    label: 'Marketplace',
    product: 'marketplace',
    items: [
      { to: '/admin/marketplace/catalog', label: 'Catálogo', icon: Store, roles: VIEWER_UP_ROLES, product: 'marketplace' },
      { to: '/admin/marketplace/acl', label: 'ACL', icon: Shield, roles: VIEWER_UP_ROLES, product: 'marketplace' },
    ],
  },
  {
    id: 'xgroup',
    label: 'XGROUP',
    product: 'xgroup',
    items: [{ to: '/admin/xgroup', label: 'Rede social', icon: AtSign, roles: VIEWER_UP_ROLES, product: 'xgroup' }],
  },
  {
    id: 'xdriver',
    label: 'XDRIVER',
    product: 'xdriver',
    items: [{ to: '/admin/shares', label: 'Shares e Drive', icon: HardDrive, roles: VIEWER_UP_ROLES, product: 'xdriver' }],
  },
  {
    id: 'iam',
    label: 'IAM',
    items: [
      { to: '/admin/users', label: 'Usuários', icon: Users, roles: VIEWER_UP_ROLES },
      { to: '/admin/rbac', label: 'Papéis', icon: Shield, roles: VIEWER_UP_ROLES },
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
            const items = user
              ? group.items.filter((item) => {
                  if (!item.roles.includes(user.role)) return false
                  if (item.always) return true
                  if (!item.product) return true
                  return hasAdminProduct(user.role, user.products, item.product)
                })
              : []
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
