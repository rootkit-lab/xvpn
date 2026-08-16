/** Catálogo visual de produto — PLAN.md §6.13. Sem hosts aqui. */

export const PRODUCT_IDS = ['ihuull', 'xvpn', 'marketplace', 'xgroup', 'xdriver'] as const

export type ProductId = (typeof PRODUCT_IDS)[number]

export const PRODUCT_META: Record<
  ProductId,
  { label: string; kicker: string }
> = {
  ihuull: { label: 'ihuull', kicker: 'plataforma' },
  xvpn: { label: 'xvpn', kicker: 'vpn' },
  marketplace: { label: 'marketplace', kicker: 'store' },
  xgroup: { label: 'xgroup', kicker: 'social' },
  xdriver: { label: 'xdriver', kicker: 'arquivos' },
}
