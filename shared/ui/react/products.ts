/** Catálogo visual de produto — PLAN.md §6.13. Sem hosts aqui.
 *
 * Slug de código (id, YAML, JWE aud) é sempre minúsculo.
 * Lockup de UI: LABEL em caixa do produto + kicker (hud-label vira MAIÚSCULO).
 * Vitrine / marketplace.yaml `name` = productDisplayName(id).
 */

export const PRODUCT_IDS = ['ihuull', 'xvpn', 'xchat', 'marketplace', 'xgroup', 'xdriver', 'xadmin', 'xgit'] as const

export type ProductId = (typeof PRODUCT_IDS)[number]

export const PRODUCT_META: Record<ProductId, { label: string; kicker: string }> = {
  ihuull: { label: 'ihuull', kicker: 'plataforma' },
  xvpn: { label: 'XVPN', kicker: 'Client' },
  xchat: { label: 'XCHAT', kicker: 'Client' },
  marketplace: { label: 'Marketplace', kicker: 'Store' },
  xgroup: { label: 'XGROUP', kicker: 'Social' },
  xdriver: { label: 'XDRIVER', kicker: 'Drive' },
  xadmin: { label: 'XADMIN', kicker: 'Console' },
  xgit: { label: 'XGIT', kicker: 'Forge' },
}

/** Nome de vitrine: "XCHAT Client", "Marketplace Store". */
export function productDisplayName(id: ProductId): string {
  const { label, kicker } = PRODUCT_META[id]
  if (id === 'ihuull') return label
  return `${label} ${kicker}`
}
