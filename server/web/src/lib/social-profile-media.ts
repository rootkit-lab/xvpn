export const PROFILE_THEMES = [
  { id: 'primary', label: 'Ciano', cssVar: '--primary' },
  { id: 'safe', label: 'Verde', cssVar: '--safe' },
  { id: 'xgroup', label: 'Magenta', cssVar: '--product-xgroup' },
  { id: 'xdriver', label: 'Âmbar', cssVar: '--product-xdriver' },
  { id: 'marketplace', label: 'Azul', cssVar: '--product-marketplace' },
  { id: 'glow-amber', label: 'Ouro', cssVar: '--glow-amber' },
  { id: 'glow-red', label: 'Vermelho', cssVar: '--glow-red' },
] as const

export type ProfileThemeId = (typeof PROFILE_THEMES)[number]['id']

const THEME_IDS = new Set<string>(PROFILE_THEMES.map((t) => t.id))

/** Tons antigos da capa (`tone:chart-2`) → paleta atual. */
const LEGACY_TONE: Record<string, ProfileThemeId> = {
  primary: 'primary',
  'chart-2': 'safe',
  'chart-3': 'xgroup',
  'chart-4': 'xdriver',
  'chart-5': 'marketplace',
  safe: 'safe',
  xgroup: 'xgroup',
  xdriver: 'xdriver',
  marketplace: 'marketplace',
  'glow-amber': 'glow-amber',
  'glow-red': 'glow-red',
}

export function isProfileTheme(id: string): id is ProfileThemeId {
  return THEME_IDS.has(id)
}

export function profileThemeById(id: ProfileThemeId) {
  return PROFILE_THEMES.find((t) => t.id === id) ?? PROFILE_THEMES[0]
}

export function fallbackTheme(username: string): ProfileThemeId {
  const n = [...username].reduce((acc, ch) => acc + ch.charCodeAt(0), 0)
  return PROFILE_THEMES[n % PROFILE_THEMES.length].id
}

export function resolveProfileTheme(theme: string | undefined, bannerUrl: string | undefined, username: string): ProfileThemeId {
  if (theme && isProfileTheme(theme)) return theme
  const fromBanner = parseBannerTone(bannerUrl)
  if (fromBanner) return fromBanner
  return fallbackTheme(username)
}

export function profileThemeStyle(theme: ProfileThemeId): Record<string, string> {
  const { cssVar } = profileThemeById(theme)
  return {
    '--profile-accent': `var(${cssVar})`,
    '--profile-accent-soft': `color-mix(in oklch, var(${cssVar}) 22%, transparent)`,
    '--profile-accent-strong': `color-mix(in oklch, var(${cssVar}) 55%, transparent)`,
  }
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

export function parseBannerTone(ref: string | undefined): ProfileThemeId | null {
  if (!ref?.startsWith('tone:')) return null
  const mapped = LEGACY_TONE[ref.slice(5)]
  return mapped ?? null
}

export function isInlineMediaUrl(ref: string): boolean {
  return ref.startsWith('blob:') || ref.startsWith('data:')
}
