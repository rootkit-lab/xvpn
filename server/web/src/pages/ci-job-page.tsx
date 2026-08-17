import { useCallback, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import {
  AlertTriangle,
  CheckCircle2,
  Circle,
  CircleSlash,
  LoaderCircle,
  RotateCw,
  XCircle,
} from 'lucide-react'
import { api, ApiError, type CiJob, type CiJobStatus } from '@/lib/api'
import { formatCompactDuration, formatRelativeTime } from '@/lib/format'
import { usePollingData } from '@/hooks/use-polling-data'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { xgitPath, xgitReposPath } from '@/lib/xgit'
import { cn } from '@/lib/utils'

const STATUS_LABEL: Record<CiJobStatus, string> = {
  awaiting_approval: 'Action required',
  pending: 'Queued',
  running: 'In progress',
  success: 'Success',
  failed: 'Failed',
  canceled: 'Canceled',
}

export function ciEventLabel(job: Pick<CiJob, 'event' | 'trigger' | 'merge_request_number' | 'sha'>): string {
  if (job.event === 'pull_request' || job.trigger === 'mr') {
    return job.merge_request_number
      ? `Pull request #${job.merge_request_number}`
      : 'Pull request'
  }
  const short = job.sha ? job.sha.slice(0, 7) : ''
  return short ? `Commit ${short} pushed` : 'Push'
}

export function CiRunStatusIcon({ status, className }: { status: CiJobStatus; className?: string }) {
  const cls = cn('size-4 shrink-0', className)
  switch (status) {
    case 'success':
      return <CheckCircle2 className={cn(cls, 'text-[var(--safe)]')} aria-label="Success" />
    case 'failed':
      return <XCircle className={cn(cls, 'text-destructive')} aria-label="Failed" />
    case 'canceled':
      return <CircleSlash className={cn(cls, 'text-muted-foreground')} aria-label="Canceled" />
    case 'running':
      return <LoaderCircle className={cn(cls, 'animate-spin text-primary')} aria-label="In progress" />
    case 'awaiting_approval':
      return <AlertTriangle className={cn(cls, 'text-[var(--warning,var(--primary))]')} aria-label="Action required" />
    default:
      return <Circle className={cn(cls, 'text-muted-foreground')} aria-label="Queued" />
  }
}

export function CiJobStatusBadge({ status }: { status: CiJobStatus }) {
  const variant = status === 'success' ? 'secondary' : status === 'failed' ? 'destructive' : 'outline'
  return <Badge variant={variant}>{STATUS_LABEL[status]}</Badge>
}

export function CiJobPage() {
  const { slug = '', n = '' } = useParams()
  const navigate = useNavigate()
  const number = Number(n)
  const fetchJob = useCallback(() => api.getCiJob(slug, number), [slug, number])
  const fetchLog = useCallback(() => api.getCiJobLog(slug, number).catch(() => ''), [slug, number])
  const { data, loading, error, reload } = usePollingData(fetchJob, 8_000)
  const { data: log } = usePollingData(fetchLog, 8_000)
  const [busy, setBusy] = useState(false)

  if (!Number.isFinite(number) || number < 1) {
    return <p className="text-sm text-destructive">Job inválido.</p>
  }
  if (loading || !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-48 w-full" />
  }

  async function act(kind: 'cancel' | 'approve' | 'rerun') {
    setBusy(true)
    try {
      if (kind === 'cancel') {
        await api.cancelCiJob(slug, number)
        toast.success('Run cancelado')
        reload()
      } else if (kind === 'approve') {
        await api.approveCiJob(slug, number)
        toast.success('Workflow aprovado')
        reload()
      } else {
        const next = await api.rerunCiJob(slug, number)
        toast.success(`Re-run #${next.number}`)
        navigate(xgitPath(`${slug}/actions/${next.number}`))
      }
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha na ação')
    } finally {
      setBusy(false)
    }
  }

  const waiting = data.status === 'awaiting_approval'
  const jobs = data.jobs?.length ? data.jobs : [{ name: 'ci', status: data.status }]

  return (
    <div className="flex flex-col gap-6">
      <p className="text-sm text-muted-foreground">
        <Link to={xgitReposPath()} className="hover:underline">
          XGIT
        </Link>
        <span className="px-1.5">/</span>
        <Link to={xgitPath(slug)} className="hover:underline">
          {slug}
        </Link>
        <span className="px-1.5">/</span>
        <Link to={xgitPath(`${slug}/actions`)} className="hover:underline">
          Actions
        </Link>
        <span className="px-1.5">/</span>
        <span className="text-foreground">#{data.number}</span>
      </p>

      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex min-w-0 items-start gap-3">
          <CiRunStatusIcon status={data.status} className="mt-1 size-5" />
          <div className="min-w-0">
            <h1 className="font-display text-xl leading-tight">
              {data.title || `ci #${data.number}`}{' '}
              <span className="text-muted-foreground">#{data.number}</span>
            </h1>
            <p className="mt-1 text-sm text-muted-foreground">
              Triggered via {ciEventLabel(data).toLowerCase()}
              {data.created_at ? ` ${formatRelativeTime(data.created_at)}` : ''}
              {data.actor ? ` · ${data.actor}` : ''}
              {data.branch ? (
                <>
                  {' · '}
                  <code className="font-mono text-xs">{data.branch}</code>
                </>
              ) : null}
            </p>
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          {data.can_rerun ? (
            <Button type="button" variant="outline" size="sm" disabled={busy} onClick={() => void act('rerun')}>
              <RotateCw className="size-3.5" />
              Re-run all jobs
            </Button>
          ) : null}
          {data.can_cancel ? (
            <Button type="button" variant="outline" size="sm" disabled={busy} onClick={() => void act('cancel')}>
              Cancel
            </Button>
          ) : null}
        </div>
      </div>

      {waiting ? (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-[18px] border border-[var(--warning,var(--primary))]/40 bg-[var(--warning,var(--primary))]/10 px-4 py-3">
          <p className="text-sm">
            This workflow is <strong>awaiting approval</strong>
            {data.merge_request_number ? (
              <>
                {' '}
                from a maintainer in{' '}
                <Link to={xgitPath(`${slug}/mrs/${data.merge_request_number}`)} className="text-primary hover:underline">
                  #{data.merge_request_number}
                </Link>
                .
              </>
            ) : (
              ' from a maintainer.'
            )}
          </p>
          {data.can_approve ? (
            <Button type="button" className="btn-glow" disabled={busy} onClick={() => void act('approve')}>
              Approve and run
            </Button>
          ) : (
            <span className="text-xs text-muted-foreground">Maintainer+ aprova</span>
          )}
        </div>
      ) : null}

      <dl className="grid gap-3 text-sm sm:grid-cols-3">
        <div className="watch-complication rounded-[18px] px-4 py-3">
          <dt className="hud-label text-muted-foreground/70">Status</dt>
          <dd className="mt-1 flex items-center gap-2">
            <CiJobStatusBadge status={data.status} />
          </dd>
        </div>
        <div className="watch-complication rounded-[18px] px-4 py-3">
          <dt className="hud-label text-muted-foreground/70">Total duration</dt>
          <dd className="mt-1">{data.duration_ms != null ? formatCompactDuration(data.duration_ms) : '—'}</dd>
        </div>
        <div className="watch-complication rounded-[18px] px-4 py-3">
          <dt className="hud-label text-muted-foreground/70">Artifacts</dt>
          <dd className="mt-1">
            {data.has_artifact ? (
              <Button type="button" variant="outline" size="sm" onClick={() => void api.downloadCiArtifact(slug, number)}>
                Download
              </Button>
            ) : (
              '—'
            )}
          </dd>
        </div>
      </dl>

      <div className="grid gap-6 lg:grid-cols-[200px_1fr]">
        <aside className="text-sm">
          <p className="hud-label px-2 text-muted-foreground/70">Run details</p>
          <p className="mt-1 rounded-md bg-muted/40 px-2 py-1.5">Summary</p>
          <p className="px-2 py-1.5 text-muted-foreground">All jobs</p>
        </aside>
        <div className="watch-complication flex flex-col gap-4 rounded-[18px] p-4">
          <p className="text-xs text-muted-foreground">
            <code className="font-mono">{data.workflow || 'ci'}</code>
            {' · '}
            <span className="font-mono">on: {data.event || 'push'}</span>
            {data.runner ? ` · ${data.runner}` : waiting || data.status === 'pending' ? ' · aguardando runner' : null}
          </p>
          <ul className="flex flex-col gap-2">
            {jobs.map((step) => (
              <li key={step.name} className="flex items-center gap-2 text-sm">
                <CiRunStatusIcon status={step.status} />
                <span className="font-mono text-xs">{step.name}</span>
              </li>
            ))}
          </ul>
          {data.error ? <p className="text-sm text-destructive">{data.error}</p> : null}
          <pre className="max-h-96 overflow-auto rounded-[14px] bg-background/50 p-4 font-mono text-xs leading-relaxed">
            {log ||
              (waiting
                ? 'Aguardando aprovação de um maintainer…'
                : data.status === 'pending'
                  ? 'Aguardando um peer role=runner na malha…'
                  : 'Sem log ainda.')}
          </pre>
        </div>
      </div>
    </div>
  )
}
