import { type FormEvent, useCallback, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import { api, ApiError, type ProjectRole } from '@/lib/api'
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
import { XgitTrackerNav } from '@/pages/xgit-tracker-nav'

const ROLE_RANK: Record<ProjectRole, number> = {
  guest: 0,
  reporter: 1,
  developer: 2,
  maintainer: 3,
  owner: 4,
}

export function XgitMilestonesPage() {
  const { slug = '' } = useParams()
  const { user } = useAuth()
  const [status, setStatus] = useState<'open' | 'closed'>('open')
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [dueOn, setDueOn] = useState('')
  const [busy, setBusy] = useState(false)
  const fetchProject = useCallback(() => api.getProject(slug), [slug])
  const fetchMs = useCallback(() => api.listMilestones(slug, status), [slug, status])
  const { data: project } = usePollingData(fetchProject, 30_000)
  const { data, loading, error, reload } = usePollingData(fetchMs, 15_000)
  const myRole = project?.members?.find((m) => m.user_id === user?.id)?.role
  const canCreate =
    (isAdminRole(user?.role) && canWriteAdminProduct(user?.role, user?.products, 'forge')) ||
    (myRole != null && ROLE_RANK[myRole] >= ROLE_RANK.reporter)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      await api.createMilestone(slug, {
        title: title.trim(),
        description: description.trim() || undefined,
        due_on: dueOn || undefined,
      })
      toast.success('Milestone criado')
      setTitle('')
      setDescription('')
      setDueOn('')
      reload()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao criar milestone')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-col gap-6 lg:flex-row">
      <XgitTrackerNav slug={slug} active="milestones" />
      <div className="min-w-0 flex-1">
        <h2 className="mb-4 font-display text-xl font-semibold">Milestones</h2>
        <div className="mb-3 flex gap-1">
          <Button type="button" size="sm" variant={status === 'open' ? 'default' : 'ghost'} onClick={() => setStatus('open')}>
            Open
          </Button>
          <Button type="button" size="sm" variant={status === 'closed' ? 'default' : 'ghost'} onClick={() => setStatus('closed')}>
            Closed
          </Button>
        </div>
        <div className="watch-complication overflow-hidden rounded-[18px]">
          {loading || !data ? (
            error ? <p className="p-5 text-sm text-destructive">{error}</p> : <p className="p-5 text-sm text-muted-foreground">Carregando…</p>
          ) : (data.items ?? []).length === 0 ? (
            <p className="p-8 text-center text-sm text-muted-foreground">Nenhum milestone neste filtro.</p>
          ) : (
            <ul className="divide-y divide-border/60">
              {data.items.map((m) => (
                <li key={m.number} className="flex flex-wrap items-start justify-between gap-3 px-4 py-3">
                  <div>
                    <p className="text-sm font-medium">{m.title}</p>
                    <p className="text-xs text-muted-foreground">
                      {m.open_issues} open · {m.closed_issues} closed
                      {m.due_on ? ` · due ${m.due_on.slice(0, 10)}` : ''}
                      {' · '}
                      {formatRelativeTime(m.created_at)}
                    </p>
                    <Link to={xgitPath(`${slug}/issues?milestone=${m.number}`)} className="text-xs text-primary hover:underline">
                      Ver issues
                    </Link>
                  </div>
                  {m.can_update ? (
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      onClick={() => {
                        void api
                          .patchMilestone(slug, m.number, { status: m.status === 'open' ? 'closed' : 'open' })
                          .then(() => reload())
                          .catch((err) => toast.error(err instanceof ApiError ? err.message : 'Falha'))
                      }}
                    >
                      {m.status === 'open' ? 'Close' : 'Reopen'}
                    </Button>
                  ) : null}
                </li>
              ))}
            </ul>
          )}
        </div>
        {canCreate ? (
          <form onSubmit={submit} className="mt-4 watch-complication flex flex-col gap-3 rounded-[18px] p-4">
            <p className="hud-label text-muted-foreground/70">New milestone</p>
            <div className="space-y-1.5">
              <Label htmlFor="ms-title">Title</Label>
              <Input id="ms-title" value={title} onChange={(e) => setTitle(e.target.value)} required maxLength={120} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="ms-desc">Description</Label>
              <Textarea id="ms-desc" value={description} onChange={(e) => setDescription(e.target.value)} rows={3} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="ms-due">Due on</Label>
              <Input id="ms-due" type="date" value={dueOn} onChange={(e) => setDueOn(e.target.value)} />
            </div>
            <Button type="submit" disabled={busy || !title.trim()}>
              {busy ? 'Creating…' : 'Create milestone'}
            </Button>
          </form>
        ) : null}
      </div>
    </div>
  )
}

export function XgitLabelsPage() {
  const { slug = '' } = useParams()
  const fetchLabels = useCallback(() => api.listIssueLabels(slug), [slug])
  const { data, loading, error } = usePollingData(fetchLabels, 20_000)

  return (
    <div className="flex flex-col gap-6 lg:flex-row">
      <XgitTrackerNav slug={slug} active="labels" />
      <div className="min-w-0 flex-1">
        <h2 className="mb-4 font-display text-xl font-semibold">Labels</h2>
        <div className="watch-complication overflow-hidden rounded-[18px]">
          {loading || !data ? (
            error ? <p className="p-5 text-sm text-destructive">{error}</p> : <p className="p-5 text-sm text-muted-foreground">Carregando…</p>
          ) : (data.items ?? []).length === 0 ? (
            <p className="p-8 text-center text-sm text-muted-foreground">Nenhuma label ainda. Elas aparecem ao criar issues.</p>
          ) : (
            <ul className="divide-y divide-border/60">
              {data.items.map((lb) => (
                <li key={lb} className="flex items-center justify-between px-4 py-3">
                  <Badge variant="outline">{lb}</Badge>
                  <Link to={xgitPath(`${slug}/issues?label=${encodeURIComponent(lb)}`)} className="text-xs text-primary hover:underline">
                    Ver issues
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
