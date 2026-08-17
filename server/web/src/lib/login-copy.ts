import type { ProductId } from '@xvpn/ui/react/products'

export type LoginVariant = 'user' | 'admin' | 'store' | 'sso'

/** Landing pública — no xauth, `/` é o próprio formulário. */
export const LOGIN_LANDING_HREF = 'https://www.ihuull.com'

export function loginHomeLink(variant: LoginVariant): {
  href: string
  external: boolean
  label: string
} {
  if (variant === 'store') {
    return { href: 'https://xvpn.ihuull.com', external: true, label: 'Voltar ao painel XVPN' }
  }
  if (variant === 'sso') {
    return { href: LOGIN_LANDING_HREF, external: true, label: 'Voltar à página inicial' }
  }
  return { href: '/', external: false, label: 'Voltar à página inicial' }
}

export function loginCopy(variant: LoginVariant): {
  product: ProductId
  title: string
  subtitle: string
  brandTitle: string
  brandBody: string
  kicker: string
} {
  switch (variant) {
    case 'admin':
      return {
        product: 'xadmin',
        title: 'Entrar na administração',
        subtitle: 'XADMIN Console — só operadores, só na VPN.',
        brandTitle: 'Controle da rede',
        brandBody: 'Peers, convites e o que só quem opera a plataforma precisa ver.',
        kicker: 'acesso seguro · console',
      }
    case 'store':
      return {
        product: 'marketplace',
        title: 'Entrar no Marketplace',
        subtitle: 'Catálogo e XDRIVER com a mesma conta ihuull.',
        brandTitle: 'Apps da organização',
        brandBody: 'Instale o que a rede libera — sem loja pública.',
        kicker: 'Marketplace · XDRIVER',
      }
    case 'sso':
      return {
        product: 'ihuull',
        title: 'Entrar na ihuull',
        subtitle: 'Uma conta para XVPN, XGROUP, XCHAT e o restante.',
        brandTitle: 'Onde a rede se encontra',
        brandBody: 'Login único com cookie .ihuull.com. Sem senha em cada app.',
        kicker: 'login único · cookie .ihuull.com',
      }
    default:
      return {
        product: 'xvpn',
        title: 'Entrar no meu espaço',
        subtitle: 'Dispositivos, apps e o portal da VPN.',
        brandTitle: 'Sua rede privada',
        brandBody: 'Exit node próprio, sem provedor terceiro. Acesso por convite.',
        kicker: 'dispositivos · marketplace',
      }
  }
}
