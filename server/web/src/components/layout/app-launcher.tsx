import { Link, useLocation } from 'react-router-dom'
import { HardDrive, LayoutDashboard, LayoutGrid, MessageCircle, MessagesSquare, Shield, Store } from 'lucide-react'
import { PRODUCT_META } from '@xvpn/ui/react/products'
import { useAuth } from '@/lib/auth-context'
import { isViewerUpRole } from '@/lib/roles'
import {
  MARKETPLACE_ORIGIN,
  PANEL_ORIGIN,
  XDRIVER_CORP_ORIGIN,
  isStoreHost,
  productKind,
} from '@/lib/product-host'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { cn } from '@/lib/utils'

export type LauncherVariant = 'user' | 'admin' | 'social' | 'marketplace' | 'xdriver'

type LauncherTile = {
  id: string
  label: string
  to?: string
  href?: string
  icon: typeof Store
  current: boolean
}

/** Waffle de apps — sempre visível com sessão. Nomes = PRODUCT_META. */
export function AppLauncher({ variant }: { variant: LauncherVariant }) {
  const { user } = useAuth()
  const location = useLocation()
  const showAdmin = isViewerUpRole(user?.role)
  const xdriverOn = Boolean(user?.samba_enabled || user?.sftp_enabled)
  const kind = productKind()
  const store = isStoreHost()
  const onMarketplace = kind === 'marketplace' || location.pathname.endsWith('/marketplace')
  const onXDriver =
    kind === 'xdriver' || kind === 'xdriver-corp' || location.pathname === '/my/files' || location.pathname.startsWith('/admin/shares')
  const onMessages = location.pathname.includes('/messages')
  const onSocial = variant === 'social' || location.pathname.startsWith('/social') || location.pathname.startsWith('/xgroup')

  const panel = (path: string) => (store ? { href: `${PANEL_ORIGIN}${path}` } : { to: path })

  const installed: LauncherTile[] = [
    {
      id: 'xvpn',
      label: PRODUCT_META.xvpn.label,
      ...panel('/'),
      icon: Shield,
      current: kind === 'xvpn' && variant === 'user' && !onMarketplace && !onXDriver,
    },
    {
      id: 'xgroup',
      label: PRODUCT_META.xgroup.label,
      ...panel('/social'),
      icon: MessageCircle,
      current: onSocial && !onMessages,
    },
    {
      id: 'xchat',
      label: PRODUCT_META.xchat.label,
      ...panel('/social/messages'),
      icon: MessagesSquare,
      current: onMessages,
    },
    ...(xdriverOn
      ? [
          {
            id: 'xdriver',
            label: PRODUCT_META.xdriver.label,
            ...(kind === 'xdriver' || kind === 'xdriver-corp'
              ? { to: '/' }
              : { href: XDRIVER_CORP_ORIGIN }),
            icon: HardDrive,
            current: onXDriver,
          } satisfies LauncherTile,
        ]
      : []),
  ]

  const catalog: LauncherTile[] = [
    {
      id: 'marketplace',
      label: PRODUCT_META.marketplace.label,
      ...(variant === 'admin'
        ? { to: '/admin/marketplace' }
        : kind === 'marketplace'
          ? { to: '/' }
          : { href: MARKETPLACE_ORIGIN }),
      icon: Store,
      current: onMarketplace,
    },
    ...(showAdmin
      ? [
          {
            id: 'admin',
            label: 'Admin',
            ...panel('/admin'),
            icon: LayoutDashboard,
            current: variant === 'admin' && !onMarketplace && !onXDriver,
          } satisfies LauncherTile,
        ]
      : []),
  ]

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" title="Apps" className="icon-well">
          <LayoutGrid className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-[17.5rem] p-3">
        <DropdownMenuLabel className="px-1 pb-2 font-display text-[13px] font-semibold tracking-tight">
          Seus apps
        </DropdownMenuLabel>
        <TileGrid tiles={installed} />
        <DropdownMenuSeparator className="my-3" />
        <DropdownMenuLabel className="px-1 pb-2 font-display text-[13px] font-semibold tracking-tight">
          Catálogo
        </DropdownMenuLabel>
        <TileGrid tiles={catalog} />
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function TileGrid({ tiles }: { tiles: LauncherTile[] }) {
  const className =
    'flex flex-col items-center gap-1.5 rounded-[14px] px-1 py-2 outline-none hover:bg-white/8 focus-visible:bg-white/8'
  return (
    <div className="grid grid-cols-3 gap-1">
      {tiles.map((tile) => {
        const inner = (
          <>
            <span
              className={cn(
                'icon-well-lg flex size-12 items-center justify-center rounded-[16px] text-foreground',
                tile.current && 'ring-1 ring-safe',
              )}
            >
              <tile.icon className="size-5" strokeWidth={1.75} />
            </span>
            <span className="font-display max-w-full truncate text-center text-[11px] font-medium">{tile.label}</span>
          </>
        )
        if (tile.href) {
          const external = tile.href.startsWith('http')
          return (
            <a
              key={tile.id}
              href={tile.href}
              className={className}
              {...(external ? { target: '_blank', rel: 'noreferrer' } : {})}
            >
              {inner}
            </a>
          )
        }
        return (
          <Link key={tile.id} to={tile.to ?? '/'} className={className}>
            {inner}
          </Link>
        )
      })}
    </div>
  )
}
