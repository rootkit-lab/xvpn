import { describe, expect, it } from 'vitest'
import {
  attachmentRef,
  fallbackTheme,
  parseAttachmentId,
  parseBannerTone,
  resolveProfileTheme,
} from './social-profile-media'

describe('parseAttachmentId', () => {
  it('aceita só attachment:<id>', () => {
    expect(parseAttachmentId(attachmentRef(12))).toBe(12)
    expect(parseAttachmentId('attachment:0')).toBeNull()
    expect(parseAttachmentId('https://evil.example/x')).toBeNull()
    expect(parseAttachmentId('javascript:alert(1)')).toBeNull()
  })
})

describe('parseBannerTone', () => {
  it('mapeia tons antigos para a paleta', () => {
    expect(parseBannerTone('tone:primary')).toBe('primary')
    expect(parseBannerTone('tone:chart-2')).toBe('safe')
    expect(parseBannerTone('tone:rainbow')).toBeNull()
    expect(parseBannerTone('attachment:1')).toBeNull()
  })
})

describe('resolveProfileTheme', () => {
  it('prefere theme gravado, depois capa, depois hash do user', () => {
    expect(resolveProfileTheme('xgroup', 'tone:primary', 'rootkit')).toBe('xgroup')
    expect(resolveProfileTheme('', 'tone:chart-3', 'rootkit')).toBe('xgroup')
    expect(fallbackTheme('rootkit')).toBe(resolveProfileTheme('', '', 'rootkit'))
  })
})
