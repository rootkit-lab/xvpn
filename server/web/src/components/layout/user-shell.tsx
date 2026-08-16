import { NavLink } from 'react-router-dom'
import { Home, Store } from 'lucide-react'
import { cn } from '@/lib/utils'
import { SystemChrome } from '@/components/layout/system-chrome'

const USER_NAV = [
  { to: '/my', label: 'Início', icon: Home, end: true },
  { to: '/my/marketplace', label: 'Marketplace', icon: Store, end: false },
] as const

export function UserShell() {
  return (
    <SystemChrome
      variant="user"
      subtitle="xvpn"
      nav={
        <nav className="flex flex-1 flex-col gap-1 overflow-y-auto px-3 py-2">
          {USER_NAV.map(({ to, label, icon: Icon, end }) => (
            <NavLink
              key={to}
              to={to}
              end={end}
              className={({ isActive }) => cn('nav-link', isActive && 'nav-link-active')}
            >
              <Icon className="size-4" />
              {label}
            </NavLink>
          ))}
        </nav>
      }
    />
  )
}
