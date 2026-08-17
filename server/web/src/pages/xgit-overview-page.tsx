import { useCallback } from 'react'
import { Link } from 'react-router-dom'
import { MessageCircle } from 'lucide-react'
import { openChat } from '@chat/messenger/open-chat'
import { api, type XgitActivityItem } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { formatRelativeTime } from '@/lib/format'
import { languageColor, xgitPath } from '@/lib/xgit'
import { PopularRepoCard } from '@/pages/xgit-repo-card'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'

export function XgitOverviewPage() {
  const fetchOverview = useCallback(() => api.getXgitOverview(), [])
  const { data, loading, error } = usePollingData(fetchOverview, 30_000)

  if (loading || !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-64 w-full" />
  }

  return (
    <div className="flex flex-col gap-8">
      <section>
        <div className="mb-3 flex items-center justify-between">
          <h2 className="font-display text-base font-semibold">Repositórios populares</h2>
          <Link to="/repositories" className="text-xs text-primary hover:underline">
            Ver todos
          </Link>
        </div>
        {data.popular.length === 0 ? (
          <p className="text-sm text-muted-foreground">Nenhum repositório ainda.</p>
        ) : (
          <div className="grid gap-3 sm:grid-cols-2">
            {data.popular.map((p) => (
              <PopularRepoCard key={p.slug} project={p} />
            ))}
          </div>
        )}
      </section>

      <section className="watch-complication rounded-[18px] p-4">
        <div className="mb-3 flex flex-wrap items-end justify-between gap-2">
          <h2 className="font-display text-base font-semibold">
            {data.contributions.total.toLocaleString('pt-BR')} contribuições no último ano
          </h2>
          <span className="text-xs text-muted-foreground">{new Date().getFullYear()}</span>
        </div>
        <ContributionGraph days={data.contributions.days} />
        <div className="mt-2 flex items-center justify-end gap-1 text-[10px] text-muted-foreground">
          Menos
          {[0, 1, 2, 3, 4].map((level) => (
            <span key={level} className="size-2.5 rounded-[2px]" style={{ background: heatColor(level) }} />
          ))}
          Mais
        </div>
      </section>

      <section>
        <h2 className="font-display mb-3 text-base font-semibold">Atividade</h2>
        <div className="flex flex-col gap-4">
          {data.activity.length === 0 ? (
            <p className="text-sm text-muted-foreground">Sem atividade neste mês.</p>
          ) : (
            data.activity.map((item, i) => <ActivityRow key={`${item.kind}-${item.slug ?? item.month}-${i}`} item={item} />)
          )}
        </div>
      </section>
    </div>
  )
}

function heatColor(level: number): string {
  if (level <= 0) return 'color-mix(in oklch, var(--foreground) 8%, transparent)'
  const pct = [0, 28, 48, 70, 92][Math.min(4, level)]
  return `color-mix(in oklch, var(--safe) ${pct}%, transparent)`
}

function ContributionGraph({ days }: { days: { date: string; count: number }[] }) {
  const max = Math.max(1, ...days.map((d) => d.count))
  const byDate = new Map(days.map((d) => [d.date, d.count]))
  const last = days[days.length - 1]?.date
  if (!last) return null
  const end = new Date(`${last}T00:00:00Z`)
  const start = new Date(end)
  start.setUTCDate(start.getUTCDate() - 52 * 7 - end.getUTCDay())
  const cells: { date: string; count: number }[] = []
  for (let i = 0; i < 53 * 7; i++) {
    const d = new Date(start)
    d.setUTCDate(start.getUTCDate() + i)
    const key = d.toISOString().slice(0, 10)
    cells.push({ date: key, count: byDate.get(key) ?? 0 })
  }
  const weeks: { date: string; count: number }[][] = []
  for (let w = 0; w < 53; w++) {
    weeks.push(cells.slice(w * 7, w * 7 + 7))
  }
  return (
    <div className="overflow-x-auto">
      <div className="flex gap-[3px]">
        {weeks.map((week, wi) => (
          <div key={wi} className="flex flex-col gap-[3px]">
            {week.map((cell) => {
              const level = cell.count <= 0 ? 0 : Math.min(4, Math.ceil((cell.count / max) * 4))
              return (
                <span
                  key={cell.date}
                  title={`${cell.date}: ${cell.count}`}
                  className="size-2.5 rounded-[2px]"
                  style={{ background: heatColor(level) }}
                />
              )
            })}
          </div>
        ))}
      </div>
    </div>
  )
}

function ActivityRow({ item }: { item: XgitActivityItem }) {
  if (item.kind === 'commits') {
    return (
      <p className="text-sm">
        Criou <strong>{item.count}</strong> commits em <strong>{item.repo_count}</strong> repositórios
      </p>
    )
  }
  if (item.kind === 'repos_created') {
    return (
      <p className="text-sm text-muted-foreground">
        Criou {item.count} repositórios
      </p>
    )
  }
  if (item.kind === 'repo_created') {
    return (
      <div className="flex items-center justify-between gap-3 text-sm">
        <Link to={xgitPath(item.slug ?? '')} className="text-primary hover:underline">
          {item.slug}
        </Link>
        <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
          <span className="size-2 rounded-full" style={{ background: languageColor(item.language) }} />
          {item.language}
          <span>{formatRelativeTime(item.created_at)}</span>
        </span>
      </div>
    )
  }
  if (item.kind === 'merge_request') {
    return (
      <div className="watch-complication rounded-[18px] p-4">
        <p className="text-sm text-muted-foreground">
          Abriu um merge request em{' '}
          <Link to={xgitPath(item.slug ?? '')} className="text-primary hover:underline">
            {item.slug}
          </Link>
          {item.comments ? ` que recebeu ${item.comments} comentários no XCHAT` : ''}
        </p>
        <Link
          to={xgitPath(`${item.slug}/mrs/${item.number}`)}
          className="mt-2 block font-medium hover:underline"
        >
          {item.title}
        </Link>
        {item.description ? (
          <p className="mt-1 line-clamp-2 text-sm text-muted-foreground">{item.description}</p>
        ) : null}
        <div className="mt-3 flex flex-wrap items-center gap-2">
          {item.thread_id ? (
            <Button
              type="button"
              size="sm"
              variant="outline"
              onClick={() => openChat({ dmId: item.thread_id, title: `!${item.number} ${item.title}` })}
            >
              <MessageCircle className="size-3.5" />
              Discutir no XCHAT
            </Button>
          ) : null}
          <span className="text-xs text-muted-foreground">{formatRelativeTime(item.created_at)}</span>
        </div>
      </div>
    )
  }
  return null
}
