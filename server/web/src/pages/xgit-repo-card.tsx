import type { MouseEvent } from 'react'
import { Link } from 'react-router-dom'
import { Star } from 'lucide-react'
import { toast } from 'sonner'
import { api, ApiError, type Project } from '@/lib/api'
import { formatRelativeTime } from '@/lib/format'
import { languageColor, xgitPath } from '@/lib/xgit'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

export function RepoSpark({ values }: { values?: number[] }) {
  const pts = values ?? []
  if (pts.length === 0) return null
  const max = Math.max(1, ...pts)
  const w = 88
  const h = 28
  const step = w / Math.max(1, pts.length - 1)
  const d = pts
    .map((n, i) => `${i === 0 ? 'M' : 'L'}${(i * step).toFixed(1)},${(h - (n / max) * (h - 2) - 1).toFixed(1)}`)
    .join(' ')
  return (
    <svg width={w} height={h} viewBox={`0 0 ${w} ${h}`} className="text-[var(--safe)]" aria-hidden>
      <path d={d} fill="none" stroke="currentColor" strokeWidth="1.5" />
    </svg>
  )
}

export function LanguageDot({ name }: { name?: string }) {
  if (!name) return null
  return (
    <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
      <span className="size-2.5 rounded-full" style={{ background: languageColor(name) }} />
      {name}
    </span>
  )
}

export function StarButton({
  project,
  onChanged,
}: {
  project: Project
  onChanged?: (next: Project) => void
}) {
  async function toggle(e: MouseEvent) {
    e.preventDefault()
    e.stopPropagation()
    try {
      const next = await api.toggleProjectStar(project.slug)
      onChanged?.(next)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao marcar estrela')
    }
  }
  return (
    <Button type="button" variant="outline" size="sm" className="h-8 gap-1.5 px-2" onClick={toggle}>
      <Star className={cn('size-3.5', project.starred && 'fill-current text-[var(--product)]')} />
      {project.star_count ?? 0}
    </Button>
  )
}

export function PopularRepoCard({ project }: { project: Project }) {
  return (
    <Link
      to={xgitPath(project.slug)}
      className="watch-complication flex flex-col gap-2 rounded-[18px] p-4 hover:bg-white/6"
    >
      <div className="flex items-center justify-between gap-2">
        <span className="truncate font-medium text-primary">{project.slug}</span>
        <Badge variant="outline">{project.visibility}</Badge>
      </div>
      <p className="line-clamp-2 min-h-10 text-xs text-muted-foreground">{project.description || project.name}</p>
      <LanguageDot name={project.language} />
    </Link>
  )
}

export function RepoListRow({
  project,
  onStarred,
}: {
  project: Project
  onStarred?: (next: Project) => void
}) {
  const updated = project.last_commit_at || project.updated_at
  return (
    <div className="flex flex-wrap items-start justify-between gap-4 border-b border-border/60 py-5">
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <Link to={xgitPath(project.slug)} className="font-medium text-primary hover:underline">
            {project.slug}
          </Link>
          <Badge variant="outline">{project.visibility}</Badge>
        </div>
        <p className="mt-1 max-w-2xl text-sm text-muted-foreground">{project.description || project.name}</p>
        <div className="mt-2 flex flex-wrap items-center gap-3">
          <LanguageDot name={project.language} />
          <span className="text-xs text-muted-foreground">Atualizado {formatRelativeTime(updated)}</span>
        </div>
      </div>
      <div className="flex items-center gap-3">
        <StarButton project={project} onChanged={onStarred} />
        <RepoSpark values={project.spark} />
      </div>
    </div>
  )
}
