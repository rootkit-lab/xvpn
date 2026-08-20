import { type DragEvent, type FormEvent, useCallback, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import { FolderKanban, LayoutGrid, Table2, Bug, Map } from 'lucide-react'
import { api, ApiError, type ProjectRole, type WorkItem, type WorkProjectTemplate } from '@/lib/api'
import { formatRelativeTime } from '@/lib/format'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { canWriteAdminProduct, isAdminRole } from '@/lib/roles'
import { xgitPath } from '@/lib/xgit'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { XgitTrackerNav } from '@/pages/xgit-tracker-nav'
import { cn } from '@/lib/utils'

const ROLE_RANK: Record<ProjectRole, number> = {
  guest: 0,
  reporter: 1,
  developer: 2,
  maintainer: 3,
  owner: 4,
}

const TEMPLATES: { id: WorkProjectTemplate; title: string; blurb: string; icon: typeof Table2 }[] = [
  { id: 'table', title: 'Table', blurb: 'Planilha com colunas Todo / In Progress / Done.', icon: Table2 },
  { id: 'kanban', title: 'Kanban', blurb: 'Board para visualizar status e limitar WIP.', icon: LayoutGrid },
  { id: 'bug', title: 'Bug tracker', blurb: 'Triage → In Progress → Done.', icon: Bug },
  { id: 'roadmap', title: 'Roadmap', blurb: 'Tabela para planejamento (sem barras de data nesta fase).', icon: Map },
]

function canWriteForge(user: ReturnType<typeof useAuth>['user'], role?: ProjectRole) {
  return (
    (isAdminRole(user?.role) && canWriteAdminProduct(user?.role, user?.products, 'forge')) ||
    (role != null && ROLE_RANK[role] >= ROLE_RANK.reporter)
  )
}

export function XgitProjectsPage() {
  const { org = '', slug = '' } = useParams()
  const navigate = useNavigate()
  const { user } = useAuth()
  const [status, setStatus] = useState<'open' | 'closed'>('open')
  const [q, setQ] = useState('')
  const [open, setOpen] = useState(false)
  const [title, setTitle] = useState('')
  const [template, setTemplate] = useState<WorkProjectTemplate>('kanban')
  const [busy, setBusy] = useState(false)
  const fetchProject = useCallback(() => api.getProject(`${org}/${slug}`), [slug])
  const fetchBoards = useCallback(() => api.listWorkProjects(`${org}/${slug}`, { status, q: q.trim() || undefined }), [slug, status, q])
  const { data: project } = usePollingData(fetchProject, 30_000)
  const { data, loading, error } = usePollingData(fetchBoards, 15_000)
  const myRole = project?.members?.find((m) => m.user_id === user?.id)?.role
  const canCreate = canWriteForge(user, myRole)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      const wp = await api.createWorkProject(`${org}/${slug}`, { title: title.trim(), template })
      toast.success(`Project #${wp.number} criado`)
      setOpen(false)
      setTitle('')
      navigate(xgitPath(`${org}/${slug}/projects/${wp.number}`))
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao criar project')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-col gap-6 lg:flex-row">
      <XgitTrackerNav org={org} slug={slug} active="projects" />
      <div className="min-w-0 flex-1">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
          <h2 className="font-display text-xl font-semibold">Projects</h2>
          {canCreate ? (
            <Button type="button" className="btn-glow" onClick={() => setOpen(true)}>
              New project
            </Button>
          ) : null}
        </div>
        <Input
          className="field-glass mb-3 h-9"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder="Search by name..."
          aria-label="Buscar projects"
        />
        <div className="watch-complication overflow-hidden rounded-[18px]">
          <div className="flex items-center gap-2 border-b border-border/60 px-3 py-2">
            <Button type="button" size="sm" variant={status === 'open' ? 'default' : 'ghost'} onClick={() => setStatus('open')}>
              Open
            </Button>
            <Button type="button" size="sm" variant={status === 'closed' ? 'default' : 'ghost'} onClick={() => setStatus('closed')}>
              Closed
            </Button>
          </div>
          {loading || !data ? (
            error ? <p className="p-5 text-sm text-destructive">{error}</p> : <p className="p-5 text-sm text-muted-foreground">Carregando…</p>
          ) : (data.items ?? []).length === 0 ? (
            <div className="flex flex-col items-center gap-3 px-4 py-16 text-center">
              <FolderKanban className="size-10 text-muted-foreground/50" />
              <p className="text-sm font-medium">No projects found</p>
              <p className="text-xs text-muted-foreground">There are no projects linked to this repository yet.</p>
              {canCreate ? (
                <Button type="button" className="btn-glow" onClick={() => setOpen(true)}>
                  New project
                </Button>
              ) : null}
            </div>
          ) : (
            <ul className="divide-y divide-border/60">
              {data.items.map((wp) => (
                <li key={wp.number}>
                  <Link
                    to={xgitPath(`${org}/${slug}/projects/${wp.number}`)}
                    className="flex flex-wrap items-start justify-between gap-3 px-4 py-3 hover:bg-muted/20"
                  >
                    <div>
                      <p className="text-sm font-medium">{wp.title}</p>
                      <p className="text-xs text-muted-foreground">
                        {wp.layout} · {wp.item_count} items · {wp.author} · {formatRelativeTime(wp.created_at)}
                      </p>
                    </div>
                    <Badge variant={wp.status === 'open' ? 'default' : 'outline'}>{wp.status}</Badge>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>Create project</DialogTitle>
            <DialogDescription>Escolha um template. O board fica ligado a este repositório.</DialogDescription>
          </DialogHeader>
          <form onSubmit={submit} className="flex flex-col gap-4">
            <div className="space-y-1.5">
              <Label htmlFor="wp-title">Title</Label>
              <Input id="wp-title" value={title} onChange={(e) => setTitle(e.target.value)} required maxLength={120} />
            </div>
            <div className="grid gap-2 sm:grid-cols-2">
              {TEMPLATES.map((tpl) => {
                const Icon = tpl.icon
                const on = template === tpl.id
                return (
                  <button
                    key={tpl.id}
                    type="button"
                    onClick={() => setTemplate(tpl.id)}
                    className={cn(
                      'flex gap-3 rounded-[14px] border px-3 py-3 text-left',
                      on ? 'border-primary bg-muted/40' : 'border-border/60 hover:bg-muted/20',
                    )}
                  >
                    <Icon className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
                    <span>
                      <span className="block text-sm font-medium">{tpl.title}</span>
                      <span className="block text-xs text-muted-foreground">{tpl.blurb}</span>
                    </span>
                  </button>
                )
              })}
            </div>
            <DialogFooter>
              <Button type="button" variant="ghost" onClick={() => setOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" className="btn-glow" disabled={busy || !title.trim()}>
                {busy ? 'Creating…' : 'Create'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}

export function XgitProjectBoardPage() {
  const { org = '', slug = '', n = '' } = useParams()
  const number = Number(n)
  const { user } = useAuth()
  const [draft, setDraft] = useState('')
  const [issueN, setIssueN] = useState('')
  const [busy, setBusy] = useState(false)
  const fetchProject = useCallback(() => api.getProject(`${org}/${slug}`), [slug])
  const fetchBoard = useCallback(() => api.getWorkProject(`${org}/${slug}`, number), [slug, number])
  const fetchIssues = useCallback(() => api.listIssues(`${org}/${slug}`, { status: 'open' }), [slug])
  const { data: project } = usePollingData(fetchProject, 30_000)
  const { data, loading, error, reload } = usePollingData(fetchBoard, 10_000)
  const { data: issues } = usePollingData(fetchIssues, 20_000)
  const myRole = project?.members?.find((m) => m.user_id === user?.id)?.role
  const canCreate = canWriteForge(user, myRole)

  if (!Number.isFinite(number) || number < 1) {
    return <p className="text-sm text-destructive">Project inválido.</p>
  }
  if (loading || !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-48 w-full" />
  }
  const items = data.items ?? []

  async function addDraft(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      await api.createWorkItem(`${org}/${slug}`, number, { title: draft.trim() })
      setDraft('')
      reload()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao adicionar')
    } finally {
      setBusy(false)
    }
  }

  async function addIssue(e: FormEvent) {
    e.preventDefault()
    const nIssue = Number(issueN)
    if (!nIssue) return
    setBusy(true)
    try {
      await api.createWorkItem(`${org}/${slug}`, number, { issue: nIssue })
      setIssueN('')
      reload()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao vincular issue')
    } finally {
      setBusy(false)
    }
  }

  async function move(item: WorkItem, column: string) {
    try {
      await api.patchWorkItem(`${org}/${slug}`, number, item.id, { column })
      reload()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao mover')
    }
  }

  async function onDrop(column: string, ev: DragEvent) {
    ev.preventDefault()
    const id = Number(ev.dataTransfer.getData('text/plain'))
    const item = items.find((it) => it.id === id)
    if (!item || item.column === column) return
    await move(item, column)
  }

  return (
    <div className="flex flex-col gap-4">
      <p className="text-sm text-muted-foreground">
        <Link to={xgitPath(`${org}/${slug}/projects`)} className="hover:underline">
          Projects
        </Link>
        <span className="px-1.5">/</span>
        <span className="text-foreground">{data.title}</span>
      </p>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <h2 className="font-display text-xl font-semibold">{data.title}</h2>
          <Badge variant={data.status === 'open' ? 'default' : 'outline'}>{data.status}</Badge>
          <span className="text-xs text-muted-foreground">{data.layout}</span>
        </div>
        {data.can_update ? (
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => {
              void api
                .patchWorkProject(slug, number, { status: data.status === 'open' ? 'closed' : 'open' })
                .then(() => reload())
                .catch((err) => toast.error(err instanceof ApiError ? err.message : 'Falha'))
            }}
          >
            {data.status === 'open' ? 'Close project' : 'Reopen'}
          </Button>
        ) : null}
      </div>
      {data.description ? <p className="text-sm text-muted-foreground">{data.description}</p> : null}

      {canCreate && data.status === 'open' ? (
        <div className="flex flex-wrap gap-3">
          <form onSubmit={addDraft} className="flex min-w-[220px] flex-1 gap-2">
            <Input className="field-glass h-9" value={draft} onChange={(e) => setDraft(e.target.value)} placeholder="Draft item" />
            <Button type="submit" size="sm" disabled={busy || !draft.trim()}>
              Add
            </Button>
          </form>
          <form onSubmit={addIssue} className="flex min-w-[220px] flex-1 gap-2">
            <select
              className="field-glass h-9 flex-1 rounded-md px-2 text-sm"
              value={issueN}
              onChange={(e) => setIssueN(e.target.value)}
            >
              <option value="">Link issue…</option>
              {(issues?.items ?? []).map((it) => (
                <option key={it.number} value={it.number}>
                  #{it.number} {it.title}
                </option>
              ))}
            </select>
            <Button type="submit" size="sm" variant="outline" disabled={busy || !issueN}>
              Link
            </Button>
          </form>
        </div>
      ) : null}

      {data.layout === 'table' ? (
        <div className="watch-complication overflow-hidden rounded-[18px]">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border/60 text-left text-xs text-muted-foreground">
                <th className="px-4 py-2">Title</th>
                <th className="px-4 py-2">Status</th>
              </tr>
            </thead>
            <tbody>
              {items.map((it) => (
                <tr key={it.id} className="border-b border-border/40">
                  <td className="px-4 py-2">
                    <ItemTitle org={org} slug={slug} item={it} />
                  </td>
                  <td className="px-4 py-2">
                    {canCreate ? (
                      <select
                        className="field-glass h-8 rounded-md px-2 text-xs"
                        value={it.column}
                        onChange={(e) => void move(it, e.target.value)}
                      >
                        {data.columns.map((col) => (
                          <option key={col} value={col}>
                            {col}
                          </option>
                        ))}
                      </select>
                    ) : (
                      it.column
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <div className="grid gap-3 md:grid-cols-3">
          {data.columns.map((col) => {
            const cards = items.filter((it) => it.column === col)
            return (
              <div
                key={col}
                className="watch-complication flex min-h-64 flex-col rounded-[18px] p-3"
                onDragOver={(e) => e.preventDefault()}
                onDrop={(e) => void onDrop(col, e)}
              >
                <p className="hud-label mb-2 text-muted-foreground/70">
                  {col} · {cards.length}
                </p>
                <ul className="flex flex-col gap-2">
                  {cards.map((it) => (
                    <li
                      key={it.id}
                      draggable={canCreate}
                      onDragStart={(e) => e.dataTransfer.setData('text/plain', String(it.id))}
                      className="rounded-xl border border-border/60 bg-background/40 px-3 py-2"
                    >
                      <ItemTitle org={org} slug={slug} item={it} />
                      {canCreate ? (
                        <div className="mt-2 flex flex-wrap gap-1">
                          {data.columns
                            .filter((c) => c !== it.column)
                            .map((c) => (
                              <button
                                key={c}
                                type="button"
                                className="text-[10px] text-muted-foreground hover:text-foreground"
                                onClick={() => void move(it, c)}
                              >
                                → {c}
                              </button>
                            ))}
                        </div>
                      ) : null}
                    </li>
                  ))}
                </ul>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}

function ItemTitle({ org, slug, item }: { org: string; slug: string; item: WorkItem }) {
  if (item.issue) {
    return (
      <Link to={xgitPath(`${org}/${slug}/issues/${item.issue}`)} className="text-sm hover:underline">
        #{item.issue} {item.title}
      </Link>
    )
  }
  if (item.mr) {
    return (
      <Link to={xgitPath(`${org}/${slug}/pulls/${item.mr}`)} className="text-sm hover:underline">
        PR #{item.mr} {item.title}
      </Link>
    )
  }
  return <p className="text-sm">{item.title}</p>
}
