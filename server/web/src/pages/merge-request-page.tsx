import { useCallback, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import { openChat } from '@chat/messenger/open-chat'
import { api, ApiError, type MergeRequestStatus, type MRReviewState } from '@/lib/api'
import { formatRelativeTime } from '@/lib/format'
import { usePollingData } from '@/hooks/use-polling-data'
import { xgitPath, xgitReposPath } from '@/lib/xgit'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Textarea } from '@/components/ui/textarea'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import { CiRunStatusIcon } from '@/pages/ci-job-page'

const STATUS_LABEL: Record<MergeRequestStatus, string> = {
  open: 'Open',
  merged: 'Merged',
  closed: 'Closed',
}

type Tab = 'conversation' | 'commits' | 'files'

export function MergeRequestPage() {
  const { slug = '', iid = '' } = useParams()
  const number = Number(iid)
  const [tab, setTab] = useState<Tab>('conversation')
  const [busy, setBusy] = useState(false)
  const [reviewBody, setReviewBody] = useState('')
  const [editing, setEditing] = useState(false)
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const fetchMR = useCallback(() => api.getMergeRequest(slug, number), [slug, number])
  const fetchCommits = useCallback(() => api.listMRCommits(slug, number), [slug, number])
  const fetchDiff = useCallback(() => api.getMRDiff(slug, number), [slug, number])
  const fetchReviews = useCallback(() => api.listMRReviews(slug, number), [slug, number])
  const fetchJobs = useCallback(() => api.listCiJobs(slug, undefined, number), [slug, number])
  const { data, loading, error, reload } = usePollingData(fetchMR, 15_000)
  const { data: commits } = usePollingData(fetchCommits, 20_000)
  const { data: diff } = usePollingData(fetchDiff, 20_000)
  const { data: reviews, reload: reloadReviews } = usePollingData(fetchReviews, 15_000)
  const { data: jobs } = usePollingData(fetchJobs, 8_000)

  if (!Number.isFinite(number) || number < 1) {
    return <p className="text-sm text-destructive">PR inválido.</p>
  }
  if (loading || !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-48 w-full" />
  }

  async function act(kind: 'merge' | 'close') {
    setBusy(true)
    try {
      if (kind === 'merge') await api.mergeMergeRequest(slug, number)
      else await api.closeMergeRequest(slug, number)
      toast.success(kind === 'merge' ? 'Merge concluído' : 'PR fechado')
      reload()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha na ação')
    } finally {
      setBusy(false)
    }
  }

  async function saveMeta() {
    setBusy(true)
    try {
      await api.patchMergeRequest(slug, number, { title: title.trim(), description: description.trim() })
      toast.success('PR atualizado')
      setEditing(false)
      reload()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao editar')
    } finally {
      setBusy(false)
    }
  }

  async function review(state: MRReviewState) {
    setBusy(true)
    try {
      await api.createMRReview(slug, number, { state, body: reviewBody.trim() || undefined })
      toast.success('Review enviado')
      setReviewBody('')
      reloadReviews()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha no review')
    } finally {
      setBusy(false)
    }
  }

  const latestJob = jobs?.items?.[0]
  const mergeBlocked = Boolean(data.checks_block)

  return (
    <div className="flex flex-col gap-5">
      <p className="text-sm text-muted-foreground">
        <Link to={xgitReposPath()} className="hover:underline">
          XGIT
        </Link>
        <span className="px-1.5">/</span>
        <Link to={xgitPath(slug)} className="hover:underline">
          {slug}
        </Link>
        <span className="px-1.5">/</span>
        <Link to={xgitPath(`${slug}/pulls`)} className="hover:underline">
          Pull requests
        </Link>
        <span className="px-1.5">/</span>
        <span className="text-foreground">#{data.number}</span>
      </p>

      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          {editing ? (
            <div className="flex flex-col gap-2">
              <Input value={title} onChange={(e) => setTitle(e.target.value)} maxLength={120} />
              <Textarea value={description} onChange={(e) => setDescription(e.target.value)} rows={3} />
              <div className="flex gap-2">
                <Button type="button" size="sm" disabled={busy} onClick={() => void saveMeta()}>
                  Salvar
                </Button>
                <Button type="button" size="sm" variant="ghost" onClick={() => setEditing(false)}>
                  Cancelar
                </Button>
              </div>
            </div>
          ) : (
            <>
              <div className="flex flex-wrap items-center gap-2">
                <h2 className="font-display text-xl font-semibold">
                  {data.title} <span className="text-muted-foreground">#{data.number}</span>
                </h2>
                <StatusBadge status={data.status} />
              </div>
              <p className="mt-1 text-sm text-muted-foreground">
                <code className="font-mono text-xs">{data.source_branch}</code>
                {' → '}
                <code className="font-mono text-xs">{data.target_branch}</code>
                {' · '}
                {data.author}
                {data.merged_by ? ` · merge por ${data.merged_by}` : null}
              </p>
            </>
          )}
        </div>
        <div className="flex flex-wrap gap-2">
          {data.can_edit && data.status === 'open' && !editing ? (
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => {
                setTitle(data.title)
                setDescription(data.description)
                setEditing(true)
              }}
            >
              Editar
            </Button>
          ) : null}
          {data.status === 'open' ? (
            <>
              <Button type="button" disabled={busy || mergeBlocked} onClick={() => void act('merge')}>
                {mergeBlocked ? data.checks_block : busy ? '…' : 'Mergear'}
              </Button>
              <Button type="button" variant="outline" disabled={busy} onClick={() => void act('close')}>
                Fechar
              </Button>
            </>
          ) : null}
        </div>
      </div>

      {latestJob ? (
        <div className="watch-complication flex flex-wrap items-center gap-3 rounded-[18px] px-4 py-3 text-sm">
          <CiRunStatusIcon status={latestJob.status} />
          <span>
            Checks: <span className="font-medium">{latestJob.status}</span>
          </span>
          <Link to={xgitPath(`${slug}/actions/${latestJob.number}`)} className="text-primary hover:underline">
            ver run
          </Link>
          {data.checks_block ? <span className="text-destructive">{data.checks_block}</span> : null}
        </div>
      ) : null}

      <nav className="flex flex-wrap gap-1 border-b border-border/60">
        {(
          [
            ['conversation', 'Conversation'],
            ['commits', `Commits${commits?.items?.length ? ` (${commits.items.length})` : ''}`],
            ['files', 'Files changed'],
          ] as const
        ).map(([id, label]) => (
          <button
            key={id}
            type="button"
            className={cn(
              'border-b-2 px-3 py-2 text-sm',
              tab === id ? 'border-primary text-foreground' : 'border-transparent text-muted-foreground hover:text-foreground',
            )}
            onClick={() => setTab(id)}
          >
            {label}
          </button>
        ))}
      </nav>

      {tab === 'conversation' ? (
        <div className="flex flex-col gap-4">
          {data.description ? (
            <div className="watch-complication rounded-[18px] p-5">
              <pre className="whitespace-pre-wrap font-sans text-sm leading-relaxed">{data.description}</pre>
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">Sem descrição.</p>
          )}
          <ul className="flex flex-col gap-2">
            {(reviews?.items ?? []).map((r) => (
              <li key={r.id} className="watch-complication rounded-[18px] px-4 py-3 text-sm">
                <p className="font-medium">
                  {r.author}{' '}
                  <span className="font-normal text-muted-foreground">
                    {r.state === 'approve' ? 'aprovou' : r.state === 'request_changes' ? 'pediu alterações' : 'comentou'}
                    {' · '}
                    {formatRelativeTime(r.created_at)}
                  </span>
                </p>
                {r.body ? <p className="mt-1 text-muted-foreground">{r.body}</p> : null}
              </li>
            ))}
          </ul>
          {data.status === 'open' ? (
            <div className="watch-complication flex flex-col gap-3 rounded-[18px] p-4">
              <Textarea
                value={reviewBody}
                onChange={(e) => setReviewBody(e.target.value)}
                rows={3}
                placeholder="Comentário do review"
              />
              <div className="flex flex-wrap gap-2">
                <Button type="button" disabled={busy} onClick={() => void review('comment')}>
                  Comment
                </Button>
                <Button type="button" variant="outline" disabled={busy} onClick={() => void review('approve')}>
                  Approve
                </Button>
                <Button type="button" variant="outline" disabled={busy} onClick={() => void review('request_changes')}>
                  Request changes
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() => openChat({ dmId: data.thread_id, title: `#${data.number} ${data.title}` })}
                >
                  Abrir no XCHAT
                </Button>
              </div>
            </div>
          ) : (
            <Button
              type="button"
              onClick={() => openChat({ dmId: data.thread_id, title: `#${data.number} ${data.title}` })}
            >
              Abrir no XCHAT
            </Button>
          )}
        </div>
      ) : null}

      {tab === 'commits' ? (
        <div className="watch-complication overflow-hidden rounded-[18px]">
          {(commits?.items ?? []).length === 0 ? (
            <p className="p-5 text-sm text-muted-foreground">Nenhum commit neste PR.</p>
          ) : (
            <ul className="divide-y divide-border/60">
              {commits?.items.map((c) => (
                <li key={c.sha} className="flex flex-wrap items-start justify-between gap-3 px-4 py-3">
                  <div className="min-w-0">
                    <p className="truncate text-sm font-medium">{c.subject}</p>
                    <p className="mt-0.5 text-xs text-muted-foreground">
                      {c.author} · {formatRelativeTime(c.date)}
                    </p>
                  </div>
                  <code className="font-mono text-xs text-muted-foreground">{c.sha.slice(0, 7)}</code>
                </li>
              ))}
            </ul>
          )}
        </div>
      ) : null}

      {tab === 'files' ? (
        <div className="flex flex-col gap-3">
          <div className="flex justify-end">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => openChat({ dmId: data.thread_id, title: `#${data.number} files` })}
            >
              Comentar no XCHAT
            </Button>
          </div>
          <pre className="watch-complication max-h-[32rem] overflow-auto rounded-[18px] p-4 font-mono text-[11px] leading-relaxed">
            {diff?.diff || 'Sem diff.'}
          </pre>
        </div>
      ) : null}
    </div>
  )
}

export function StatusBadge({ status }: { status: MergeRequestStatus }) {
  const variant = status === 'merged' ? 'secondary' : status === 'closed' ? 'outline' : 'default'
  return <Badge variant={variant}>{STATUS_LABEL[status]}</Badge>
}
