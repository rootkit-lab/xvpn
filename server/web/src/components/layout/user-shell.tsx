import { NavLink, Outlet, useLocation, Link, useNavigate } from 'react-router-dom'
import { AnimatePresence, motion } from 'framer-motion'
import { Download, Home, LogOut, Settings2, Store } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { useAuth } from '@/lib/auth-context'
import { isViewerUpRole, ROLE_BADGE_VARIANT, ROLE_LABELS } from '@/lib/roles'

const USER_NAV = [
  { to: '/app', label: 'Início', icon: Home, end: true },
  { to: '/app/download', label: 'Downloads', icon: Download, end: false },
  { to: '/app/marketplace', label: 'Apps', icon: Store, end: false },
] as const

/** Shell do painel do usuário — autosserviço, sem chrome de ops. */
export function UserShell() {
  const { user, logout } = useAuth()
  const location = useLocation()
  const navigate = useNavigate()
  const showAdminLink = isViewerUpRole(user?.role)

  return (
    <div className="relative flex min-h-svh w-full bg-background">
      <div
        className="pointer-events-none fixed inset-0 opacity-80"
        style={{
          background:
            'radial-gradient(90% 60% at 10% 0%, color-mix(in oklch, var(--primary) 18%, transparent), transparent 55%), radial-gradient(70% 50% at 90% 100%, color-mix(in oklch, var(--primary) 10%, transparent), transparent 50%)',
        }}
      />
      <aside className="relative z-10 flex w-60 shrink-0 flex-col border-r border-white/8 bg-card/50 backdrop-blur-xl">
        <div className="flex items-center gap-2.5 px-5 py-5">
          <img src="/logo-192.png" alt="XVPN" className="size-8 rounded-[10px]" />
          <div className="min-w-0">
            <span className="block text-base font-semibold tracking-tight">XVPN</span>
            <span className="text-[11px] text-muted-foreground">Meu espaço</span>
          </div>
        </div>
        <nav className="flex flex-1 flex-col gap-1 px-3 py-2">
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
        <div className="space-y-1 border-t border-white/8 p-3">
          {user && (
            <div className="flex items-center justify-between gap-2 px-3 py-2">
              <span className="truncate text-sm font-medium" title={user.username}>
                {user.username}
              </span>
              <Badge variant={ROLE_BADGE_VARIANT[user.role]}>{ROLE_LABELS[user.role]}</Badge>
            </div>
          )}
          {showAdminLink && (
            <Button variant="ghost" className="w-full justify-start gap-3 rounded-xl" asChild>
              <Link to="/admin">
                <Settings2 className="size-4" />
                Administração
              </Link>
            </Button>
          )}
          <Button
            variant="ghost"
            className="w-full justify-start gap-3 rounded-xl"
            onClick={() => {
              logout()
              navigate('/app/login', { replace: true })
            }}
          >
            <LogOut className="size-4" />
            Sair
          </Button>
        </div>
      </aside>
      <main className="relative z-10 flex-1 overflow-y-auto p-6 md:p-8">
        <AnimatePresence mode="wait">
          <motion.div
            key={location.pathname}
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -6 }}
            transition={{ duration: 0.22, ease: 'easeOut' }}
          >
            <Outlet />
          </motion.div>
        </AnimatePresence>
      </main>
    </div>
  )
}
