import { NavLink } from 'react-router-dom'
import { MessageSquare, Users, UsersRound } from 'lucide-react'
import { cn } from '@/lib/utils'
import { SystemChrome } from '@/components/layout/system-chrome'

const SOCIAL_NAV = [
  { to: '/social', label: 'Pessoas', icon: Users, end: true },
  { to: '/social/messages', label: 'Mensagens', icon: MessageSquare, end: false },
  { to: '/social/groups', label: 'Grupos', icon: UsersRound, end: false },
] as const

export function SocialShell() {
  return (
    <SystemChrome
      variant="social"
      subtitle="Social"
      nav={
        <nav className="flex flex-1 flex-col gap-1 overflow-y-auto px-3 py-2">
          {SOCIAL_NAV.map(({ to, label, icon: Icon, end }) => (
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
