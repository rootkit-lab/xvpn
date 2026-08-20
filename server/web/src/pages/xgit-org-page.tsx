import { useCallback } from 'react'
import { Link, useParams } from 'react-router-dom'
import { FolderGit2, Workflow } from 'lucide-react'
import { api, type ForgeOrg, type Project } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { xgitPath, xgitRepoPath } from '@/lib/xgit'
import { RepoListRow } from '@/pages/xgit-repo-card'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'

function RepoGroup({ title, hint, repos }: { title: string; hint?: string; repos: Project[] }) {
  return (
    <section className="flex flex-col gap-3">
      <div>
        <h2 className="font-display text-base font-semibold">{title}</h2>
        {hint ? <p className="text-xs text-muted-foreground">{hint}</p> : null}
      </div>
      {repos.length === 0 ? (
        <p className="text-sm text-muted-foreground">Nenhum repositório neste time.</p>
      ) : (
        <ul className="divide-y divide-border/60 overflow-hidden rounded-2xl border border-border/60">
          {repos.map((p) => (
            <li key={`${p.org}/${p.slug}`}>
              <RepoListRow project={p} />
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

export function XgitOrgPage() {
  const { org = '' } = useParams()
  const fetchOrg = useCallback(() => api.getForgeOrg(org), [org])
  const { data, loading, error } = usePollingData(fetchOrg, 20_000)

  if (loading || !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-64 w-full" />
  }

  const packages = data.teams.find((t) => t.slug === 'packages')
  const workflows = data.teams.find((t) => t.slug === 'workflows')
  const applyHref = data.root[0]
    ? xgitRepoPath(data.root[0].org, data.root[0].slug, 'actions/new')
    : packages?.repos[0]
      ? xgitRepoPath(packages.repos[0].org, packages.repos[0].slug, 'actions/new')
      : xgitPath(`${data.slug}`)

  return (
    <div className="mx-auto flex w-full max-w-4xl flex-col gap-8">
      <header>
        <div className="flex flex-wrap items-center gap-2">
          <FolderGit2 className="size-5 text-muted-foreground" />
          <h1 className="font-display text-2xl font-semibold tracking-tight">{data.name}</h1>
          <Badge variant="outline">org</Badge>
        </div>
        {data.description ? <p className="mt-1 text-sm text-muted-foreground">{data.description}</p> : null}
      </header>

      <RepoGroup title="Repositórios" hint="Produtos na raiz da org." repos={data.root} />
      {packages ? <RepoGroup title="Packages" hint="Time exemplos / packages." repos={packages.repos} /> : null}

      {workflows ? (
        <section className="flex flex-col gap-3">
          <div>
            <h2 className="font-display text-base font-semibold">Workflows</h2>
            <p className="text-xs text-muted-foreground">Templates abertos (CI + Publish) e repos do time.</p>
          </div>
          {workflows.templates && workflows.templates.length > 0 ? (
            <ul className="grid gap-2 sm:grid-cols-2">
              {workflows.templates.map((tpl) => (
                <li key={tpl.id}>
                  <Link
                    to={`${applyHref}?category=${encodeURIComponent(tpl.category)}`}
                    className="flex items-start gap-2 rounded-xl border border-border/60 px-3 py-2 hover:bg-muted/30"
                  >
                    <Workflow className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
                    <span>
                      <span className="block text-sm font-medium">{tpl.name}</span>
                      <span className="block text-xs text-muted-foreground">{tpl.description}</span>
                    </span>
                  </Link>
                </li>
              ))}
            </ul>
          ) : null}
          {workflows.repos.length > 0 ? (
            <ul className="divide-y divide-border/60 overflow-hidden rounded-2xl border border-border/60">
              {workflows.repos.map((p) => (
                <li key={`${p.org}/${p.slug}`}>
                  <RepoListRow project={p} />
                </li>
              ))}
            </ul>
          ) : null}
        </section>
      ) : null}
    </div>
  )
}

export type { ForgeOrg }
