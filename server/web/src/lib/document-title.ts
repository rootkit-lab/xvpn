import { PRODUCT_META } from '@xvpn/ui/react/products'
import { headerProduct } from '@/lib/product-host'
import { pageMetaForPath } from '@/lib/page-meta'

export function titleBrand(hostname: string, pathname: string): string {
  return PRODUCT_META[headerProduct(hostname, pathname)].label
}

function lastPathSegment(pathname: string, prefix: RegExp): string {
  const m = pathname.match(prefix)
  if (!m?.[1]) return ''
  try {
    return decodeURIComponent(m[1])
  } catch {
    return m[1]
  }
}

/** Rótulo à esquerda do · — conversa, pasta, perfil ou título da rota. */
export function titlePage(opts: {
  hostname: string
  pathname: string
  pageOverride?: string
  chatTitle?: string
}): string {
  const chat = opts.chatTitle?.trim()
  if (chat) return chat
  const override = opts.pageOverride?.trim()
  if (override) return override

  const path = opts.pathname.replace(/\/+$/, '') || '/'
  if (path === '/login' || path === '/my/login' || path === '/admin/login') {
    return 'Entrar'
  }
  const profile = lastPathSegment(path, /\/(?:social|xgroup)\/u\/([^/]+)/)
  if (profile) return profile
  const app = lastPathSegment(path, /^\/app\/([^/]+)/)
  if (app) return app
  if (path === '/') return ''

  const page = pageMetaForPath(opts.pathname).title.trim()
  const brand = titleBrand(opts.hostname, opts.pathname)
  if (!page || page === brand || page === 'Painel') return ''
  return page
}

export function documentTitle(opts: {
  hostname: string
  pathname: string
  pageOverride?: string
  chatTitle?: string
  unread?: number
}): string {
  const brand = titleBrand(opts.hostname, opts.pathname)
  const page = titlePage(opts)
  let title = page && page !== brand ? `${page} · ${brand}` : brand
  const unread = opts.unread ?? 0
  if (unread > 0) {
    const n = unread > 99 ? '99+' : String(unread)
    title = `(${n}) ${title}`
  }
  return title
}

export function focusedChatKey(opts: {
  pathname: string
  activeKey: string | null
  popouts: { key: string; minimized: boolean }[]
}): string | null {
  const expanded = opts.popouts.filter((p) => !p.minimized)
  const focused = expanded.find((p) => p.key === opts.activeKey) ?? expanded.at(-1)
  if (focused) return focused.key
  if (/\/(?:social|xgroup|xchat)\/messages/.test(opts.pathname)) {
    return opts.activeKey
  }
  return null
}
