import { describe, expect, it } from 'vitest'
import {
  headerProduct,
  isAuthPath,
  isLoggedOutParam,
  isProductAppHost,
  isStoreHost,
  productKind,
  safeReturnURL,
  ssoContinueURL,
  ssoLoginURL,
  ssoLogoutURL,
} from './product-host'

describe('productKind', () => {
  it('reconhece cada host de produto — corp não cai no painel', () => {
    expect(productKind('marketplace.ihuull.com')).toBe('marketplace')
    expect(productKind('xdriver.ihuull.com')).toBe('xdriver')
    expect(productKind('xdriver.corp.ihuull.com')).toBe('xdriver-corp')
    expect(productKind('xvpn.ihuull.com')).toBe('xvpn')
    expect(productKind('xvpn.localhost')).toBe('xvpn')
    expect(productKind('localhost')).toBe('xvpn')
    expect(productKind('xgroup.ihuull.com')).toBe('xgroup')
    expect(productKind('xgroup.localhost')).toBe('xgroup')
    expect(productKind('xgroup.corp.ihuull.com')).toBe('xgroup-corp')
    expect(productKind('xchat.ihuull.com')).toBe('xchat')
    expect(productKind('xchat.corp.ihuull.com')).toBe('xchat-corp')
    expect(productKind('corp.ihuull.com')).toBe('corp')
    expect(productKind('xauth.ihuull.com')).toBe('xauth')
    expect(productKind('xauth.localhost')).toBe('xauth')
    expect(productKind('ihuull.com')).toBe('core')
  })

  it('trata apps de produto com login /login (não /admin)', () => {
    expect(isStoreHost('xdriver.corp.ihuull.com')).toBe(true)
    expect(isStoreHost('xdriver.ihuull.com')).toBe(false)
    expect(isStoreHost('marketplace.ihuull.com')).toBe(true)
    expect(isStoreHost('xvpn.ihuull.com')).toBe(false)
    expect(isStoreHost('xgroup.ihuull.com')).toBe(false)
    expect(isProductAppHost('xchat.corp.ihuull.com')).toBe(true)
    expect(isProductAppHost('xgroup.corp.ihuull.com')).toBe(true)
    expect(isProductAppHost('corp.ihuull.com')).toBe(true)
    expect(isProductAppHost('xvpn.ihuull.com')).toBe(false)
    expect(isProductAppHost('xchat.ihuull.com')).toBe(false)
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
    expect(headerProduct('xchat.corp.ihuull.com', '/admin')).toBe('xchat')
    expect(headerProduct('xchat.ihuull.com', '/')).toBe('xchat')
    expect(headerProduct('xgroup.corp.ihuull.com', '/social')).toBe('xgroup')
    expect(headerProduct('corp.ihuull.com', '/')).toBe('xvpn')
  })
})

describe('safeReturnURL', () => {
  it('aceita só hosts ihuull conhecidos', () => {
    expect(safeReturnURL('https://marketplace.ihuull.com/')).toBe('https://marketplace.ihuull.com/')
    expect(safeReturnURL('https://xvpn.ihuull.com/admin')).toContain('xvpn.ihuull.com')
    expect(safeReturnURL('https://xchat.corp.ihuull.com/social/messages')).toContain('xchat.corp.ihuull.com')
    expect(safeReturnURL('https://evil.example/phish')).toBeNull()
    expect(safeReturnURL('javascript:alert(1)')).toBeNull()
  })

  it('reescreve o alias legado vpn.ihuull.com e tira /admin de host que não é o painel', () => {
    expect(safeReturnURL('https://vpn.ihuull.com/social')).toBe('https://xvpn.ihuull.com/social')
    expect(safeReturnURL('https://xchat.corp.ihuull.com/admin')).toBe('https://xvpn.ihuull.com/admin')
    expect(safeReturnURL('https://xgroup.ihuull.com/admin/users')).toBe('https://xvpn.ihuull.com/admin')
    expect(safeReturnURL('https://marketplace.ihuull.com/admin')).toBe('https://xvpn.ihuull.com/admin')
  })
})

describe('ssoLoginURL', () => {
  it('aponta para xauth com return seguro', () => {
    const url = ssoLoginURL('https://marketplace.ihuull.com/')
    expect(url).toContain('xauth')
    expect(url).toContain('return=')
    expect(ssoLoginURL('https://evil.example/')).not.toContain('evil.example')
  })

  it('não devolve tela de login como return (quebra o loop xauth)', () => {
    const url = ssoLoginURL('https://xvpn.ihuull.com/my/login')
    expect(url).toContain('return=')
    expect(decodeURIComponent(new URL(url).searchParams.get('return') ?? '')).toBe('https://xvpn.ihuull.com/')
  })
})

describe('ssoContinueURL', () => {
  it('ignora return de login e manda member ao portal', () => {
    expect(ssoContinueURL('member', 'https://xvpn.ihuull.com/my/login')).toBe('https://xvpn.ihuull.com/')
    expect(ssoContinueURL('admin', 'https://xvpn.ihuull.com/admin')).toBe('https://xvpn.ihuull.com/admin')
  })

  it('reconhece paths de autenticação', () => {
    expect(isAuthPath('/login')).toBe(true)
    expect(isAuthPath('/my/login')).toBe(true)
    expect(isAuthPath('/admin/login')).toBe(true)
    expect(isAuthPath('/admin')).toBe(false)
  })
})

describe('ssoLogoutURL', () => {
  it('marca logged_out para o xauth não auto-continuar', () => {
    const url = ssoLogoutURL('https://marketplace.ihuull.com/')
    expect(url).toContain('xauth')
    expect(url).toContain('logged_out=1')
    expect(decodeURIComponent(new URL(url).searchParams.get('return') ?? '')).toBe('https://marketplace.ihuull.com/')
    expect(isLoggedOutParam('?logged_out=1&return=https://marketplace.ihuull.com/')).toBe(true)
    expect(isLoggedOutParam('?return=https://marketplace.ihuull.com/')).toBe(false)
  })
})
