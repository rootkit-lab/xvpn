import { describe, expect, it } from 'vitest'
import { livePresence, normalizePresence, presenceLabel } from './social-presence'

describe('normalizePresence', () => {
  it('esconde invisible e lixo como offline', () => {
    expect(normalizePresence('online')).toBe('online')
    expect(normalizePresence('away')).toBe('away')
    expect(normalizePresence('invisible')).toBe('offline')
    expect(normalizePresence('')).toBe('offline')
  })
})

describe('presenceLabel', () => {
  it('rótulo em pt-BR', () => {
    expect(presenceLabel('online')).toBe('Online')
    expect(presenceLabel('away')).toBe('Ausente')
    expect(presenceLabel('offline')).toBe('Offline')
  })
})

describe('livePresence', () => {
  it('prefere o snapshot ao vivo do chat', () => {
    expect(livePresence(2, 'offline', { 2: 'online' })).toBe('online')
    expect(livePresence(2, 'online', { 2: 'invisible' })).toBe('offline')
    expect(livePresence(2, 'away')).toBe('away')
  })
})
