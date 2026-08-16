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
