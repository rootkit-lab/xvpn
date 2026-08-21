import { useCallback, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { codespaceOpenHref } from '@/lib/product-host'
import { xgitRepoPath } from '@/lib/xgit'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'

type Filter = 'all' | 'mine' | 'attention' | 'active' | 'completed'

export function XgitAgentsPage() {
  const { org = '', slug = '' } = useParams()
  const repo = `${org}/${slug}`
  const [filter, setFilter] = useState<Filter>('all')
  const fetchAgents = useCallback(
    () => api.listProjectAgents(repo, filter === 'all' ? undefined : filter),
    [repo, filter],
  )
  const { data, loading, error } = usePollingData(fetchAgents, 15_000)
  const items = data?.items ?? []
  const settings = xgitRepoPath(org, slug, 'settings')

  const filters = useMemo(
    () =>
      [
        { id: 'all' as const, label: 'Newest' },
        { id: 'mine' as const, label: `Created by me${data?.mine ? ` (${data.mine})` : ''}` },
        { id: 'attention' as const, label: `Needs attention${data?.attention ? ` (${data.attention})` : ''}` },
        { id: 'active' as const, label: 'Active' },
        { id: 'completed' as const, label: 'Completed' },
      ],
    [data?.mine, data?.attention],
  )

  if (loading && !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-48 w-full" />
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap gap-2">
        {filters.map((f) => (
          <Button key={f.id} size="sm" variant={filter === f.id ? 'default' : 'outline'} onClick={() => setFilter(f.id)}>
            {f.label}
          </Button>
        ))}
      </div>
      {items.length === 0 ? (
        <div className="rounded-md border border-dashed border-border/70 px-4 py-8 text-sm text-muted-foreground">
          <p>Nenhuma sessão de agente neste repositório.</p>
          <p className="mt-2">
            Configure o ambiente em <Link to={settings}>Settings → Codespaces</Link>.
          </p>
        </div>
      ) : (
        <ul className="flex flex-col gap-2">
          {items.map((cs) => (
            <li key={cs.id} className="flex items-center justify-between gap-3 rounded-md border border-border/60 px-3 py-2">
              <div className="min-w-0">
                <p className="truncate text-sm font-medium">
                  {cs.author} · {cs.branch}
                </p>
                <p className="text-xs text-muted-foreground">{cs.kind}</p>
              </div>
              <div className="flex items-center gap-2">
                <Badge variant="outline">{cs.status}</Badge>
                <Button asChild size="sm">
                  <a href={codespaceOpenHref({ id: cs.id, runtime_url: cs.runtime_url || cs.open_url })}>Abrir</a>
                </Button>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
