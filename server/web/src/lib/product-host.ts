import type { ProductId } from '@xvpn/ui/react/products'

/** Hosts de produto ihuull — PLAN.md §5.1. */

export const PANEL_ORIGIN = 'https://xvpn.ihuull.com'
export const XADMIN_CORP_ORIGIN = 'https://xadmin.corp.ihuull.com'
export const MARKETPLACE_ORIGIN = 'https://marketplace.ihuull.com'
export const XDRIVER_ORIGIN = 'https://xdriver.ihuull.com'
export const XDRIVER_CORP_ORIGIN = 'https://xdriver.corp.ihuull.com'
export const XGROUP_ORIGIN = 'https://xgroup.ihuull.com'
export const XGROUP_CORP_ORIGIN = 'https://xgroup.corp.ihuull.com'
export const XCHAT_CORP_ORIGIN = 'https://xchat.corp.ihuull.com'
export const XCHAT_ORIGIN = 'https://xchat.ihuull.com'
export const CORP_ORIGIN = 'https://corp.ihuull.com'
export const XAUTH_ORIGIN = 'https://xauth.ihuull.com'
export const XGIT_CORP_ORIGIN = 'https://xgit.corp.ihuull.com'
export const XCODESPACES_CORP_ORIGIN = 'https://xcodespaces.corp.ihuull.com'

export type ProductKind =
  | 'marketplace'
  | 'xdriver'
  | 'xdriver-corp'
  | 'xgroup'
  | 'xgroup-corp'
  | 'xchat'
  | 'xchat-corp'
  | 'corp'
  | 'xvpn'
  | 'xadmin-corp'
  | 'xgit-corp'
  | 'xcodespaces-corp'
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
  'xchat.corp.ihuull.com',
  'corp.ihuull.com',
  'xadmin.corp.ihuull.com',
  'xgit.corp.ihuull.com',
  'xcodespaces.corp.ihuull.com',
  'www.ihuull.com',
  'ihuull.com',
  'xauth.localhost',
  'xvpn.localhost',
  'xadmin.corp.localhost',
  'xgit.corp.localhost',
  'xcodespaces.corp.localhost',
  'marketplace.localhost',
  'xdriver.localhost',
  'xdriver.corp.localhost',
  'xgroup.localhost',
  'xgroup.corp.localhost',
  'xchat.localhost',
  'xchat.corp.localhost',
  'corp.localhost',
  'localhost',
  '127.0.0.1',
])

export function productKind(hostname = window.location.hostname): ProductKind {
  const host = hostname.toLowerCase()
  if (host === 'xauth.ihuull.com' || host === 'xauth.localhost') return 'xauth'
  if (host === 'marketplace.ihuull.com' || host === 'marketplace.localhost') return 'marketplace'
  if (host === 'xdriver.corp.ihuull.com' || host === 'xdriver.corp.localhost') return 'xdriver-corp'
  if (host === 'xdriver.ihuull.com' || host === 'xdriver.localhost') return 'xdriver'
  if (host === 'xgroup.corp.ihuull.com' || host === 'xgroup.corp.localhost') return 'xgroup-corp'
  if (host === 'xgroup.ihuull.com' || host === 'xgroup.localhost') return 'xgroup'
  if (host === 'xchat.corp.ihuull.com' || host === 'xchat.corp.localhost') return 'xchat-corp'
  if (host === 'xchat.ihuull.com' || host === 'xchat.localhost') return 'xchat'
  if (host === 'corp.ihuull.com' || host === 'corp.localhost') return 'corp'
  if (host === 'xadmin.corp.ihuull.com' || host === 'xadmin.corp.localhost') return 'xadmin-corp'
  if (host === 'xgit.corp.ihuull.com' || host === 'xgit.corp.localhost') return 'xgit-corp'
  if (host === 'xcodespaces.corp.ihuull.com' || host === 'xcodespaces.corp.localhost') return 'xcodespaces-corp'
  if (/^cs-[a-f0-9]{12}\.corp\.(ihuull\.com|localhost)$/.test(host)) return 'xcodespaces-corp'
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
  if (kind === 'xchat' || kind === 'xchat-corp') return 'xchat'
  if (kind === 'xgroup' || kind === 'xgroup-corp') return 'xgroup'
  if (kind === 'xadmin-corp') return 'xadmin'
  if (kind === 'xgit-corp') return 'xgit'
  if (kind === 'xcodespaces-corp') return 'xcodespaces'
  if (kind === 'corp') return 'xvpn'
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

/** Hosts de produto com login em /login (não /my/login nem /admin). */
export function isProductAppHost(hostname = window.location.hostname): boolean {
  const kind = productKind(hostname)
  return (
    kind === 'marketplace' ||
    kind === 'xdriver-corp' ||
    kind === 'xchat-corp' ||
    kind === 'xgroup' ||
    kind === 'xgroup-corp' ||
    kind === 'xgit-corp' ||
    kind === 'xcodespaces-corp' ||
    kind === 'corp'
  )
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
    const host = u.hostname.toLowerCase()
    const local = host === 'localhost' || host === '127.0.0.1' || host.endsWith('.localhost')
    if (u.protocol !== 'https:' && !(local && u.protocol === 'http:')) return null
    // Alias legado — vpn.ihuull.com nunca foi produto (PLAN.md §5.1).
    if (host === 'vpn.ihuull.com' || host === 'vpn.localhost') {
      u.hostname = host.endsWith('.localhost') ? 'xvpn.localhost' : 'xvpn.ihuull.com'
    }
    const hostOk =
      SAFE_RETURN_HOSTS.has(u.hostname.toLowerCase()) ||
      /^cs-[a-f0-9]{12}\.corp\.(ihuull\.com|localhost)$/.test(u.hostname.toLowerCase())
    if (!hostOk) return null
    const kind = productKind(u.hostname)
    if (kind !== 'xadmin-corp' && u.pathname.startsWith('/admin')) {
      return `${XADMIN_CORP_ORIGIN}/admin`
    }
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
  return role === 'member' ? `${PANEL_ORIGIN}/` : `${XADMIN_CORP_ORIGIN}/admin`
}

/** aud do JWE no login deste host — PLAN.md §6.14. */
export function loginAudience(hostname = window.location.hostname): string {
  const kind = productKind(hostname)
  if (kind === 'xadmin-corp') return 'xadmin'
  if (kind === 'xgit-corp') return 'xgit'
  if (kind === 'xcodespaces-corp') return 'xcodespaces'
  if (kind === 'xchat' || kind === 'xchat-corp') return 'xchat'
  if (kind === 'xgroup' || kind === 'xgroup-corp') return 'xgroup'
  if (kind === 'xdriver' || kind === 'xdriver-corp') return 'xdriver'
  return 'xvpn'
}

/** Navegação top-level no xauth: o servidor POSTA o cookie, sem JSON. */
export function ssoHandoffContinueURL(role: string, returnTo?: string | null): string {
  const dest = new URL('/api/auth/handoff-continue', xauthOrigin())
  dest.searchParams.set('return', ssoContinueURL(role, returnTo))
  return dest.toString()
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

/** Leva o JWE ao host de destino via POST (cookie Domain=.ihuull.com lá). */
export function ssoHandoff(role: string, returnTo: string | null | undefined, token: string | null): void {
  const dest = ssoContinueURL(role, returnTo)
  if (!token || typeof document === 'undefined') {
    window.location.replace(dest)
    return
  }
  const destURL = new URL(dest)
  const local = destURL.hostname === 'localhost' || destURL.hostname === '127.0.0.1' || destURL.hostname.endsWith('.localhost')
  if (destURL.protocol !== 'https:' && !(local && destURL.protocol === 'http:')) {
    window.location.replace(dest)
    return
  }
  const form = document.createElement('form')
  form.method = 'POST'
  form.action = `${destURL.origin}/api/auth/session`
  form.style.display = 'none'
  const add = (name: string, value: string) => {
    const input = document.createElement('input')
    input.type = 'hidden'
    input.name = name
    input.value = value
    form.appendChild(input)
  }
  add('token', token)
  add('return', dest)
  document.body.appendChild(form)
  form.submit()
}

/** Depois do logout: xauth mostra o form, nunca auto-continua com cookie velho. */
export function ssoLogoutURL(returnTo?: string): string {
  const dest = new URL(ssoLoginURL(returnTo ?? (typeof window === 'undefined' ? PANEL_ORIGIN : `${window.location.origin}/`)))
  dest.searchParams.set('logged_out', '1')
  return dest.toString()
}

export function isLoggedOutParam(search: string): boolean {
  return new URLSearchParams(search).get('logged_out') === '1'
}
