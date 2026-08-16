import { describe, expect, it } from 'vitest'
import { headerProduct, isStoreHost, productKind, safeReturnURL, ssoLoginURL } from './product-host'

describe('productKind', () => {
  it('reconhece a loja, o portal XVPN, o Drive, o marketing xgroup e o xauth', () => {
    expect(productKind('marketplace.ihuull.com')).toBe('marketplace')
    expect(productKind('xdriver.ihuull.com')).toBe('xdriver')
    expect(productKind('xdriver.corp.ihuull.com')).toBe('xdriver-corp')
    expect(productKind('xvpn.ihuull.com')).toBe('xvpn')
    expect(productKind('xvpn.localhost')).toBe('xvpn')
    expect(productKind('localhost')).toBe('xvpn')
    expect(productKind('xgroup.ihuull.com')).toBe('xgroup')
    expect(productKind('xgroup.localhost')).toBe('xgroup')
    expect(productKind('xauth.ihuull.com')).toBe('xauth')
    expect(productKind('xauth.localhost')).toBe('xauth')
    expect(productKind('ihuull.com')).toBe('core')
    expect(productKind('xchat.ihuull.com')).toBe('core')
  })

  it('trata o Drive corp como host de produto (login /login)', () => {
    expect(isStoreHost('xdriver.corp.ihuull.com')).toBe(true)
    expect(isStoreHost('xdriver.ihuull.com')).toBe(false)
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
    expect(headerProduct('xvpn.ihuull.com', '/social/messages')).toBe('xchat')
    expect(headerProduct('xvpn.ihuull.com', '/xgroup/groups')).toBe('xgroup')
    expect(headerProduct('xvpn.ihuull.com', '/xgroup/messages')).toBe('xchat')
    expect(headerProduct('ihuull.com', '/')).toBe('ihuull')
    expect(headerProduct('www.ihuull.com', '/')).toBe('ihuull')
    expect(headerProduct('xauth.ihuull.com', '/login')).toBe('ihuull')
  })
})

describe('safeReturnURL', () => {
  it('aceita só hosts ihuull conhecidos', () => {
    expect(safeReturnURL('https://marketplace.ihuull.com/')).toBe('https://marketplace.ihuull.com/')
    expect(safeReturnURL('https://xvpn.ihuull.com/admin')).toContain('xvpn.ihuull.com')
    expect(safeReturnURL('https://evil.example/phish')).toBeNull()
    expect(safeReturnURL('javascript:alert(1)')).toBeNull()
  })
})

describe('ssoLoginURL', () => {
  it('aponta para xauth com return seguro', () => {
    const url = ssoLoginURL('https://marketplace.ihuull.com/')
    expect(url).toContain('xauth')
    expect(url).toContain('return=')
    expect(ssoLoginURL('https://evil.example/')).not.toContain('evil.example')
  })
})
