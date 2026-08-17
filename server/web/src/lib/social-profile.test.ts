import { describe, expect, it } from 'vitest'
import {
  isProfileUsername,
  isSocialProfilePath,
  profileHref,
  profilePath,
  profileUsernameFromPath,
} from './social-profile'

describe('isProfileUsername', () => {
  it('aceita username Unix e rejeita rota reservada', () => {
    expect(isProfileUsername('rootkit')).toBe(true)
    expect(isProfileUsername('ab')).toBe(false)
    expect(isProfileUsername('Admin')).toBe(false)
    expect(isProfileUsername('social')).toBe(false)
    expect(isProfileUsername('admin')).toBe(false)
    expect(isProfileUsername('messages')).toBe(false)
  })
})

describe('profilePath / profileHref', () => {
  it('monta a URL amigável no host público', () => {
    expect(profilePath('rootkit')).toBe('/rootkit')
    expect(profileHref('rootkit')).toBe('https://xgroup.ihuull.com/rootkit')
  })
})

describe('profileUsernameFromPath', () => {
  it('lê /social/u, /xgroup/u e /:user só no host xgroup', () => {
    expect(profileUsernameFromPath('/social/u/rootkit')).toBe('rootkit')
    expect(profileUsernameFromPath('/xgroup/u/rootkit')).toBe('rootkit')
    expect(profileUsernameFromPath('/rootkit', 'xgroup.ihuull.com')).toBe('rootkit')
    expect(profileUsernameFromPath('/rootkit', 'xvpn.ihuull.com')).toBe('')
    expect(profileUsernameFromPath('/social', 'xgroup.ihuull.com')).toBe('')
    expect(profileUsernameFromPath('/social/explore')).toBe('')
    expect(profileUsernameFromPath('/admin')).toBe('')
  })
})

describe('isSocialProfilePath', () => {
  it('só o perfil, não o feed', () => {
    expect(isSocialProfilePath('/social/u/ana')).toBe(true)
    expect(isSocialProfilePath('/ana', 'xgroup.ihuull.com')).toBe(true)
    expect(isSocialProfilePath('/ana', 'xvpn.ihuull.com')).toBe(false)
    expect(isSocialProfilePath('/social')).toBe(false)
    expect(isSocialProfilePath('/social/messages')).toBe(false)
  })
})
