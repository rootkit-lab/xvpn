import { describe, expect, it } from 'vitest'
import { productIdFromCatalogSlug } from './marketplace-product'

describe('productIdFromCatalogSlug', () => {
  it('mapeia slugs do catálogo para o produto da fita', () => {
    expect(productIdFromCatalogSlug('xchat')).toBe('xchat')
    expect(productIdFromCatalogSlug('xvpn-client')).toBe('xvpn')
    expect(productIdFromCatalogSlug('XVPN')).toBe('xvpn')
    expect(productIdFromCatalogSlug('outro')).toBeNull()
  })
})
