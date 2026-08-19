import { ProductHeader } from '@xvpn/ui/react/product-header'
import { headerProduct } from '@/lib/product-host'
import { useAuth } from '@/lib/auth-context'
import { AccountMenu, ProductSwitcher } from '@/components/layout/account-menu'
import { AppSettingsButton } from '@/components/layout/app-settings-button'

/** Chrome de sistema — só app (ícone + nome) e ações da conta. */
export function PanelHeader({ variant }: { variant: 'user' | 'admin' | 'social' }) {
  const { user } = useAuth()
  const product = headerProduct()
  const href =
    product === 'xchat' ? '/social/messages' : variant === 'social' ? '/social' : variant === 'admin' ? '/admin' : '/'

  return (
    <ProductHeader
      product={product}
      href={href}
      trailing={
        user ? (
          <>
            <AppSettingsButton kind={variant} />
            <ProductSwitcher variant={variant} />
            <AccountMenu variant={variant} />
          </>
        ) : null
      }
    />
  )
}
