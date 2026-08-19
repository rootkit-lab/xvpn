import type { LucideIcon } from 'lucide-react'
import {
  Box,
  File,
  FileCode,
  FileJson,
  FileText,
  Folder,
  GitBranch,
  Image,
  ListTodo,
  Lock,
  Palette,
  Terminal,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { type GitFileGlyph, gitFileGlyph } from '@/lib/git-file'

const GLYPH: Record<GitFileGlyph, { Icon: LucideIcon; color: string }> = {
  folder: { Icon: Folder, color: '#dcb67a' },
  go: { Icon: FileCode, color: '#00add8' },
  ts: { Icon: FileCode, color: '#3178c6' },
  js: { Icon: FileCode, color: '#f1e05a' },
  md: { Icon: FileText, color: '#7d8cff' },
  yaml: { Icon: FileCode, color: '#cb171e' },
  json: { Icon: FileJson, color: '#8b949e' },
  html: { Icon: FileCode, color: '#e34c26' },
  css: { Icon: Palette, color: '#c6538c' },
  python: { Icon: FileCode, color: '#3572a5' },
  shell: { Icon: Terminal, color: '#89e051' },
  image: { Icon: Image, color: '#a371f7' },
  lock: { Icon: Lock, color: '#8b949e' },
  git: { Icon: GitBranch, color: '#f14e32' },
  docker: { Icon: Box, color: '#2496ed' },
  task: { Icon: ListTodo, color: '#89e051' },
  text: { Icon: FileText, color: '#8b949e' },
  file: { Icon: File, color: '#8b949e' },
}

export function GitFileIcon({
  name,
  type,
  className,
}: {
  name: string
  type: string
  className?: string
}) {
  const { Icon, color } = GLYPH[gitFileGlyph(name, type)]
  return <Icon className={cn('size-4 shrink-0', className)} style={{ color }} aria-hidden />
}
