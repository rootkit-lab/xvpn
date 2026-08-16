import { Link, useNavigate } from 'react-router-dom'
import { LogOut, Settings2, UserRound } from 'lucide-react'
import { useAuth } from '@/lib/auth-context'
import { ROLE_BADGE_VARIANT, ROLE_LABELS } from '@/lib/roles'
import { PANEL_ORIGIN, XGROUP_CORP_ORIGIN, productKind, ssoLogoutURL } from '@/lib/product-host'
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

export function AccountMenu({ variant: _variant }: { variant: 'user' | 'admin' | 'social' }) {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  if (!user) return null

  const kind = productKind()
  const onPanel = kind === 'xvpn'
  const onGroup = kind === 'xgroup-corp'
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
          {onGroup || onPanel ? (
            <Link to={profilePath}>
              <UserRound className="size-4" />
              Perfil social
            </Link>
          ) : (
            <a href={`${XGROUP_CORP_ORIGIN}${profilePath}`}>
              <UserRound className="size-4" />
              Perfil social
            </a>
          )}
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          {onPanel ? (
            <Link to={accountPath}>
              <Settings2 className="size-4" />
              Conta
            </Link>
          ) : (
            <a href={`${PANEL_ORIGIN}${accountPath}`}>
              <Settings2 className="size-4" />
              Conta
            </a>
          )}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          onClick={() => {
            void (async () => {
              await logout()
              if (productKind() === 'xauth') {
                navigate('/login?logged_out=1', { replace: true })
                return
              }
              window.location.replace(ssoLogoutURL())
            })()
          }}
        >
          <LogOut className="size-4" />
          Sair
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
