/** Hosts de produto ihuull — PLAN.md §5.1. */

export const PANEL_ORIGIN = 'https://xvpn.ihuull.com'
export const MARKETPLACE_ORIGIN = 'https://marketplace.ihuull.com'
export const XDRIVER_ORIGIN = 'https://xdriver.ihuull.com'
export const XDRIVER_CORP_ORIGIN = 'https://xdriver.corp.ihuull.com'

export type ProductKind = 'marketplace' | 'xdriver' | 'core'

export function productKind(hostname = window.location.hostname): ProductKind {
  const host = hostname.toLowerCase()
  if (host === 'marketplace.ihuull.com' || host === 'marketplace.localhost') return 'marketplace'
  if (host === 'xdriver.ihuull.com' || host === 'xdriver.localhost') return 'xdriver'
  return 'core'
}

export function isStoreHost(hostname = window.location.hostname): boolean {
  const kind = productKind(hostname)
  return kind === 'marketplace' || kind === 'xdriver'
}

export function storeLoginPath(): string {
  return '/login'
}
