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
  Store,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { useAuth } from '@/lib/auth-context'
import { ROLE_BADGE_VARIANT, ROLE_LABELS, VIEWER_UP_ROLES, type Role } from '@/lib/roles'

// roles: quem vê o item na navegação — ver PLAN.md §6.7. member só enxerga
// "Meus dispositivos", "Downloads" e "Marketplace"; viewer/admin/super_admin
// veem as telas administrativas completas (a diferença entre eles é dentro
// de cada página, não na navegação — ver users-page.tsx/waitlist-page.tsx).
// Marketplace (Fase 11) é catálogo de terceiros para todo mundo navegar —
// admin/super_admin ganham os controles de publicar dentro da própria
// página (ver marketplace-page.tsx), não uma rota separada.
const NAV_ITEMS: { to: string; label: string; icon: typeof LayoutDashboard; roles: Role[] }[] = [
  { to: '/dashboard', label: 'Dashboard', icon: LayoutDashboard, roles: VIEWER_UP_ROLES },
  { to: '/users', label: 'Usuários', icon: Users, roles: VIEWER_UP_ROLES },
  { to: '/devices', label: 'Dispositivos', icon: Laptop, roles: VIEWER_UP_ROLES },
  { to: '/portal', label: 'Meus dispositivos', icon: Laptop, roles: ['member'] },
  { to: '/shares', label: 'Compartilhamentos', icon: HardDrive, roles: VIEWER_UP_ROLES },
  { to: '/waitlist', label: 'Lista de espera', icon: ListChecks, roles: VIEWER_UP_ROLES },
  { to: '/download', label: 'Downloads', icon: Download, roles: ['super_admin', 'admin', 'viewer', 'member'] },
  { to: '/marketplace', label: 'Marketplace', icon: Store, roles: ['super_admin', 'admin', 'viewer', 'member'] },
  { to: '/settings', label: 'Configurações', icon: Settings, roles: VIEWER_UP_ROLES },
  { to: '/audit', label: 'Auditoria', icon: ScrollText, roles: VIEWER_UP_ROLES },
]

export function AppShell() {
  const { user, logout } = useAuth()
  const location = useLocation()

  const items = user ? NAV_ITEMS.filter((item) => item.roles.includes(user.role)) : []

  return (
    <div className="relative flex min-h-svh w-full bg-background">
      {/* dot-grid atrás de tudo — assinatura técnica do fundo das telas
          logadas. Pointer-events-none pra não capturar cliques. */}
      <div className="dot-grid pointer-events-none fixed inset-0 opacity-60" />
      <aside className="cyber-frame relative z-10 flex w-64 shrink-0 flex-col border-r border-white/5 bg-card/70 backdrop-blur">
        <div className="flex items-center gap-2 px-6 py-5">
          <img src="/logo-192.png" alt="XVPN" className="size-8 drop-shadow-[0_0_12px_var(--color-glow)]" />
          <span className="text-lg font-semibold tracking-tight">XVPN</span>
          <span className="hud-label ml-auto text-muted-foreground/70">ctrl</span>
        </div>
        <div className="scanline mx-3" />
        <nav className="flex flex-1 flex-col gap-1 px-3 py-4">
          <span className="hud-label mb-2 px-3 text-muted-foreground/60">// navegação</span>
          {items.map(({ to, label, icon: Icon }) => (
            <NavLink
              key={to}
              to={to}
              end
              className={({ isActive }) =>
                cn(
                  'group relative flex items-center gap-3 rounded-md px-3 py-2 font-mono text-[0.8125rem] tracking-wide transition-all',
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
        <div className="border-t border-white/5 p-3">
          {user && (
            <div className="flex items-center justify-between gap-2 px-3 py-2">
              <span className="truncate font-mono text-sm" title={user.username}>
                {user.username}
              </span>
              <Badge variant={ROLE_BADGE_VARIANT[user.role]}>{ROLE_LABELS[user.role]}</Badge>
            </div>
          )}
          <Button variant="ghost" className="w-full justify-start gap-3 rounded-md font-mono" onClick={logout}>
            <LogOut className="size-4" />
            Sair
          </Button>
        </div>
      </aside>
      <main className="relative z-10 flex-1 overflow-y-auto p-8">
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
