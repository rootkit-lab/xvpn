import { NavLink } from 'react-router-dom'
import { GitBranch, Home, Laptop, Store } from 'lucide-react'
import { MARKETPLACE_ORIGIN, XADMIN_CORP_ORIGIN } from '@/lib/product-host'
import { cn } from '@/lib/utils'
import { SystemChrome } from '@/components/layout/system-chrome'

export function UserShell() {
  return (
    <SystemChrome
      variant="user"
      subtitle="xvpn"
      nav={
        <nav className="flex flex-1 flex-col gap-1 overflow-y-auto px-3 py-2">
          <NavLink to="/" end className={({ isActive }) => cn('nav-link', isActive && 'nav-link-active')}>
            <Home className="size-4" />
            Início
          </NavLink>
          <NavLink to="/my/devices" className={({ isActive }) => cn('nav-link', isActive && 'nav-link-active')}>
            <Laptop className="size-4" />
            Dispositivos
          </NavLink>
          <a href={`${XADMIN_CORP_ORIGIN}/admin/xgit`} className="nav-link">
            <GitBranch className="size-4" />
            XGIT
          </a>
          <a href={MARKETPLACE_ORIGIN} className="nav-link" target="_blank" rel="noreferrer">
            <Store className="size-4" />
            Marketplace
          </a>
        </nav>
      }
    />
  )
}
