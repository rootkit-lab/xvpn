import { NavLink, Outlet, useLocation } from 'react-router-dom'
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
  { to: '/download', label: 'Downloads', icon: Download },
  { to: '/settings', label: 'Configurações', icon: Settings },
  { to: '/audit', label: 'Auditoria', icon: ScrollText },
]

export function AppShell() {
  const { logout } = useAuth()
  const location = useLocation()

  return (
    <div className="flex min-h-svh w-full bg-background">
      <aside className="flex w-64 shrink-0 flex-col border-r border-white/5 bg-card/60 backdrop-blur">
        <div className="flex items-center gap-2 px-6 py-5">
          <img src="/logo-192.png" alt="XVPN" className="size-8 drop-shadow-[0_0_12px_var(--color-glow)]" />
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
                  'flex items-center gap-3 rounded-full px-3 py-2 text-sm font-medium transition-all',
                  isActive
                    ? 'bg-primary text-primary-foreground shadow-[0_0_20px_-4px_var(--color-glow)]'
                    : 'text-muted-foreground hover:bg-white/5 hover:text-foreground',
                )
              }
            >
              <Icon className="size-4" />
              {label}
            </NavLink>
          ))}
        </nav>
        <div className="border-t border-white/5 p-3">
          <Button variant="ghost" className="w-full justify-start gap-3 rounded-full" onClick={logout}>
            <LogOut className="size-4" />
            Sair
          </Button>
        </div>
      </aside>
      <main className="relative flex-1 overflow-y-auto p-8">
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
