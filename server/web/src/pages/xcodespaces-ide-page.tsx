import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import Editor from '@monaco-editor/react'
import { toast } from 'sonner'
import { File, Folder } from 'lucide-react'
import { api, ApiError } from '@/lib/api'
import { defineIhuullTheme, languageForPath } from '@/lib/monaco'
import { usePollingData } from '@/hooks/use-polling-data'
import { XGIT_CORP_ORIGIN, codespaceOpenHref } from '@/lib/product-host'
import { Button } from '@/components/ui/button'
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

export function XcodespacesIdePage() {
  const { id = '' } = useParams()
  const [dir, setDir] = useState('')
  const [filePath, setFilePath] = useState('')
  const [value, setValue] = useState<string | null>(null)
  const [open, setOpen] = useState(false)
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)
  const fetchCs = useCallback(() => api.getCodespace(id), [id])
  const fetchTree = useCallback(() => api.listCodespaceTree(id, dir || undefined), [id, dir])
  const fetchBlob = useCallback(
    () => (filePath ? api.getCodespaceBlob(id, filePath) : Promise.resolve(null)),
    [id, filePath],
  )
  const { data: cs, loading, error } = usePollingData(fetchCs, 30_000)
  const { data: tree } = usePollingData(fetchTree, 10_000)
  const { data: blob } = usePollingData(fetchBlob, 30_000)
  const original = blob?.content ?? ''
  const current = value ?? original
  const dirty = Boolean(filePath && current !== original)
  const language = languageForPath(filePath)

  const crumbs = useMemo(() => (dir ? dir.split('/') : []), [dir])

  useEffect(() => {
    if (cs?.kind === 'remote') {
      window.location.replace(codespaceOpenHref(cs))
    }
  }, [cs])

  if (loading || !cs) {
    return error ? <p className="p-6 text-sm text-destructive">{error}</p> : <Skeleton className="m-6 h-64" />
  }

  async function saveAndCommit() {
    if (!filePath) return
    setBusy(true)
    try {
      await api.writeCodespaceFile(id, { path: filePath, content: current })
      const out = await api.commitCodespace(id, { message: message.trim() })
      toast.success(out.merge_request_number ? `Commit + PR #${out.merge_request_number}` : `Commit ${out.sha.slice(0, 7)}`)
      setOpen(false)
      setMessage('')
      setValue(null)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha no commit')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex flex-wrap items-center justify-between gap-2 border-b border-border/60 px-4 py-2">
        <p className="text-sm">
          <Link to="/" className="text-muted-foreground hover:underline">
            Codespaces
          </Link>
          <span className="px-1.5 text-muted-foreground">/</span>
          <a href={`${XGIT_CORP_ORIGIN}/${cs.slug}`} className="text-primary hover:underline">
            {cs.slug}
          </a>
          <span className="px-1.5 text-muted-foreground">·</span>
          <span className="font-mono text-xs">{cs.branch}</span>
        </p>
        <div className="flex items-center gap-2">
          {cs.kind === 'quick' ? (
            <Button
              type="button"
              size="sm"
              className="btn-glow"
              disabled={busy}
              onClick={() => {
                setBusy(true)
                void api
                  .createCodespace({ slug: cs.slug, branch: cs.branch, kind: 'remote' })
                  .then((created) => {
                    window.location.href = codespaceOpenHref(created)
                  })
                  .catch((err) => {
                    toast.error(err instanceof ApiError ? err.message : 'Falha ao criar codespace')
                    setBusy(false)
                  })
              }}
            >
              {busy ? 'Criando VS Code…' : 'Abrir no VS Code'}
            </Button>
          ) : (
            <Button asChild size="sm" variant="outline">
              <a href={codespaceOpenHref(cs)}>Abrir VS Code</a>
            </Button>
          )}
          <Button type="button" className="btn-glow" size="sm" disabled={!dirty || !cs.can_write} onClick={() => setOpen(true)}>
            Commit
          </Button>
        </div>
      </div>
      {cs.kind === 'quick' ? (
        <p className="border-b border-border/60 px-4 py-2 text-xs text-muted-foreground">
          Editor rápido (Monaco). O VS Code com terminal fica em Create codespace no XGIT, ou no botão acima.
        </p>
      ) : null}
      <div className="grid min-h-0 flex-1 md:grid-cols-[220px_minmax(0,1fr)]">
        <aside className="min-h-0 overflow-y-auto border-r border-border/60 p-3 text-sm">
          <p className="hud-label mb-2 text-muted-foreground/70">Files</p>
          {dir ? (
            <button type="button" className="mb-2 text-xs text-primary hover:underline" onClick={() => setDir(dir.includes('/') ? dir.slice(0, dir.lastIndexOf('/')) : '')}>
              ← {crumbs[crumbs.length - 1] ?? '..'}
            </button>
          ) : null}
          <ul className="flex flex-col gap-0.5">
            {(tree?.items ?? []).map((e) => (
              <li key={e.path}>
                <button
                  type="button"
                  className="flex w-full items-center gap-2 rounded-md px-2 py-1 text-left hover:bg-muted/30"
                  onClick={() => {
                    if (e.type === 'tree') {
                      setDir(e.path)
                      return
                    }
                    setFilePath(e.path)
                    setValue(null)
                  }}
                >
                  {e.type === 'tree' ? <Folder className="size-3.5" /> : <File className="size-3.5" />}
                  <span className="truncate">{e.name}</span>
                </button>
              </li>
            ))}
          </ul>
        </aside>
        <div className="min-h-0 min-w-0">
          {filePath ? (
            <Editor
              height="100%"
              theme="ihuull-dark"
              language={language}
              value={current}
              onChange={(v) => setValue(v ?? '')}
              beforeMount={defineIhuullTheme}
              options={{ minimap: { enabled: false }, fontSize: 13, readOnly: !cs.can_write }}
            />
          ) : (
            <p className="p-6 text-sm text-muted-foreground">Escolha um arquivo à esquerda.</p>
          )}
        </div>
      </div>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Commit changes</DialogTitle>
            <DialogDescription>
              {cs.can_write ? `Salva ${filePath} e commita no worktree.` : 'Somente leitura.'}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-1.5">
            <Label htmlFor="cs-msg">Message</Label>
            <Input id="cs-msg" value={message} onChange={(e) => setMessage(e.target.value)} required maxLength={200} />
          </div>
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button type="button" className="btn-glow" disabled={busy || !message.trim()} onClick={() => void saveAndCommit()}>
              {busy ? 'Commiting…' : 'Commit'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
