import type { ProductId } from '@xvpn/ui/react/products'

/** Slug do catálogo (`marketplace.yaml`) → produto visual da fita. */
export function productIdFromCatalogSlug(slug: string): ProductId | null {
  const s = slug.trim().toLowerCase()
  if (s === 'xvpn' || s === 'xvpn-client') return 'xvpn'
  if (s === 'xchat' || s === 'xvpn-chat') return 'xchat'
  if (s === 'xgroup' || s === 'xvpn-group') return 'xgroup'
  if (s === 'xdriver') return 'xdriver'
  if (s === 'marketplace') return 'marketplace'
  if (s === 'xgit') return 'xgit'
  return null
}
