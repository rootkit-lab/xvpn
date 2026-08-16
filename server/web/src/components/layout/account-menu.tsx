import { Link, useNavigate } from 'react-router-dom'
import { LayoutGrid, LogOut, Settings2, UserRound } from 'lucide-react'
import { useAuth } from '@/lib/auth-context'
import { isViewerUpRole, ROLE_BADGE_VARIANT, ROLE_LABELS } from '@/lib/roles'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

export function ProductSwitcher({ variant }: { variant: 'user' | 'admin' | 'social' }) {
  const { user } = useAuth()
  const showAdmin = isViewerUpRole(user?.role)
  const current = variant

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" title="Produtos XVPN">
          <LayoutGrid className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-52">
        <DropdownMenuLabel>Produtos</DropdownMenuLabel>
        <DropdownMenuItem asChild>
          <Link to="/my">{current === 'user' ? '✓ ' : ''}xvpn</Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link to="/social">{current === 'social' ? '✓ ' : ''}Social</Link>
        </DropdownMenuItem>
        {showAdmin && (
          <DropdownMenuItem asChild>
            <Link to="/admin">{current === 'admin' ? '✓ ' : ''}Administração</Link>
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

export function AccountMenu({ variant }: { variant: 'user' | 'admin' | 'social' }) {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  if (!user) return null

  const loginPath = variant === 'admin' ? '/admin/login' : '/my/login'

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          className="flex items-center gap-2 rounded-full border border-white/8 bg-background/40 px-3 py-1.5"
        >
          <span className="max-w-28 truncate text-sm font-medium">{user.username}</span>
          <Badge variant={ROLE_BADGE_VARIANT[user.role]}>{ROLE_LABELS[user.role]}</Badge>
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-56">
        <DropdownMenuLabel>Conta</DropdownMenuLabel>
        <DropdownMenuItem asChild>
          <Link to={`/social/u/${user.username}`}>
            <UserRound className="size-4" />
            Perfil social
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link to="/my/account">
            <Settings2 className="size-4" />
            Conta
          </Link>
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
