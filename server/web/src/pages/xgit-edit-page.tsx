import { useCallback, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import Editor from '@monaco-editor/react'
import { toast } from 'sonner'
import { api, ApiError, type ProjectRole } from '@/lib/api'
import { defineIhuullTheme, languageForPath } from '@/lib/monaco'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { canWriteAdminProduct, isAdminRole } from '@/lib/roles'
import { xgitPath } from '@/lib/xgit'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
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

const ROLE_RANK: Record<ProjectRole, number> = {
  guest: 0,
  reporter: 1,
  developer: 2,
  maintainer: 3,
  owner: 4,
}

const MAX_EDIT_BYTES = 2 << 20

export function XgitEditPage() {
  const { org = '', slug = '', ref = 'HEAD' } = useParams()
  const splat = useParams()['*'] ?? ''
  const filePath = decodeURIComponent(splat)
  const navigate = useNavigate()
  const { user } = useAuth()
  const fetchBlob = useCallback(
    () => api.getProjectBlob(`${org}/${slug}`, filePath, ref),
    [slug, filePath, ref],
  )
  const fetchGit = useCallback(() => api.getProjectGit(`${org}/${slug}`), [slug])
  const fetchProject = useCallback(() => api.getProject(`${org}/${slug}`), [slug])
  const { data: blob, loading, error } = usePollingData(fetchBlob, 60_000)
  const { data: git } = usePollingData(fetchGit, 30_000)
  const { data: project } = usePollingData(fetchProject, 30_000)
  const [value, setValue] = useState<string | null>(null)
  const [open, setOpen] = useState(false)
  const [message, setMessage] = useState('')
  const [description, setDescription] = useState('')
  const [busy, setBusy] = useState(false)

  const original = blob?.content ?? ''
  const current = value ?? original
  const dirty = current !== original
  const language = languageForPath(filePath)
  const tooBig = original.length > MAX_EDIT_BYTES
  const myRole = project?.members?.find((m) => m.user_id === user?.id)?.role
  const canWrite =
    (isAdminRole(user?.role) && canWriteAdminProduct(user?.role, user?.products, 'forge')) ||
    (myRole != null && ROLE_RANK[myRole] >= ROLE_RANK.developer)
  const protectedRule = (git?.protected_branches ?? []).find((r) => matchBranch(r.pattern, ref))
  const minRank = protectedRule ? ROLE_RANK[protectedRule.min_push_role] : 0
  const myRank =
    isAdminRole(user?.role) && canWriteAdminProduct(user?.role, user?.products, 'forge')
      ? ROLE_RANK.owner
      : myRole
        ? ROLE_RANK[myRole]
        : 0
  const mustPR = Boolean(protectedRule && myRank < minRank)
  const preview = useMemo(() => previewDiff(filePath, original, current), [filePath, original, current])

  if (!filePath) {
    return <p className="text-sm text-destructive">Caminho inválido.</p>
  }
  if (loading || !blob) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-64 w-full" />
  }
  if (blob.binary || tooBig) {
    return (
      <p className="text-sm text-muted-foreground">
        Este arquivo não abre no editor (binário ou maior que 2 MiB). Clone o repositório.
      </p>
    )
  }
  if (!canWrite) {
    return <p className="text-sm text-muted-foreground">Sem permissão para commitar neste repositório.</p>
  }

  async function commit() {
    const msg = message.trim()
    if (!msg) {
      toast.error('Mensagem de commit obrigatória')
      return
    }
    setBusy(true)
    try {
      const res = await api.putContents(`${org}/${slug}`, {
        path: filePath,
        ref,
        content: current,
        message: msg,
        description: description.trim() || undefined,
        open_pr: mustPR,
      })
      if (res.merge_request_number) {
        toast.success(`PR #${res.merge_request_number} aberto em ${res.branch}`)
        navigate(xgitPath(`${org}/${slug}/pulls/${res.merge_request_number}`))
      } else {
        toast.success(`Commit ${res.sha.slice(0, 7)} em ${res.branch}`)
        navigate(xgitPath(`${org}/${slug}/blob/${filePath}`))
      }
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao commitar')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-col gap-3">
      <p className="text-sm text-muted-foreground">
        <Link to={xgitPath(`${org}/${slug}/blob/${filePath}`)} className="hover:underline">
          {filePath}
        </Link>
        <span className="px-1.5">·</span>
        <span className="font-mono text-xs">{ref}</span>
      </p>
      <div className="watch-complication overflow-hidden rounded-[18px]">
        <Editor
          height="28rem"
          language={language}
          value={current}
          theme={defineIhuullTheme()}
          onChange={(v) => setValue(v ?? '')}
          options={{
            minimap: { enabled: false },
            fontSize: 13,
            fontFamily: 'JetBrains Mono, Fira Code, ui-monospace, monospace',
            scrollBeyondLastLine: false,
            wordWrap: 'on',
          }}
        />
      </div>
      <div className="flex flex-wrap gap-2">
        <Button type="button" disabled={!dirty} onClick={() => setOpen(true)}>
          Salvar
        </Button>
        <Button
          type="button"
          variant="outline"
          onClick={() => {
            setValue(original)
            navigate(xgitPath(`${org}/${slug}/blob/${filePath}`))
          }}
        >
          Cancelar
        </Button>
      </div>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Commit changes</DialogTitle>
            <DialogDescription>
              {mustPR
                ? 'Branch protegida: o commit vai para uma branch nova e abre um pull request.'
                : `O commit entra em ${ref}.`}
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="commit-msg">Mensagem</Label>
              <Input
                id="commit-msg"
                value={message}
                onChange={(e) => setMessage(e.target.value)}
                required
                maxLength={200}
                placeholder={`Update ${filePath}`}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="commit-desc">Descrição (opcional)</Label>
              <Textarea id="commit-desc" value={description} onChange={(e) => setDescription(e.target.value)} rows={3} />
            </div>
            <pre className="max-h-40 overflow-auto rounded-md bg-muted/40 p-3 font-mono text-[11px] leading-relaxed">
              {preview || 'Sem alterações'}
            </pre>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setOpen(false)}>
              Voltar
            </Button>
            <Button type="button" disabled={busy || !dirty || !message.trim()} onClick={() => void commit()}>
              {busy ? 'Commitando…' : mustPR ? 'Commitar e abrir PR' : 'Commitar'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function matchBranch(pattern: string, ref: string): boolean {
  const name = ref.replace(/^refs\/heads\//, '')
  if (pattern === name || pattern === `refs/heads/${name}`) return true
  if (pattern.endsWith('/*')) {
    const prefix = pattern.slice(0, -1)
    return name.startsWith(prefix) || `refs/heads/${name}`.startsWith(prefix)
  }
  return false
}

function previewDiff(path: string, before: string, after: string): string {
  if (before === after) return ''
  const a = before.split('\n')
  const b = after.split('\n')
  const lines = [`--- a/${path}`, `+++ b/${path}`]
  const max = Math.max(a.length, b.length)
  for (let i = 0; i < max; i++) {
    if (a[i] === b[i]) continue
    if (i < a.length) lines.push(`-${a[i]}`)
    if (i < b.length) lines.push(`+${b[i]}`)
  }
  return lines.join('\n')
}
