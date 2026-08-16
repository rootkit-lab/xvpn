import { NavLink } from 'react-router-dom'
import { Download, FolderOpen, Home, Store } from 'lucide-react'
import { cn } from '@/lib/utils'
import { SystemChrome } from '@/components/layout/system-chrome'

const USER_NAV = [
  { to: '/my', label: 'Início', icon: Home, end: true },
  { to: '/my/files', label: 'Arquivos', icon: FolderOpen, end: false },
  { to: '/my/download', label: 'Downloads', icon: Download, end: false },
  { to: '/my/marketplace', label: 'Apps', icon: Store, end: false },
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
              className={({ isActive }) =>
                cn(
                  'flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium transition-colors',
                  isActive
                    ? 'bg-primary/15 text-primary'
                    : 'text-muted-foreground hover:bg-white/5 hover:text-foreground',
                )
              }
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
