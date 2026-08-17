import { describe, expect, it } from 'vitest'
import { plexusLinkDistance, plexusParticleCount } from './network-plexus'

describe('network plexus', () => {
  it('escala a densidade com a área e limita o teto', () => {
    expect(plexusParticleCount(0, 0)).toBe(22)
    expect(plexusParticleCount(800, 900)).toBeGreaterThan(22)
    expect(plexusParticleCount(4000, 3000)).toBe(68)
  })

  it('mantém o alcance das linhas numa faixa útil', () => {
    expect(plexusLinkDistance(200, 200)).toBe(96)
    expect(plexusLinkDistance(900, 800)).toBeGreaterThan(96)
    expect(plexusLinkDistance(2000, 2000)).toBe(168)
  })
})
