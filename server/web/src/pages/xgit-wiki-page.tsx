import { useCallback, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import { api, ApiError } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { xgitRepoPath } from '@/lib/xgit'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Skeleton } from '@/components/ui/skeleton'
import { MarkdownDoc } from '@/components/markdown-doc'
import { cn } from '@/lib/utils'

export function XgitWikiPage() {
  const { org = '', slug = '', page: pageParam } = useParams()
  const page = pageParam || 'Home'
  const { user } = useAuth()
  const repo = `${org}/${slug}`
  const fetchList = useCallback(() => api.listWikiPages(repo), [repo])
  const fetchPage = useCallback(() => api.getWikiPage(repo, page), [repo, page])
  const { data: list, reload: reloadList } = usePollingData(fetchList, 20_000)
  const { data, loading, error, reload } = usePollingData(fetchPage, 20_000)
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState('')
  const [busy, setBusy] = useState(false)
  const [newPage, setNewPage] = useState('')
  const canWrite = Boolean(user)

  const items = list?.items ?? []
  const missing = Boolean(error) && !data
  const text = data?.content ?? ''

  async function save() {
    setBusy(true)
    try {
      await api.putWikiPage(repo, page, draft, `wiki: ${page}`)
      toast.success('Wiki gravada')
      setEditing(false)
      reload()
      reloadList()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao gravar')
    } finally {
      setBusy(false)
    }
  }

  if (loading && !data && !missing) {
    return <Skeleton className="h-64 w-full" />
  }

  return (
    <div className="grid gap-6 md:grid-cols-[14rem_1fr]">
      <aside className="flex flex-col gap-2">
        <p className="text-xs uppercase tracking-wide text-muted-foreground">Páginas</p>
        {items.length === 0 ? <p className="text-sm text-muted-foreground">Nenhuma ainda.</p> : null}
        {items.map((it) => (
          <Link
            key={it.page}
            to={xgitRepoPath(org, slug, `wiki/${encodeURIComponent(it.page)}`)}
            className={cn(
              'rounded-md px-2 py-1 text-sm',
              it.page === page ? 'bg-muted font-medium' : 'text-muted-foreground hover:text-foreground',
            )}
          >
            {it.page}
          </Link>
        ))}
        {canWrite ? (
          <form
            className="mt-2 flex gap-1"
            onSubmit={(e) => {
              e.preventDefault()
              const name = newPage.trim()
              if (name) window.location.assign(xgitRepoPath(org, slug, `wiki/${encodeURIComponent(name)}`))
            }}
          >
            <Input value={newPage} onChange={(e) => setNewPage(e.target.value)} placeholder="Nova página" />
          </form>
        ) : null}
      </aside>
      <section className="flex min-w-0 flex-col gap-3">
        <div className="flex items-center justify-between gap-2">
          <h1 className="text-lg font-semibold">{page}</h1>
          {canWrite ? (
            editing ? (
              <div className="flex gap-2">
                <Button variant="ghost" onClick={() => setEditing(false)} disabled={busy}>
                  Cancelar
                </Button>
                <Button onClick={() => void save()} disabled={busy}>
                  Gravar
                </Button>
              </div>
            ) : (
              <Button
                variant="outline"
                onClick={() => {
                  setDraft(text)
                  setEditing(true)
                }}
              >
                Editar
              </Button>
            )
          ) : null}
        </div>
        {editing ? (
          <Textarea value={draft} onChange={(e) => setDraft(e.target.value)} className="min-h-72 font-mono text-sm" />
        ) : missing ? (
          <p className="text-sm text-muted-foreground">
            Página vazia. {canWrite ? 'Clique em Editar para criar a Home (#1).' : ''}
          </p>
        ) : (
          <MarkdownDoc text={text || '_vazia_'} label={page} />
        )}
      </section>
    </div>
  )
}
