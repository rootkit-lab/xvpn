export type DriverFileKind = 'folder' | 'text' | 'image' | 'video' | 'audio' | 'pdf' | 'archive' | 'other'

const TEXT_EXT = new Set([
  '.txt',
  '.md',
  '.markdown',
  '.json',
  '.csv',
  '.tsv',
  '.yml',
  '.yaml',
  '.xml',
  '.html',
  '.htm',
  '.css',
  '.js',
  '.ts',
  '.tsx',
  '.jsx',
  '.go',
  '.py',
  '.sh',
  '.bash',
  '.env',
  '.ini',
  '.toml',
  '.conf',
  '.cfg',
  '.log',
  '.svg',
])

const IMAGE_EXT = new Set(['.jpg', '.jpeg', '.png', '.gif', '.webp', '.avif', '.bmp'])
const VIDEO_EXT = new Set(['.mp4', '.webm', '.mov', '.mkv', '.ogv'])
const AUDIO_EXT = new Set(['.mp3', '.wav', '.ogg', '.m4a', '.flac', '.aac'])

export function fileSuffix(name: string): string {
  const n = name.trim().toLowerCase()
  if (n.endsWith('.tar.gz')) return '.tar.gz'
  if (n.endsWith('.tar.bz2')) return '.tar.bz2'
  const i = n.lastIndexOf('.')
  return i >= 0 ? n.slice(i) : ''
}

export function driverFileKind(name: string, isDir: boolean): DriverFileKind {
  if (isDir) return 'folder'
  const ext = fileSuffix(name)
  if (TEXT_EXT.has(ext)) return 'text'
  if (IMAGE_EXT.has(ext)) return 'image'
  if (VIDEO_EXT.has(ext)) return 'video'
  if (AUDIO_EXT.has(ext)) return 'audio'
  if (ext === '.pdf') return 'pdf'
  if (ext === '.zip' || ext === '.tar.gz' || ext === '.tgz' || ext === '.tar.bz2' || ext === '.7z' || ext === '.rar') {
    return 'archive'
  }
  return 'other'
}

export function archiveExtractable(name: string): boolean {
  const ext = fileSuffix(name)
  return ext === '.zip' || ext === '.tar.gz' || ext === '.tgz'
}

export function driverOpenMode(kind: DriverFileKind): 'edit' | 'view' | 'folder' | null {
  if (kind === 'folder') return 'folder'
  if (kind === 'text') return 'edit'
  if (kind === 'image' || kind === 'video' || kind === 'audio' || kind === 'pdf') return 'view'
  return null
}

export function driverItemHref(mode: 'edit' | 'view', root: string, path: string): string {
  const sp = new URLSearchParams({ root, path })
  return `/${mode}?${sp}`
}
