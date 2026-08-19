import { describe, expect, it } from 'vitest'
import { loginCopy, loginHomeLink, LOGIN_LANDING_HREF } from './login-copy'

describe('loginCopy', () => {
  it('SSO fala ihuull, admin fala painel', () => {
    expect(loginCopy('sso').product).toBe('ihuull')
    expect(loginCopy('sso').title).toMatch(/ihuull/i)
    expect(loginCopy('admin').product).toBe('xadmin')
    expect(loginCopy('admin').title).toMatch(/administração/i)
    expect(loginCopy('store').product).toBe('marketplace')
    expect(loginCopy('user').product).toBe('xvpn')
  })

  it('SSO não aponta “voltar” para o próprio formulário', () => {
    expect(loginHomeLink('sso')).toEqual({
      href: LOGIN_LANDING_HREF,
      external: true,
      label: 'Voltar à página inicial',
    })
    expect(loginHomeLink('user').href).toBe('/')
    expect(loginHomeLink('user').external).toBe(false)
  })
})
