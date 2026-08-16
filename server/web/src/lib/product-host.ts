import type { ProductId } from '@xvpn/ui/react/products'

/** Hosts de produto ihuull — PLAN.md §5.1. */

export const PANEL_ORIGIN = 'https://xvpn.ihuull.com'
export const MARKETPLACE_ORIGIN = 'https://marketplace.ihuull.com'
export const XDRIVER_ORIGIN = 'https://xdriver.ihuull.com'
export const XDRIVER_CORP_ORIGIN = 'https://xdriver.corp.ihuull.com'

export type ProductKind = 'marketplace' | 'xdriver' | 'xdriver-corp' | 'core'

export function productKind(hostname = window.location.hostname): ProductKind {
  const host = hostname.toLowerCase()
  if (host === 'marketplace.ihuull.com' || host === 'marketplace.localhost') return 'marketplace'
  if (host === 'xdriver.corp.ihuull.com' || host === 'xdriver.corp.localhost') return 'xdriver-corp'
  if (host === 'xdriver.ihuull.com' || host === 'xdriver.localhost') return 'xdriver'
  return 'core'
}

/** Produto do header — host + path. Não muda o roteamento de `productKind`. */
export function headerProduct(
  hostname = window.location.hostname,
  pathname = typeof window === 'undefined' ? '/' : window.location.pathname,
): ProductId {
  const kind = productKind(hostname)
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
  if (pathname.startsWith('/social') || pathname.startsWith('/xgroup')) return 'xgroup'
  return 'xvpn'
}

export function isStoreHost(hostname = window.location.hostname): boolean {
  const kind = productKind(hostname)
  return kind === 'marketplace' || kind === 'xdriver' || kind === 'xdriver-corp'
}

export function storeLoginPath(): string {
  return '/login'
}
