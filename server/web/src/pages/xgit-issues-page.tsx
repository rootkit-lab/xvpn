import { type FormEvent, useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { toast } from 'sonner'
import { openChat } from '@chat/messenger/open-chat'
import { api, ApiError, type IssueStatus, type ProjectRole } from '@/lib/api'
import { formatRelativeTime } from '@/lib/format'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { canWriteAdminProduct, isAdminRole } from '@/lib/roles'
import { xgitPath } from '@/lib/xgit'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { XgitTrackerNav, type TrackerView } from '@/pages/xgit-tracker-nav'

const ROLE_RANK: Record<ProjectRole, number> = {
  guest: 0,
  reporter: 1,
  developer: 2,
  maintainer: 3,
  owner: 4,
}

const ISSUE_VIEWS = new Set<TrackerView>(['issues', 'assigned', 'created', 'mentioned', 'recent'])

function issueQueryHint(view: TrackerView, status: IssueStatus | '') {
  const parts = ['is:issue']
  if (status) parts.push(`state:${status}`)
  if (view === 'assigned') parts.push('assignee:@me')
  if (view === 'created') parts.push('author:@me')
  if (view === 'mentioned') parts.push('mentions:@me')
  if (view === 'recent') parts.push('sort:updated')
  return parts.join(' ')
}

export function XgitIssuesPage() {
  const { slug = '' } = useParams()
  const { user } = useAuth()
  const [params, setParams] = useSearchParams()
  const viewRaw = params.get('view') ?? 'issues'
  const view: TrackerView = ISSUE_VIEWS.has(viewRaw as TrackerView) ? (viewRaw as TrackerView) : 'issues'
  const status = (params.get('status') as IssueStatus | null) ?? 'open'
  const q = params.get('q') ?? ''
  const author = params.get('author') ?? ''
  const assignee = params.get('assignee') ?? ''
  const label = params.get('label') ?? ''
  const milestone = params.get('milestone')
  const sort = (params.get('sort') as 'newest' | 'oldest' | 'updated' | null) ?? (view === 'recent' ? 'updated' : 'newest')

  const setFilter = (key: string, value: string) => {
    const next = new URLSearchParams(params)
    if (value) next.set(key, value)
    else next.delete(key)
    if (key === 'view' && value === 'issues') next.delete('view')
    setParams(next, { replace: true })
  }

  const fetchProject = useCallback(() => api.getProject(slug), [slug])
  const fetchLabels = useCallback(() => api.listIssueLabels(slug), [slug])
  const fetchMilestones = useCallback(() => api.listMilestones(slug, 'open'), [slug])
  const fetchIssues = useCallback(
    () =>
      api.listIssues(slug, {
        status: status || undefined,
        q: q.trim() || undefined,
        author: view === 'created' ? 'me' : author || undefined,
        assignee: view === 'assigned' ? 'me' : assignee || undefined,
        mentioned: view === 'mentioned' ? 'me' : undefined,
        label: label || undefined,
        milestone: milestone ? Number(milestone) : undefined,
        sort,
      }),
    [slug, status, q, view, author, assignee, label, milestone, sort],
  )
  const { data: project } = usePollingData(fetchProject, 30_000)
  const { data: labelData } = usePollingData(fetchLabels, 30_000)
  const { data: msData } = usePollingData(fetchMilestones, 30_000)
  const { data, loading, error } = usePollingData(fetchIssues, 15_000)
  const myRole = project?.members?.find((m) => m.user_id === user?.id)?.role
  const canCreate =
    (isAdminRole(user?.role) && canWriteAdminProduct(user?.role, user?.products, 'forge')) ||
    (myRole != null && ROLE_RANK[myRole] >= ROLE_RANK.reporter)
  const authors = useMemo(() => {
    const names = new Set((project?.members ?? []).map((m) => m.username))
    return [...names]
  }, [project])

  return (
    <div className="flex flex-col gap-6 lg:flex-row">
      <XgitTrackerNav slug={slug} active={view} />
      <div className="min-w-0 flex-1">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
          <h2 className="font-display text-xl font-semibold">All issues</h2>
          {canCreate ? (
            <Button asChild className="btn-glow">
              <Link to={xgitPath(`${slug}/issues/new`)}>New issue</Link>
            </Button>
          ) : null}
        </div>
        <Input
          className="field-glass mb-3 h-9 font-mono text-xs"
          value={q}
          onChange={(e) => setFilter('q', e.target.value)}
          placeholder={issueQueryHint(view, status)}
          aria-label="Filtrar issues"
        />
        <div className="watch-complication overflow-hidden rounded-[18px]">
          <div className="flex flex-wrap items-center gap-2 border-b border-border/60 px-3 py-2">
            <Button type="button" size="sm" variant={status === 'open' ? 'default' : 'ghost'} onClick={() => setFilter('status', 'open')}>
              Open {data?.open_count ?? 0}
            </Button>
            <Button type="button" size="sm" variant={status === 'closed' ? 'default' : 'ghost'} onClick={() => setFilter('status', 'closed')}>
              Closed {data?.closed_count ?? 0}
            </Button>
            <div className="ml-auto flex flex-wrap gap-1">
              <FilterMenu
                label="Author"
                value={author}
                items={authors.map((name) => ({ value: name, label: name }))}
                onChange={(v) => setFilter('author', v)}
              />
              <FilterMenu
                label="Labels"
                value={label}
                items={(labelData?.items ?? []).map((name) => ({ value: name, label: name }))}
                onChange={(v) => setFilter('label', v)}
              />
              <FilterMenu
                label="Milestones"
                value={milestone ?? ''}
                items={(msData?.items ?? []).map((m) => ({ value: String(m.number), label: m.title }))}
                onChange={(v) => setFilter('milestone', v)}
              />
              <FilterMenu
                label="Assignees"
                value={assignee}
                items={authors.map((name) => ({ value: name, label: name }))}
                onChange={(v) => setFilter('assignee', v)}
              />
              <FilterMenu
                label="Sort"
                value={sort}
                allowEmpty={false}
                items={[
                  { value: 'newest', label: 'Newest' },
                  { value: 'oldest', label: 'Oldest' },
                  { value: 'updated', label: 'Recently updated' },
                ]}
                onChange={(v) => setFilter('sort', v)}
              />
            </div>
          </div>
          {loading || !data ? (
            error ? <p className="p-5 text-sm text-destructive">{error}</p> : <p className="p-5 text-sm text-muted-foreground">Carregando…</p>
          ) : (data.items ?? []).length === 0 ? (
            <div className="px-4 py-16 text-center">
              <p className="text-sm font-medium">No results</p>
              <p className="mt-1 text-xs text-muted-foreground">Try adjusting your search filters</p>
            </div>
          ) : (
            <ul className="divide-y divide-border/60">
              {data.items.map((it) => (
                <li key={it.number}>
                  <Link
                    to={xgitPath(`${slug}/issues/${it.number}`)}
                    className="flex flex-wrap items-start justify-between gap-3 px-4 py-3 hover:bg-muted/20"
                  >
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">
                        {it.title} <span className="text-muted-foreground">#{it.number}</span>
                      </p>
                      <p className="mt-0.5 text-xs text-muted-foreground">
                        {it.author} · {formatRelativeTime(it.created_at)}
                        {it.assignees.length > 0 ? ` · ${it.assignees.join(', ')}` : ''}
                        {it.milestone_title ? ` · ${it.milestone_title}` : ''}
                      </p>
                      {it.labels.length > 0 ? (
                        <div className="mt-1 flex flex-wrap gap-1">
                          {it.labels.map((lb) => (
                            <Badge key={lb} variant="outline">
                              {lb}
                            </Badge>
                          ))}
                        </div>
                      ) : null}
                    </div>
                    <Badge variant={it.status === 'open' ? 'default' : 'outline'}>
                      {it.status === 'open' ? 'Open' : 'Closed'}
                    </Badge>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </div>
  )
}

function FilterMenu({
  label,
  value,
  items,
  onChange,
  allowEmpty = true,
}: {
  label: string
  value: string
  items: { value: string; label: string }[]
  onChange: (v: string) => void
  allowEmpty?: boolean
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button type="button" size="sm" variant="ghost" className="text-muted-foreground">
          {label}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {allowEmpty ? (
          <DropdownMenuItem onClick={() => onChange('')}>Qualquer</DropdownMenuItem>
        ) : null}
        {items.map((it) => (
          <DropdownMenuItem key={it.value} onClick={() => onChange(it.value)}>
            {it.value === value ? '✓ ' : ''}
            {it.label}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

export function XgitIssuePage() {
  const { slug = '', n = '' } = useParams()
  const number = Number(n)
  const fetchIssue = useCallback(() => api.getIssue(slug, number), [slug, number])
  const { data, loading, error, reload } = usePollingData(fetchIssue, 15_000)
  const [busy, setBusy] = useState(false)

  if (!Number.isFinite(number) || number < 1) {
    return <p className="text-sm text-destructive">Issue inválida.</p>
  }
  if (loading || !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-48 w-full" />
  }

  async function setStatus(status: IssueStatus) {
    setBusy(true)
    try {
      await api.patchIssue(slug, number, { status })
      toast.success(status === 'closed' ? 'Issue fechada' : 'Issue reaberta')
      reload()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha na ação')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_240px]">
      <div className="flex flex-col gap-4">
        <p className="text-sm text-muted-foreground">
          <Link to={xgitPath(`${slug}/issues`)} className="hover:underline">
            Issues
          </Link>
          <span className="px-1.5">/</span>
          <span className="text-foreground">#{data.number}</span>
        </p>
        <div className="flex flex-wrap items-center gap-2">
          <h2 className="font-display text-xl font-semibold">{data.title}</h2>
          <Badge variant={data.status === 'open' ? 'default' : 'outline'}>
            {data.status === 'open' ? 'Open' : 'Closed'}
          </Badge>
        </div>
        <p className="text-sm text-muted-foreground">
          {data.author} abriu {formatRelativeTime(data.created_at)}
          {data.closed_by ? ` · fechada por ${data.closed_by}` : ''}
        </p>
        <div className="watch-complication rounded-[18px] p-5">
          {data.body ? (
            <pre className="whitespace-pre-wrap font-sans text-sm leading-relaxed">{data.body}</pre>
          ) : (
            <p className="text-sm text-muted-foreground">Sem descrição.</p>
          )}
        </div>
        <div className="flex flex-wrap gap-2">
          <Button type="button" onClick={() => openChat({ dmId: data.thread_id, title: `#${data.number} ${data.title}` })}>
            Discutir no XCHAT
          </Button>
          {data.can_close ? (
            <Button type="button" variant="outline" disabled={busy} onClick={() => void setStatus('closed')}>
              Fechar
            </Button>
          ) : null}
          {data.can_reopen ? (
            <Button type="button" variant="outline" disabled={busy} onClick={() => void setStatus('open')}>
              Reabrir
            </Button>
          ) : null}
        </div>
      </div>
      <aside className="flex flex-col gap-4 text-sm">
        <div>
          <p className="hud-label mb-2 text-muted-foreground/70">Labels</p>
          {data.labels.length === 0 ? (
            <p className="text-muted-foreground">Nenhuma</p>
          ) : (
            <div className="flex flex-wrap gap-1">
              {data.labels.map((lb) => (
                <Badge key={lb} variant="outline">
                  {lb}
                </Badge>
              ))}
            </div>
          )}
        </div>
        <div>
          <p className="hud-label mb-2 text-muted-foreground/70">Assignees</p>
          {data.assignees.length === 0 ? (
            <p className="text-muted-foreground">Ninguém</p>
          ) : (
            <ul className="flex flex-col gap-1">
              {data.assignees.map((name) => (
                <li key={name}>{name}</li>
              ))}
            </ul>
          )}
        </div>
        <div>
          <p className="hud-label mb-2 text-muted-foreground/70">Milestone</p>
          {data.milestone_title ? (
            <p>{data.milestone_title}</p>
          ) : (
            <p className="text-muted-foreground">Nenhum</p>
          )}
        </div>
      </aside>
    </div>
  )
}

export function XgitPullsPage() {
  const { slug = '' } = useParams()
  const { user } = useAuth()
  const [status, setStatus] = useState<'open' | 'closed' | 'merged' | ''>('open')
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [source, setSource] = useState('')
  const [target, setTarget] = useState('main')
  const [busy, setBusy] = useState(false)
  const fetchProject = useCallback(() => api.getProject(slug), [slug])
  const fetchMRs = useCallback(
    () => api.listMergeRequests(slug, status || undefined),
    [slug, status],
  )
  const fetchBranches = useCallback(() => api.listProjectBranches(slug), [slug])
  const { data: project } = usePollingData(fetchProject, 30_000)
  const { data, loading, error, reload } = usePollingData(fetchMRs, 15_000)
  const { data: branchData } = usePollingData(fetchBranches, 30_000)
  const branches = branchData?.items ?? []
  const myRole = project?.members?.find((m) => m.user_id === user?.id)?.role
  const canCreate =
    (isAdminRole(user?.role) && canWriteAdminProduct(user?.role, user?.products, 'forge')) ||
    (myRole != null && ROLE_RANK[myRole] >= ROLE_RANK.developer)

  const heads = branches
  useEffect(() => {
    if (!heads.length) return
    setSource((s) => s || heads.find((b) => b !== 'main' && b !== 'master') || heads[0] || '')
    setTarget((t) => (t && heads.includes(t) ? t : heads.includes('main') ? 'main' : heads[0] ?? ''))
  }, [heads])

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      const mr = await api.createMergeRequest(slug, {
        title: title.trim(),
        description: description.trim() || undefined,
        source_branch: source,
        target_branch: target,
      })
      toast.success(`PR #${mr.number} aberto`)
      setTitle('')
      setDescription('')
      reload()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao abrir PR')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap gap-1">
        {(['open', 'closed', 'merged', ''] as const).map((st) => (
          <Button
            key={st || 'all'}
            type="button"
            size="sm"
            variant={status === st ? 'default' : 'ghost'}
            onClick={() => setStatus(st)}
          >
            {st === '' ? 'All' : st === 'open' ? 'Open' : st === 'closed' ? 'Closed' : 'Merged'}
          </Button>
        ))}
      </div>
      <div className="watch-complication overflow-hidden rounded-[18px]">
        {loading || !data ? (
          error ? <p className="p-5 text-sm text-destructive">{error}</p> : <p className="p-5 text-sm text-muted-foreground">Carregando…</p>
        ) : (data.items ?? []).length === 0 ? (
          <p className="p-5 text-sm text-muted-foreground">Nenhum pull request neste filtro.</p>
        ) : (
          <ul className="divide-y divide-border/60">
            {data.items.map((mr) => (
              <li key={mr.number}>
                <Link
                  to={xgitPath(`${slug}/pulls/${mr.number}`)}
                  className="flex flex-wrap items-start justify-between gap-3 px-4 py-3 hover:bg-muted/20"
                >
                  <div className="min-w-0">
                    <p className="truncate text-sm font-medium">
                      {mr.title} <span className="text-muted-foreground">#{mr.number}</span>
                    </p>
                    <p className="mt-0.5 font-mono text-xs text-muted-foreground">
                      {mr.source_branch} → {mr.target_branch} · {mr.author} · {formatRelativeTime(mr.created_at)}
                    </p>
                  </div>
                  <Badge variant={mr.status === 'merged' ? 'secondary' : mr.status === 'closed' ? 'outline' : 'default'}>
                    {mr.status === 'open' ? 'Open' : mr.status === 'merged' ? 'Merged' : 'Closed'}
                  </Badge>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </div>
      {canCreate ? (
        heads.length < 2 ? (
          <p className="text-sm text-muted-foreground">Faça push de pelo menos duas branches para abrir um PR.</p>
        ) : (
          <form onSubmit={submit} className="watch-complication grid gap-3 rounded-[18px] p-4 sm:grid-cols-2">
            <p className="hud-label sm:col-span-2 text-muted-foreground/70">Novo pull request</p>
            <div className="space-y-1.5 sm:col-span-2">
              <Label htmlFor="pr-title">Título</Label>
              <Input id="pr-title" value={title} onChange={(e) => setTitle(e.target.value)} required maxLength={120} />
            </div>
            <div className="space-y-1.5 sm:col-span-2">
              <Label htmlFor="pr-desc">Descrição</Label>
              <Textarea id="pr-desc" value={description} onChange={(e) => setDescription(e.target.value)} rows={3} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="pr-src">Source</Label>
              <select
                id="pr-src"
                className="field-glass h-9 w-full rounded-md px-2 text-sm"
                value={source}
                onChange={(e) => setSource(e.target.value)}
              >
                {heads.map((b) => (
                  <option key={b} value={b}>
                    {b}
                  </option>
                ))}
              </select>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="pr-dst">Target</Label>
              <select
                id="pr-dst"
                className="field-glass h-9 w-full rounded-md px-2 text-sm"
                value={target}
                onChange={(e) => setTarget(e.target.value)}
              >
                {heads.map((b) => (
                  <option key={b} value={b}>
                    {b}
                  </option>
                ))}
              </select>
            </div>
            <div className="sm:col-span-2">
              <Button type="submit" disabled={busy || !source || source === target}>
                {busy ? 'Abrindo…' : 'Abrir pull request'}
              </Button>
            </div>
          </form>
        )
      ) : null}
    </div>
  )
}

export { XgitPullsPage as XgitMrsPage }

