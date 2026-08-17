import { useCallback, useState } from 'react'
import { api, type Project } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { RepoListRow } from '@/pages/xgit-repo-card'
import { Skeleton } from '@/components/ui/skeleton'

export function XgitStarsPage() {
  const fetchStars = useCallback(() => api.listXgitStars(), [])
  const { data, loading, error } = usePollingData(fetchStars, 20_000)
  const [local, setLocal] = useState<Project[] | null>(null)
  const items = local ?? data?.items ?? []

  if (loading && !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-48 w-full" />
  }

  return (
    <div>
      <h2 className="font-display mb-2 text-base font-semibold">Stars</h2>
      {items.length === 0 ? (
        <p className="text-sm text-muted-foreground">Nenhum repositório marcado.</p>
      ) : (
        items.map((p) => (
          <RepoListRow
            key={p.slug}
            project={p}
            onStarred={(next) => {
              setLocal(items.map((it) => (it.slug === next.slug ? { ...it, ...next } : it)).filter((it) => it.starred))
            }}
          />
        ))
      )}
    </div>
  )
}
