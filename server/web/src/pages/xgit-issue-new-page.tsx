import { type FormEvent, type ReactNode, useCallback, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import {
  Bold,
  Code,
  Heading,
  Italic,
  Link as LinkIcon,
  List,
  ListChecks,
  ListOrdered,
  Paperclip,
  Quote,
} from 'lucide-react'
import { api, ApiError, type ProjectRole } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { canWriteAdminProduct, isAdminRole } from '@/lib/roles'
import { xgitPath } from '@/lib/xgit'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import { MarkdownPreview } from '@/components/markdown-doc'

const ROLE_RANK: Record<ProjectRole, number> = {
  guest: 0,
  reporter: 1,
  developer: 2,
  maintainer: 3,
  owner: 4,
}

function insertAround(el: HTMLTextAreaElement, before: string, after = '', placeholder = 'texto') {
  const start = el.selectionStart
  const end = el.selectionEnd
  const selected = el.value.slice(start, end) || placeholder
  const next = el.value.slice(0, start) + before + selected + after + el.value.slice(end)
  const cursor = start + before.length + selected.length + after.length
  return { next, cursor }
}

export function XgitIssueNewPage() {
  const { org = '', slug = '' } = useParams()
  const navigate = useNavigate()
  const { user } = useAuth()
  const [title, setTitle] = useState('')
  const [body, setBody] = useState('')
  const [tab, setTab] = useState<'write' | 'preview'>('write')
  const [assignees, setAssignees] = useState<string[]>([])
  const [labels, setLabels] = useState<string[]>([])
  const [labelDraft, setLabelDraft] = useState('')
  const [milestone, setMilestone] = useState<number | 0>(0)
  const [busy, setBusy] = useState(false)
  const fetchProject = useCallback(() => api.getProject(`${org}/${slug}`), [slug])
  const fetchLabels = useCallback(() => api.listIssueLabels(`${org}/${slug}`), [slug])
  const fetchMilestones = useCallback(() => api.listMilestones(`${org}/${slug}`, 'open'), [slug])
  const { data: project } = usePollingData(fetchProject, 30_000)
  const { data: labelData } = usePollingData(fetchLabels, 30_000)
  const { data: msData } = usePollingData(fetchMilestones, 30_000)
  const myRole = project?.members?.find((m) => m.user_id === user?.id)?.role
  const canCreate =
    (isAdminRole(user?.role) && canWriteAdminProduct(user?.role, user?.products, 'forge')) ||
    (myRole != null && ROLE_RANK[myRole] >= ROLE_RANK.reporter)

  function applyWrap(before: string, after = '', placeholder?: string) {
    const el = document.getElementById('issue-body-editor') as HTMLTextAreaElement | null
    if (!el) {
      setBody((b) => b + before + (placeholder ?? '') + after)
      return
    }
    const { next, cursor } = insertAround(el, before, after, placeholder)
    setBody(next)
    requestAnimationFrame(() => {
      el.focus()
      el.setSelectionRange(cursor, cursor)
    })
  }

  function toggleAssignee(name: string) {
    setAssignees((cur) => (cur.includes(name) ? cur.filter((n) => n !== name) : [...cur, name]))
  }

  function addLabel(raw: string) {
    const s = raw.trim()
    if (!s || labels.includes(s)) return
    setLabels((cur) => [...cur, s])
    setLabelDraft('')
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (!canCreate) return
    setBusy(true)
    try {
      const issue = await api.createIssue(`${org}/${slug}`, {
        title: title.trim(),
        body: body.trim() || undefined,
        labels: labels.length ? labels : undefined,
        assignees: assignees.length ? assignees : undefined,
        milestone: milestone || undefined,
      })
      toast.success(`Issue #${issue.number} aberta`)
      navigate(xgitPath(`${org}/${slug}/issues/${issue.number}`))
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao abrir issue')
    } finally {
      setBusy(false)
    }
  }

  if (!canCreate) {
    return (
      <p className="text-sm text-muted-foreground">
        Sem permissão para criar issue.{' '}
        <Link to={xgitPath(`${org}/${slug}/issues`)} className="text-primary hover:underline">
          Voltar
        </Link>
      </p>
    )
  }

  return (
    <form onSubmit={submit} className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_240px]">
      <div className="flex flex-col gap-4">
        <div className="flex items-center gap-2">
          <h2 className="font-display text-xl font-semibold">Create new issue</h2>
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="new-issue-title">Add a title *</Label>
          <Input
            id="new-issue-title"
            className="field-glass"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="Title"
            required
            maxLength={120}
          />
        </div>
        <div className="space-y-2">
          <Label>Add a description</Label>
          <div className="watch-complication overflow-hidden rounded-[18px]">
            <div className="flex flex-wrap items-center gap-1 border-b border-border/60 px-2 py-1">
              <Button type="button" size="sm" variant={tab === 'write' ? 'default' : 'ghost'} onClick={() => setTab('write')}>
                Write
              </Button>
              <Button type="button" size="sm" variant={tab === 'preview' ? 'default' : 'ghost'} onClick={() => setTab('preview')}>
                Preview
              </Button>
              <div className="ml-auto flex flex-wrap gap-0.5">
                <IconBtn label="Heading" onClick={() => applyWrap('## ', '', 'Heading')}><Heading className="size-3.5" /></IconBtn>
                <IconBtn label="Bold" onClick={() => applyWrap('**', '**')}><Bold className="size-3.5" /></IconBtn>
                <IconBtn label="Italic" onClick={() => applyWrap('*', '*')}><Italic className="size-3.5" /></IconBtn>
                <IconBtn label="Quote" onClick={() => applyWrap('> ', '')}><Quote className="size-3.5" /></IconBtn>
                <IconBtn label="Code" onClick={() => applyWrap('`', '`')}><Code className="size-3.5" /></IconBtn>
                <IconBtn label="Link" onClick={() => applyWrap('[', '](url)', 'link')}><LinkIcon className="size-3.5" /></IconBtn>
                <IconBtn label="List" onClick={() => applyWrap('- ', '')}><List className="size-3.5" /></IconBtn>
                <IconBtn label="Numbered" onClick={() => applyWrap('1. ', '')}><ListOrdered className="size-3.5" /></IconBtn>
                <IconBtn label="Checklist" onClick={() => applyWrap('- [ ] ', '')}><ListChecks className="size-3.5" /></IconBtn>
              </div>
            </div>
            {tab === 'write' ? (
              <Textarea
                id="issue-body-editor"
                className="min-h-48 rounded-none border-0"
                value={body}
                onChange={(e) => setBody(e.target.value)}
                placeholder="Type your description here..."
                rows={10}
              />
            ) : (
              <div className="min-h-48 p-4">
                <MarkdownPreview text={body} />
              </div>
            )}
            <p className="flex items-center gap-2 border-t border-border/60 px-3 py-2 text-xs text-muted-foreground">
              <Paperclip className="size-3.5" />
              Markdown no corpo. Anexos ficam no XDRIVER do projeto.
            </p>
          </div>
        </div>
        <div className="flex flex-wrap items-center justify-end gap-2">
          <Button type="button" variant="ghost" onClick={() => navigate(xgitPath(`${org}/${slug}/issues`))}>
            Cancel
          </Button>
          <Button type="submit" className="btn-glow" disabled={busy || !title.trim()}>
            {busy ? 'Creating…' : 'Create'}
          </Button>
        </div>
      </div>
      <aside className="flex flex-col gap-5 text-sm">
        <div>
          <p className="hud-label mb-2 text-muted-foreground/70">Assignees</p>
          {(project?.members ?? []).length === 0 ? (
            <p className="text-muted-foreground">No one</p>
          ) : (
            <ul className="flex flex-col gap-1">
              {(project?.members ?? []).map((m) => (
                <li key={m.user_id}>
                  <label className="inline-flex items-center gap-2">
                    <input
                      type="checkbox"
                      checked={assignees.includes(m.username)}
                      onChange={() => toggleAssignee(m.username)}
                    />
                    {m.username}
                  </label>
                </li>
              ))}
            </ul>
          )}
          {user?.username && !assignees.includes(user.username) ? (
            <button type="button" className="mt-1 text-xs text-primary hover:underline" onClick={() => toggleAssignee(user.username)}>
              Assign yourself
            </button>
          ) : null}
        </div>
        <div>
          <p className="hud-label mb-2 text-muted-foreground/70">Labels</p>
          {labels.length === 0 ? <p className="mb-2 text-muted-foreground">No labels</p> : (
            <div className="mb-2 flex flex-wrap gap-1">
              {labels.map((lb) => (
                <Badge key={lb} variant="outline" className="cursor-pointer" onClick={() => setLabels((cur) => cur.filter((x) => x !== lb))}>
                  {lb} ×
                </Badge>
              ))}
            </div>
          )}
          <Input
            className="field-glass h-8"
            value={labelDraft}
            onChange={(e) => setLabelDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault()
                addLabel(labelDraft)
              }
            }}
            placeholder="Add label"
          />
          {(labelData?.items ?? []).length > 0 ? (
            <div className="mt-2 flex flex-wrap gap-1">
              {labelData?.items.map((lb) => (
                <button key={lb} type="button" className="text-xs text-muted-foreground hover:text-foreground" onClick={() => addLabel(lb)}>
                  {lb}
                </button>
              ))}
            </div>
          ) : null}
        </div>
        <div>
          <p className="hud-label mb-2 text-muted-foreground/70">Milestone</p>
          <select
            className="field-glass h-9 w-full rounded-md px-2 text-sm"
            value={milestone}
            onChange={(e) => setMilestone(Number(e.target.value))}
          >
            <option value={0}>No milestone</option>
            {(msData?.items ?? []).map((m) => (
              <option key={m.number} value={m.number}>
                {m.title}
              </option>
            ))}
          </select>
        </div>
      </aside>
    </form>
  )
}

function IconBtn({ label, onClick, children }: { label: string; onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      title={label}
      aria-label={label}
      onClick={onClick}
      className={cn('rounded-md p-1.5 text-muted-foreground hover:bg-muted/40 hover:text-foreground')}
    >
      {children}
    </button>
  )
}
