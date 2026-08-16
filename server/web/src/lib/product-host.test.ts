import { describe, expect, it } from 'vitest'
import { isStoreHost, productKind } from './product-host'

describe('productKind', () => {
  it('reconhece a loja, o portal público e o Drive da intranet', () => {
    expect(productKind('marketplace.ihuull.com')).toBe('marketplace')
    expect(productKind('xdriver.ihuull.com')).toBe('xdriver')
    expect(productKind('xdriver.corp.ihuull.com')).toBe('xdriver-corp')
    expect(productKind('xvpn.ihuull.com')).toBe('core')
  })

  it('trata o Drive corp como host de produto (login /login)', () => {
    expect(isStoreHost('xdriver.corp.ihuull.com')).toBe(true)
    expect(isStoreHost('xdriver.ihuull.com')).toBe(true)
    expect(isStoreHost('marketplace.ihuull.com')).toBe(true)
    expect(isStoreHost('xvpn.ihuull.com')).toBe(false)
  })
})
