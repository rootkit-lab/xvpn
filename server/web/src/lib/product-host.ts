import type { ProductId } from '@xvpn/ui/react/products'

/** Hosts de produto ihuull — PLAN.md §5.1. */

export const PANEL_ORIGIN = 'https://xvpn.ihuull.com'
export const MARKETPLACE_ORIGIN = 'https://marketplace.ihuull.com'
export const XDRIVER_ORIGIN = 'https://xdriver.ihuull.com'
export const XDRIVER_CORP_ORIGIN = 'https://xdriver.corp.ihuull.com'
export const XGROUP_ORIGIN = 'https://xgroup.ihuull.com'
export const XGROUP_CORP_ORIGIN = 'https://xgroup.corp.ihuull.com'
export const XCHAT_CORP_ORIGIN = 'https://xchat.corp.ihuull.com'
export const XAUTH_ORIGIN = 'https://xauth.ihuull.com'

export type ProductKind =
  | 'marketplace'
  | 'xdriver'
  | 'xdriver-corp'
  | 'xgroup'
  | 'xvpn'
  | 'xauth'
  | 'core'

const SAFE_RETURN_HOSTS = new Set([
  'xauth.ihuull.com',
  'xvpn.ihuull.com',
  'marketplace.ihuull.com',
  'xdriver.ihuull.com',
  'xdriver.corp.ihuull.com',
  'xgroup.ihuull.com',
  'xgroup.corp.ihuull.com',
  'xchat.ihuull.com',
  'www.ihuull.com',
  'ihuull.com',
  'xauth.localhost',
  'xvpn.localhost',
  'marketplace.localhost',
  'xdriver.localhost',
  'xdriver.corp.localhost',
  'xgroup.localhost',
  'xgroup.corp.localhost',
  'localhost',
  '127.0.0.1',
])

export function productKind(hostname = window.location.hostname): ProductKind {
  const host = hostname.toLowerCase()
  if (host === 'xauth.ihuull.com' || host === 'xauth.localhost') return 'xauth'
  if (host === 'marketplace.ihuull.com' || host === 'marketplace.localhost') return 'marketplace'
  if (host === 'xdriver.corp.ihuull.com' || host === 'xdriver.corp.localhost') return 'xdriver-corp'
  if (host === 'xdriver.ihuull.com' || host === 'xdriver.localhost') return 'xdriver'
  if (host === 'xgroup.ihuull.com' || host === 'xgroup.localhost') return 'xgroup'
  if (host === 'xvpn.ihuull.com' || host === 'xvpn.localhost' || host === 'localhost' || host === '127.0.0.1') {
    return 'xvpn'
  }
  return 'core'
}

/** Produto do header — host + path. Não muda o roteamento de `productKind`. */
export function headerProduct(
  hostname = window.location.hostname,
  pathname = typeof window === 'undefined' ? '/' : window.location.pathname,
): ProductId {
  const kind = productKind(hostname)
  if (kind === 'xauth') return 'ihuull'
  if (kind === 'marketplace') return 'marketplace'
  if (kind === 'xdriver' || kind === 'xdriver-corp') return 'xdriver'
  const host = hostname.toLowerCase()
  if (
    host === 'ihuull.com' ||
    host === 'www.ihuull.com' ||
    host === 'ihuu.com' ||
    host === 'www.ihuu.com' ||
    host === 'ihuull.localhost'
  ) {
    return 'ihuull'
  }
  if (pathname.includes('/messages') || pathname.startsWith('/xchat')) return 'xchat'
  if (pathname.startsWith('/social') || pathname.startsWith('/xgroup')) return 'xgroup'
  return 'xvpn'
}

export function isStoreHost(hostname = window.location.hostname): boolean {
  const kind = productKind(hostname)
  return kind === 'marketplace' || kind === 'xdriver-corp'
}

export function storeLoginPath(): string {
  return '/login'
}

export function xauthOrigin(hostname = typeof window === 'undefined' ? '' : window.location.hostname): string {
  const host = hostname.toLowerCase()
  if (host.endsWith('.localhost') || host === 'localhost' || host === '127.0.0.1') {
    const proto = typeof window === 'undefined' ? 'http:' : window.location.protocol
    const port = typeof window === 'undefined' || !window.location.port ? '' : `:${window.location.port}`
    return `${proto}//xauth.localhost${port}`
  }
  return XAUTH_ORIGIN
}

/** Return URL só para hosts ihuull conhecidos — bloqueia open redirect. */
export function safeReturnURL(raw: string | null | undefined): string | null {
  if (!raw) return null
  try {
    const u = new URL(raw, 'https://xauth.ihuull.com')
    if (u.protocol !== 'https:' && u.protocol !== 'http:') return null
    if (!SAFE_RETURN_HOSTS.has(u.hostname.toLowerCase())) return null
    return u.toString()
  } catch {
    return null
  }
}

/** Login pages não podem ser return do SSO — senão xauth ↔ /my/login entra em loop. */
export function isAuthPath(pathname: string): boolean {
  const p = pathname.replace(/\/+$/, '') || '/'
  return p === '/login' || p.endsWith('/login')
}

/** Destino depois do cookie no xauth: return seguro, nunca outra tela de login. */
export function ssoContinueURL(role: string, returnTo?: string | null): string {
  const safe = safeReturnURL(returnTo)
  if (safe) {
    try {
      const u = new URL(safe)
      if (!isAuthPath(u.pathname)) return safe
    } catch {
      // cai no default
    }
  }
  return `${PANEL_ORIGIN}${role === 'member' ? '/' : '/admin'}`
}

export function ssoLoginURL(returnTo?: string): string {
  const dest = new URL('/login', xauthOrigin())
  const fallback =
    returnTo ?? (typeof window === 'undefined' ? PANEL_ORIGIN : window.location.href)
  const safe = safeReturnURL(fallback)
  if (!safe) return dest.toString()
  const u = new URL(safe)
  dest.searchParams.set('return', isAuthPath(u.pathname) ? `${u.origin}/` : safe)
  return dest.toString()
}
