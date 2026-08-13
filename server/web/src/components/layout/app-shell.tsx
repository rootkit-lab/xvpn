import { NavLink, Outlet } from 'react-router-dom'
import {
  LayoutDashboard,
  Users,
  Laptop,
  HardDrive,
  Settings,
  ScrollText,
  LogOut,
  ListChecks,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { useAuth } from '@/lib/auth-context'

const NAV_ITEMS = [
  { to: '/dashboard', label: 'Dashboard', icon: LayoutDashboard },
  { to: '/users', label: 'Usuários', icon: Users },
  { to: '/devices', label: 'Dispositivos', icon: Laptop },
  { to: '/shares', label: 'Compartilhamentos', icon: HardDrive },
  { to: '/waitlist', label: 'Lista de espera', icon: ListChecks },
  { to: '/settings', label: 'Configurações', icon: Settings },
  { to: '/audit', label: 'Auditoria', icon: ScrollText },
]

export function AppShell() {
  const { logout } = useAuth()

  return (
    <div className="flex min-h-svh w-full">
      <aside className="flex w-64 shrink-0 flex-col border-r bg-card">
        <div className="flex items-center gap-2 px-6 py-5">
          <img src="/logo-192.png" alt="XVPN" className="size-8" />
          <span className="text-lg font-semibold">XVPN</span>
        </div>
        <nav className="flex flex-1 flex-col gap-1 px-3">
          {NAV_ITEMS.map(({ to, label, icon: Icon }) => (
            <NavLink
              key={to}
              to={to}
              end
              className={({ isActive }) =>
                cn(
                  'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors',
                  isActive
                    ? 'bg-primary text-primary-foreground'
                    : 'text-muted-foreground hover:bg-muted hover:text-foreground',
                )
              }
            >
              <Icon className="size-4" />
              {label}
            </NavLink>
          ))}
        </nav>
        <div className="border-t p-3">
          <Button variant="ghost" className="w-full justify-start gap-3" onClick={logout}>
            <LogOut className="size-4" />
            Sair
          </Button>
        </div>
      </aside>
      <main className="flex-1 overflow-y-auto bg-background p-8">
        <Outlet />
      </main>
    </div>
  )
}
