import { describe, expect, it } from 'vitest'
import { documentTitle, focusedChatKey, titleBrand, titlePage } from './document-title'

describe('titleBrand', () => {
  it('usa o produto do host, não um Painel genérico', () => {
    expect(titleBrand('xdriver.corp.ihuull.com', '/')).toBe('XDRIVER')
    expect(titleBrand('xchat.corp.ihuull.com', '/social/messages')).toBe('XCHAT')
    expect(titleBrand('xgroup.corp.ihuull.com', '/social')).toBe('XGROUP')
    expect(titleBrand('marketplace.ihuull.com', '/')).toBe('Marketplace')
    expect(titleBrand('xauth.ihuull.com', '/login')).toBe('ihuull')
    expect(titleBrand('xvpn.ihuull.com', '/admin/users')).toBe('XVPN')
    expect(titleBrand('xadmin.corp.ihuull.com', '/admin/users')).toBe('XADMIN')
    expect(titleBrand('xgit.corp.ihuull.com', '/')).toBe('XGIT')
    expect(titleBrand('xcodespaces.corp.ihuull.com', '/')).toBe('XCODESPACES')
  })

  it('no painel, a rota social vira XGROUP/XCHAT', () => {
    expect(titleBrand('xvpn.ihuull.com', '/social')).toBe('XGROUP')
    expect(titleBrand('xvpn.ihuull.com', '/social/messages')).toBe('XCHAT')
  })
})

describe('documentTitle', () => {
  it('não repete XVPN — Painel em todo host', () => {
    expect(documentTitle({ hostname: 'xdriver.corp.ihuull.com', pathname: '/' })).toBe('XDRIVER')
    expect(documentTitle({ hostname: 'xchat.ihuull.com', pathname: '/' })).toBe('XCHAT')
    expect(documentTitle({ hostname: 'xvpn.ihuull.com', pathname: '/' })).toBe('XVPN')
    expect(documentTitle({ hostname: 'xgit.corp.ihuull.com', pathname: '/' })).toBe('XGIT')
    expect(documentTitle({ hostname: 'xgit.corp.ihuull.com', pathname: '/repositories' })).toBe(
      'Repositórios · XGIT',
    )
  })

  it('página admin e portal', () => {
    expect(documentTitle({ hostname: 'xvpn.ihuull.com', pathname: '/admin/users' })).toBe('Usuários · XVPN')
    expect(documentTitle({ hostname: 'xadmin.corp.ihuull.com', pathname: '/admin/users' })).toBe('Usuários · XADMIN')
    expect(documentTitle({ hostname: 'xadmin.corp.ihuull.com', pathname: '/admin/xgit' })).toBe('Repositórios · XADMIN')
    expect(documentTitle({ hostname: 'xadmin.corp.ihuull.com', pathname: '/admin/xgit/settings' })).toBe(
      'Configurações · XADMIN',
    )
    expect(documentTitle({ hostname: 'xadmin.corp.ihuull.com', pathname: '/admin/servers' })).toBe('Servidores · XADMIN')
    expect(documentTitle({ hostname: 'xadmin.corp.ihuull.com', pathname: '/admin/networks' })).toBe('Redes · XADMIN')
    expect(documentTitle({ hostname: 'xadmin.corp.ihuull.com', pathname: '/admin/services' })).toBe('Instâncias · XADMIN')
    expect(documentTitle({ hostname: 'xadmin.corp.ihuull.com', pathname: '/admin/compute/settings' })).toBe(
      'Configurações · XADMIN',
    )
    expect(documentTitle({ hostname: 'xadmin.corp.ihuull.com', pathname: '/admin/dns/public' })).toBe(
      'Zonas públicas · XADMIN',
    )
    expect(documentTitle({ hostname: 'xvpn.ihuull.com', pathname: '/my/profile' })).toBe('Perfil · XVPN')
    expect(documentTitle({ hostname: 'xauth.ihuull.com', pathname: '/login' })).toBe('Entrar · ihuull')
  })

  it('aproveita conversa, pasta e perfil no título', () => {
    expect(
      documentTitle({
        hostname: 'xchat.corp.ihuull.com',
        pathname: '/social/messages',
        chatTitle: 'Ana',
      }),
    ).toBe('Ana · XCHAT')
    expect(
      documentTitle({
        hostname: 'xdriver.corp.ihuull.com',
        pathname: '/',
        pageOverride: 'xxxx · Compartilhado',
      }),
    ).toBe('xxxx · Compartilhado · XDRIVER')
    expect(documentTitle({ hostname: 'xgroup.corp.ihuull.com', pathname: '/social/u/rootkit' })).toBe(
      'rootkit · XGROUP',
    )
    expect(documentTitle({ hostname: 'xgroup.ihuull.com', pathname: '/rootkit' })).toBe('rootkit · XGROUP')
  })

  it('mostra não-lidas no prefixo', () => {
    expect(
      documentTitle({
        hostname: 'xchat.corp.ihuull.com',
        pathname: '/social/messages',
        chatTitle: 'Ana',
        unread: 3,
      }),
    ).toBe('(3) Ana · XCHAT')
    expect(
      documentTitle({
        hostname: 'xchat.corp.ihuull.com',
        pathname: '/social/messages',
        unread: 120,
      }),
    ).toBe('(99+) Mensagens · XCHAT')
  })
})

describe('titlePage', () => {
  it('ignora o fallback Painel', () => {
    expect(titlePage({ hostname: 'xvpn.ihuull.com', pathname: '/nope' })).toBe('')
  })
})

describe('focusedChatKey', () => {
  it('prioriza popout aberto; na página de mensagens usa activeKey', () => {
    expect(
      focusedChatKey({
        pathname: '/admin/users',
        activeKey: 'dm:1',
        popouts: [{ key: 'dm:2', minimized: false }],
      }),
    ).toBe('dm:2')
    expect(
      focusedChatKey({
        pathname: '/admin/users',
        activeKey: 'dm:1',
        popouts: [{ key: 'dm:2', minimized: true }],
      }),
    ).toBeNull()
    expect(
      focusedChatKey({
        pathname: '/social/messages',
        activeKey: 'dm:1',
        popouts: [],
      }),
    ).toBe('dm:1')
  })
})
