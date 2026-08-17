import { describe, expect, it, vi } from 'vitest'
import {
  headerProduct,
  isAuthPath,
  isLoggedOutParam,
  isProductAppHost,
  isStoreHost,
  productKind,
  safeReturnURL,
  ssoContinueURL,
  ssoHandoff,
  ssoHandoffContinueURL,
  ssoLoginURL,
  loginAudience,
  ssoLogoutURL,
  codespaceOpenHref,
  codespaceRuntimeURL,
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
    expect(productKind('xadmin.corp.ihuull.com')).toBe('xadmin-corp')
    expect(productKind('xadmin.corp.localhost')).toBe('xadmin-corp')
    expect(productKind('xgit.corp.ihuull.com')).toBe('xgit-corp')
    expect(productKind('xgit.corp.localhost')).toBe('xgit-corp')
    expect(productKind('xcodespaces.corp.ihuull.com')).toBe('xcodespaces-corp')
    expect(productKind('cs-aabbccddeeff.corp.ihuull.com')).toBe('xcodespaces-corp')
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
    expect(isProductAppHost('xgroup.ihuull.com')).toBe(true)
    expect(isProductAppHost('xgroup.corp.ihuull.com')).toBe(true)
    expect(isProductAppHost('corp.ihuull.com')).toBe(true)
    expect(isProductAppHost('xgit.corp.ihuull.com')).toBe(true)
    expect(isProductAppHost('xcodespaces.corp.ihuull.com')).toBe(true)
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
    expect(headerProduct('xadmin.corp.ihuull.com', '/admin')).toBe('xadmin')
    expect(headerProduct('xgit.corp.ihuull.com', '/')).toBe('xgit')
    expect(headerProduct('xcodespaces.corp.ihuull.com', '/')).toBe('xcodespaces')
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

describe('loginAudience', () => {
  it('emite aud do host de produto', () => {
    expect(loginAudience('xgit.corp.ihuull.com')).toBe('xgit')
    expect(loginAudience('xcodespaces.corp.ihuull.com')).toBe('xcodespaces')
    expect(loginAudience('xadmin.corp.ihuull.com')).toBe('xadmin')
    expect(loginAudience('xvpn.ihuull.com')).toBe('xvpn')
  })
})

describe('safeReturnURL', () => {
  it('aceita só hosts ihuull conhecidos', () => {
    expect(safeReturnURL('https://marketplace.ihuull.com/')).toBe('https://marketplace.ihuull.com/')
    expect(safeReturnURL('https://xvpn.ihuull.com/admin')).toBe('https://xadmin.corp.ihuull.com/admin')
    expect(safeReturnURL('https://xadmin.corp.ihuull.com/admin')).toBe('https://xadmin.corp.ihuull.com/admin')
    expect(safeReturnURL('https://xchat.corp.ihuull.com/social/messages')).toContain('xchat.corp.ihuull.com')
    expect(safeReturnURL('https://evil.example/phish')).toBeNull()
    expect(safeReturnURL('javascript:alert(1)')).toBeNull()
    expect(safeReturnURL('http://xvpn.ihuull.com/admin')).toBeNull()
    expect(safeReturnURL('http://localhost/admin')).toBe('https://xadmin.corp.ihuull.com/admin')
  })

  it('reescreve o alias legado vpn.ihuull.com e tira /admin de host que não é o console', () => {
    expect(safeReturnURL('https://vpn.ihuull.com/social')).toBe('https://xvpn.ihuull.com/social')
    expect(safeReturnURL('https://xchat.corp.ihuull.com/admin')).toBe('https://xadmin.corp.ihuull.com/admin')
    expect(safeReturnURL('https://xgroup.ihuull.com/admin/users')).toBe('https://xadmin.corp.ihuull.com/admin')
    expect(safeReturnURL('https://marketplace.ihuull.com/admin')).toBe('https://xadmin.corp.ihuull.com/admin')
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
    expect(ssoContinueURL('admin', 'https://xvpn.ihuull.com/admin')).toBe('https://xadmin.corp.ihuull.com/admin')
    expect(ssoContinueURL('admin')).toBe('https://xadmin.corp.ihuull.com/admin')
  })

  it('reconhece paths de autenticação', () => {
    expect(isAuthPath('/login')).toBe(true)
    expect(isAuthPath('/my/login')).toBe(true)
    expect(isAuthPath('/admin/login')).toBe(true)
    expect(isAuthPath('/admin')).toBe(false)
  })
})

describe('ssoHandoffContinueURL', () => {
  it('manda o return ao xauth sem expor o JWE', () => {
    const url = ssoHandoffContinueURL('member', 'https://xvpn.ihuull.com/')
    expect(url).toContain('xauth')
    expect(url).toContain('/api/auth/handoff-continue')
    expect(decodeURIComponent(new URL(url).searchParams.get('return') ?? '')).toBe('https://xvpn.ihuull.com/')
  })
})

describe('ssoHandoff', () => {
  it('posta o JWE no host de destino em vez de só redirecionar', () => {
    const submit = vi.spyOn(HTMLFormElement.prototype, 'submit').mockImplementation(() => {})
    ssoHandoff('admin', 'https://xvpn.ihuull.com/admin', 'jwe-token')
    const form = document.querySelector('form[action="https://xadmin.corp.ihuull.com/api/auth/session"]')
    expect(form).toBeTruthy()
    const fields = Object.fromEntries(new FormData(form as HTMLFormElement))
    expect(fields.token).toBe('jwe-token')
    expect(fields.return).toBe('https://xadmin.corp.ihuull.com/admin')
    expect(submit).toHaveBeenCalledOnce()
    submit.mockRestore()
    form?.remove()
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

describe('codespaceOpenHref', () => {
  it('remote vai para cs-* mesmo sem runtime_url', () => {
    expect(codespaceRuntimeURL('aabbccddeeff')).toBe('https://cs-aabbccddeeff.corp.ihuull.com')
    expect(codespaceOpenHref({ id: 'aabbccddeeff', kind: 'remote' })).toBe(
      'https://cs-aabbccddeeff.corp.ihuull.com',
    )
    expect(codespaceOpenHref({ id: 'aabbccddeeff', kind: 'quick' })).toBe(
      'https://xcodespaces.corp.ihuull.com/aabbccddeeff',
    )
  })
})
