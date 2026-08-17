import { describe, expect, it } from 'vitest'
import {
  attachmentRef,
  fallbackBannerClass,
  parseAttachmentId,
  parseBannerTone,
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
  it('aceita tons do design system', () => {
    expect(parseBannerTone('tone:primary')).toBe('primary')
    expect(parseBannerTone('tone:rainbow')).toBeNull()
    expect(parseBannerTone('attachment:1')).toBeNull()
  })
})

describe('fallbackBannerClass', () => {
  it('é estável por username', () => {
    expect(fallbackBannerClass('rootkit')).toBe(fallbackBannerClass('rootkit'))
    expect(fallbackBannerClass('rootkit')).toMatch(/^bg-/)
  })
})
