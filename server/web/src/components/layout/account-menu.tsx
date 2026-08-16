import { Link, useNavigate } from 'react-router-dom'
import { LogOut, Settings2, UserRound } from 'lucide-react'
import { useAuth } from '@/lib/auth-context'
import { ROLE_BADGE_VARIANT, ROLE_LABELS } from '@/lib/roles'
import { PANEL_ORIGIN, isStoreHost } from '@/lib/product-host'
import { Badge } from '@/components/ui/badge'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

export { AppLauncher as ProductSwitcher } from '@/components/layout/app-launcher'

export function AccountMenu({ variant }: { variant: 'user' | 'admin' | 'social' }) {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  if (!user) return null

  const store = isStoreHost()
  const loginPath = variant === 'admin' ? '/admin/login' : store ? '/login' : '/my/login'
  const profilePath = `/social/u/${user.username}`
  const accountPath = '/my/account'

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button type="button" className="icon-well flex items-center gap-2 rounded-full px-3 py-1.5">
          <span className="max-w-28 truncate text-sm font-medium">{user.username}</span>
          <Badge variant={ROLE_BADGE_VARIANT[user.role]}>{ROLE_LABELS[user.role]}</Badge>
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-56">
        <DropdownMenuLabel>Conta</DropdownMenuLabel>
        <DropdownMenuItem asChild>
          {store ? (
            <a href={`${PANEL_ORIGIN}${profilePath}`}>
              <UserRound className="size-4" />
              Perfil social
            </a>
          ) : (
            <Link to={profilePath}>
              <UserRound className="size-4" />
              Perfil social
            </Link>
          )}
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          {store ? (
            <a href={`${PANEL_ORIGIN}${accountPath}`}>
              <Settings2 className="size-4" />
              Conta
            </a>
          ) : (
            <Link to={accountPath}>
              <Settings2 className="size-4" />
              Conta
            </Link>
          )}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          onClick={() => {
            logout()
            navigate(loginPath, { replace: true })
          }}
        >
          <LogOut className="size-4" />
          Sair
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
