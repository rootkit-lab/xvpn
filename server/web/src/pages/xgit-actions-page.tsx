import { useCallback } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { CirclePlus, GitBranch, Server } from 'lucide-react'
import { api, type CiJob, type CiWorkflow } from '@/lib/api'
import { formatCompactDuration, formatRelativeTime } from '@/lib/format'
import { usePollingData } from '@/hooks/use-polling-data'
import { isXgitAdminHost, xgitPath } from '@/lib/xgit'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import { CiRunStatusIcon, ciEventLabel } from '@/pages/ci-job-page'
import { ProjectServicesCard } from '@/pages/project-detail-page'

export function XgitActionsPage() {
  const { slug = '' } = useParams()
  const [search, setSearch] = useSearchParams()
  const workflow = search.get('workflow') || ''
  const section = search.get('view') || 'runs'
  const fetchJobs = useCallback(() => api.listCiJobs(slug, workflow || undefined), [slug, workflow])
  const { data, loading, error } = usePollingData(fetchJobs, 8_000)
  const workflows = data?.workflows ?? [{ name: 'ci', path: '.xvpn-ci.sh' }]
  const items = data?.items ?? []

  if (loading && !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-48 w-full" />
  }

  return (
    <div className="grid gap-6 lg:grid-cols-[220px_1fr]">
      <aside className="flex flex-col gap-4 text-sm">
        <Button type="button" variant="outline" size="sm" className="justify-start gap-2" disabled>
          <CirclePlus className="size-3.5" />
          New workflow
        </Button>
        <div>
          <p className="hud-label px-2 text-muted-foreground/70">Workflows</p>
          <button
            type="button"
            className={cn(
              'mt-1 flex w-full rounded-md px-2 py-1.5 text-left',
              !workflow ? 'bg-muted/50 text-foreground' : 'text-muted-foreground hover:text-foreground',
            )}
            onClick={() => setSearch({})}
          >
            All workflows
          </button>
          {workflows.map((wf) => (
            <WorkflowLink
              key={wf.name}
              workflow={wf}
              active={workflow === wf.name}
              onSelect={() => setSearch({ workflow: wf.name })}
            />
          ))}
        </div>
        <div>
          <p className="hud-label px-2 text-muted-foreground/70">Management</p>
          <button
            type="button"
            className={cn(
              'mt-1 flex w-full rounded-md px-2 py-1.5 text-left',
              section === 'runners' ? 'bg-muted/50 text-foreground' : 'text-muted-foreground hover:text-foreground',
            )}
            onClick={() => setSearch({ view: 'runners' })}
          >
            Runners
          </button>
          <button
            type="button"
            className={cn(
              'flex w-full rounded-md px-2 py-1.5 text-left',
              section === 'services' ? 'bg-muted/50 text-foreground' : 'text-muted-foreground hover:text-foreground',
            )}
            onClick={() => setSearch({ view: 'services' })}
          >
            Services
          </button>
        </div>
      </aside>

      <div className="min-w-0">
        {section === 'runners' ? (
          <RunnersPanel slug={slug} />
        ) : section === 'services' ? (
          <ProjectServicesCard slug={slug} />
        ) : (
          <RunsPanel slug={slug} workflow={workflow} items={items} />
        )}
      </div>
    </div>
  )
}

function WorkflowLink({
  workflow,
  active,
  onSelect,
}: {
  workflow: CiWorkflow
  active: boolean
  onSelect: () => void
}) {
  return (
    <button
      type="button"
      className={cn(
        'flex w-full rounded-md px-2 py-1.5 text-left font-mono text-xs',
        active ? 'bg-muted/50 text-foreground' : 'text-muted-foreground hover:text-foreground',
      )}
      onClick={onSelect}
    >
      {workflow.name}
    </button>
  )
}

function RunsPanel({ slug, workflow, items }: { slug: string; workflow: string; items: CiJob[] }) {
  return (
    <div className="flex flex-col gap-3">
      <div>
        <h2 className="font-display text-lg">{workflow || 'All workflows'}</h2>
        <p className="text-sm text-muted-foreground">
          {items.length} {items.length === 1 ? 'workflow run' : 'workflow runs'}
        </p>
      </div>
      {items.length === 0 ? (
        <div className="watch-complication rounded-[18px] p-8 text-center">
          <p className="font-medium">No workflow runs</p>
          <p className="mt-1 text-sm text-muted-foreground">
            Push ou abra um pull request. O workflow <code className="font-mono text-xs">ci</code> vive em{' '}
            <code className="font-mono text-xs">.xvpn-ci.sh</code>.
          </p>
        </div>
      ) : (
        <ul className="watch-complication divide-y divide-border/50 overflow-hidden rounded-[18px]">
          {items.map((job) => (
            <li key={job.number}>
              <Link
                to={xgitPath(`${slug}/actions/${job.number}`)}
                className="flex items-start gap-3 px-4 py-3 hover:bg-muted/30"
              >
                <CiRunStatusIcon status={job.status} className="mt-0.5" />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium">{job.title || `ci #${job.number}`}</p>
                  <p className="truncate text-xs text-muted-foreground">
                    {job.workflow || 'ci'} #{job.number}: {ciEventLabel(job)}
                    {job.actor ? ` by ${job.actor}` : ''}
                  </p>
                </div>
                <div className="hidden shrink-0 items-center gap-3 text-xs text-muted-foreground sm:flex">
                  <span className="inline-flex items-center gap-1 font-mono">
                    <GitBranch className="size-3" />
                    {job.branch || job.ref.replace('refs/heads/', '')}
                  </span>
                  <span>{formatRelativeTime(job.created_at)}</span>
                  <span className="w-14 text-right">{job.duration_ms != null ? formatCompactDuration(job.duration_ms) : '—'}</span>
                </div>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

function RunnersPanel({ slug }: { slug: string }) {
  const fetchRunners = useCallback(() => api.listProjectRunners(slug), [slug])
  const { data, loading, error } = usePollingData(fetchRunners, 15_000)
  const items = data?.items ?? []
  const admin = isXgitAdminHost()

  if (loading && !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-32 w-full" />
  }

  return (
    <div className="flex flex-col gap-3">
      <div>
        <h2 className="font-display text-lg">Runners</h2>
        <p className="text-sm text-muted-foreground">
          Peers <code className="font-mono text-xs">role=runner</code> da malha. A execução não roda no xvpn-server.
        </p>
      </div>
      {items.length === 0 ? (
        <div className="watch-complication rounded-[18px] p-8 text-center">
          <Server className="mx-auto size-6 text-muted-foreground" />
          <p className="mt-2 font-medium">No runners</p>
          <p className="mt-1 text-sm text-muted-foreground">
            {admin ? (
              <Link to="/admin/servers" className="text-primary hover:underline">
                Compute → Servidores
              </Link>
            ) : (
              'Peça a um admin para marcar um peer como runner.'
            )}
          </p>
        </div>
      ) : (
        <ul className="watch-complication divide-y divide-border/50 overflow-hidden rounded-[18px]">
          {items.map((r) => (
            <li key={r.hostname} className="flex items-center justify-between gap-3 px-4 py-3 text-sm">
              <div>
                <p className="font-medium">{r.hostname}</p>
                <p className="text-xs text-muted-foreground">
                  {r.name}
                  {r.wg_ip ? ` · ${r.wg_ip}` : ''}
                  {r.labels?.length ? ` · ${r.labels.join(', ')}` : ''}
                </p>
              </div>
              <span className="text-xs text-muted-foreground">{r.status}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
