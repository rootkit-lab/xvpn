export const BANNER_TONES = ['primary', 'chart-2', 'chart-3', 'chart-4', 'chart-5'] as const

export type BannerTone = (typeof BANNER_TONES)[number]

const TONE_CLASS: Record<BannerTone, string> = {
  primary: 'bg-primary/35',
  'chart-2': 'bg-chart-2/40',
  'chart-3': 'bg-chart-3/40',
  'chart-4': 'bg-chart-4/35',
  'chart-5': 'bg-chart-5/35',
}

export function attachmentRef(id: number): string {
  return `attachment:${id}`
}

export function parseAttachmentId(ref: string | undefined): number | null {
  if (!ref) return null
  const match = /^attachment:(\d+)$/.exec(ref.trim())
  if (!match) return null
  const id = Number(match[1])
  return Number.isInteger(id) && id > 0 ? id : null
}

export function parseBannerTone(ref: string | undefined): BannerTone | null {
  if (!ref?.startsWith('tone:')) return null
  const tone = ref.slice(5)
  return (BANNER_TONES as readonly string[]).includes(tone) ? (tone as BannerTone) : null
}

export function bannerToneClass(tone: BannerTone): string {
  return TONE_CLASS[tone]
}

export function fallbackBannerClass(username: string): string {
  const n = [...username].reduce((acc, ch) => acc + ch.charCodeAt(0), 0)
  return bannerToneClass(BANNER_TONES[n % BANNER_TONES.length])
}

export function isInlineMediaUrl(ref: string): boolean {
  return ref.startsWith('blob:') || ref.startsWith('data:')
}
