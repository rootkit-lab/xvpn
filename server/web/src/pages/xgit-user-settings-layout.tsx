import { NavLink, Outlet } from 'react-router-dom'
import { KeyRound } from 'lucide-react'
import { useAuth } from '@/lib/auth-context'
import { cn } from '@/lib/utils'

const NAV = [
  { to: '/settings/ssh', label: 'SSH and GPG keys', icon: KeyRound },
] as const

export function XgitUserSettingsLayout() {
  const { user } = useAuth()

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-8 lg:flex-row">
      <aside className="w-full shrink-0 lg:w-56">
        <div className="mb-4">
          <p className="text-sm font-semibold">{user?.username ?? 'Conta'}</p>
          <p className="text-xs text-muted-foreground">Your personal account</p>
        </div>
        <nav className="flex flex-col gap-1">
          <p className="px-2 py-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">Access</p>
          {NAV.map(({ to, label, icon: Icon }) => (
            <NavLink
              key={to}
              to={to}
              className={({ isActive }) =>
                cn(
                  'flex items-center gap-2 rounded-md px-2 py-1.5 text-sm',
                  isActive ? 'bg-accent font-medium text-accent-foreground' : 'text-muted-foreground hover:bg-accent/50',
                )
              }
            >
              <Icon className="size-4" />
              {label}
            </NavLink>
          ))}
        </nav>
      </aside>
      <div className="min-w-0 flex-1">
        <Outlet />
      </div>
    </div>
  )
}
