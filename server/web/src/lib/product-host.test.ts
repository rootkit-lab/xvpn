import { describe, expect, it } from 'vitest'
import { headerProduct, isStoreHost, productKind } from './product-host'

describe('productKind', () => {
  it('reconhece a loja, o portal XVPN, o Drive e o marketing xgroup', () => {
    expect(productKind('marketplace.ihuull.com')).toBe('marketplace')
    expect(productKind('xdriver.ihuull.com')).toBe('xdriver')
    expect(productKind('xdriver.corp.ihuull.com')).toBe('xdriver-corp')
    expect(productKind('xvpn.ihuull.com')).toBe('xvpn')
    expect(productKind('xvpn.localhost')).toBe('xvpn')
    expect(productKind('localhost')).toBe('xvpn')
    expect(productKind('xgroup.ihuull.com')).toBe('xgroup')
    expect(productKind('xgroup.localhost')).toBe('xgroup')
    expect(productKind('ihuull.com')).toBe('core')
    expect(productKind('xchat.ihuull.com')).toBe('core')
  })

  it('trata o Drive corp como host de produto (login /login)', () => {
    expect(isStoreHost('xdriver.corp.ihuull.com')).toBe(true)
    expect(isStoreHost('xdriver.ihuull.com')).toBe(true)
    expect(isStoreHost('marketplace.ihuull.com')).toBe(true)
    expect(isStoreHost('xvpn.ihuull.com')).toBe(false)
    expect(isStoreHost('xgroup.ihuull.com')).toBe(false)
  })
})

describe('headerProduct', () => {
  it('mapeia host e path para a identidade do chrome', () => {
    expect(headerProduct('marketplace.ihuull.com', '/')).toBe('marketplace')
    expect(headerProduct('xdriver.ihuull.com', '/')).toBe('xdriver')
    expect(headerProduct('xdriver.corp.ihuull.com', '/')).toBe('xdriver')
    expect(headerProduct('xvpn.ihuull.com', '/')).toBe('xvpn')
    expect(headerProduct('xvpn.ihuull.com', '/my')).toBe('xvpn')
    expect(headerProduct('xvpn.ihuull.com', '/admin')).toBe('xvpn')
    expect(headerProduct('xvpn.ihuull.com', '/social')).toBe('xgroup')
    expect(headerProduct('xvpn.ihuull.com', '/xgroup/groups')).toBe('xgroup')
    expect(headerProduct('ihuull.com', '/')).toBe('ihuull')
    expect(headerProduct('www.ihuull.com', '/')).toBe('ihuull')
  })
})
