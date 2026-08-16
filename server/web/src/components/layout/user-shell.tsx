import { NavLink } from 'react-router-dom'
import { Home, Store } from 'lucide-react'
import { MARKETPLACE_ORIGIN } from '@/lib/product-host'
import { cn } from '@/lib/utils'
import { SystemChrome } from '@/components/layout/system-chrome'

export function UserShell() {
  return (
    <SystemChrome
      variant="user"
      subtitle="xvpn"
      nav={
        <nav className="flex flex-1 flex-col gap-1 overflow-y-auto px-3 py-2">
          <NavLink to="/my" end className={({ isActive }) => cn('nav-link', isActive && 'nav-link-active')}>
            <Home className="size-4" />
            Início
          </NavLink>
          <a href={MARKETPLACE_ORIGIN} className="nav-link" target="_blank" rel="noreferrer">
            <Store className="size-4" />
            Marketplace
          </a>
        </nav>
      }
    />
  )
}
