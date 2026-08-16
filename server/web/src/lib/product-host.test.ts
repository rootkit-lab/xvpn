import { describe, expect, it } from 'vitest'
import { isStoreHost, productKind } from './product-host'

describe('productKind', () => {
  it('reconhece a loja e o portal', () => {
    expect(productKind('marketplace.ihuull.com')).toBe('marketplace')
    expect(productKind('xdriver.ihuull.com')).toBe('xdriver')
    expect(productKind('xvpn.ihuull.com')).toBe('core')
    expect(productKind('xdriver.corp.ihuull.com')).toBe('core')
  })

  it('não trata o FileBrowser corp como portal público', () => {
    expect(isStoreHost('xdriver.corp.ihuull.com')).toBe(false)
    expect(isStoreHost('marketplace.ihuull.com')).toBe(true)
  })
})
