import { XGROUP_ORIGIN, productKind } from '@/lib/product-host'

/** Unix username — mesmo padrão de `provision.ValidUsername`. */
const USERNAME_RE = /^[a-z][a-z0-9_-]{2,31}$/

/** Slugs que não podem ser `/:username` (rotas e hosts de produto). */
const RESERVED = new Set([
  'admin',
  'api',
  'app',
  'assets',
  'explore',
  'groups',
  'health',
  'login',
  'marketplace',
  'messages',
  'my',
  'settings',
  'social',
  'static',
  'u',
  'xauth',
  'xchat',
  'xdriver',
  'xgroup',
  'xvpn',
])

export function isProfileUsername(name: string): boolean {
  return USERNAME_RE.test(name) && !RESERVED.has(name)
}

export function profilePath(username: string): string {
  return `/${encodeURIComponent(username)}`
}

/** URL canônica do perfil — `xgroup.ihuull.com/<user>`. */
export function profileHref(username: string): string {
  return `${XGROUP_ORIGIN}${profilePath(username)}`
}

export function profileLocation(username: string): { href: string; external: boolean } {
  const path = profilePath(username)
  if (productKind() === 'xgroup') return { href: path, external: false }
  return { href: `${XGROUP_ORIGIN}${path}`, external: true }
}

function decodeSlug(raw: string): string {
  try {
    return decodeURIComponent(raw)
  } catch {
    return ''
  }
}

export function profileUsernameFromPath(
  pathname: string,
  hostname = typeof window === 'undefined' ? '' : window.location.hostname,
): string {
  const path = pathname.replace(/\/+$/, '') || '/'
  const nested = path.match(/^\/(?:social|xgroup)\/u\/([^/]+)$/)
  if (nested?.[1]) {
    const name = decodeSlug(nested[1])
    return isProfileUsername(name) ? name : ''
  }
  const kind = hostname ? productKind(hostname) : ''
  if (kind !== 'xgroup' && kind !== 'xgroup-corp') return ''
  const bare = path.match(/^\/([^/]+)$/)
  if (!bare?.[1]) return ''
  const name = decodeSlug(bare[1])
  return isProfileUsername(name) ? name : ''
}

export function isSocialProfilePath(
  pathname: string,
  hostname = typeof window === 'undefined' ? '' : window.location.hostname,
): boolean {
  return profileUsernameFromPath(pathname, hostname) !== ''
}
