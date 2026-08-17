import { useCallback, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { GitBranch, Lock, Plus } from 'lucide-react'
import { toast } from 'sonner'
import { api, ApiError, type Project } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { canWriteAdminProduct, isAdminRole } from '@/lib/roles'
import { isXgitAdminHost, xgitPath } from '@/lib/xgit'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

export function XgitReposPage() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const canWrite = isAdminRole(user?.role) && canWriteAdminProduct(user?.role, user?.products, 'forge')
  const adminHost = isXgitAdminHost()
  const fetchProjects = useCallback(() => api.listProjects(adminHost ? 'all' : 'mine'), [adminHost])
  const fetchSettings = useCallback(() => api.getXgitSettings(), [])
  const { data, loading, reload } = usePollingData(fetchProjects, 20_000)
  const { data: settings } = usePollingData(fetchSettings, 60_000)
  const canCreate = canWrite || (user?.role === 'member' && settings?.allow_member_create)
  const [query, setQuery] = useState('')
  const items = (data?.items ?? []).filter((p) => {
    const q = query.trim().toLowerCase()
    if (!q) return true
    return p.slug.includes(q) || p.name.toLowerCase().includes(q) || p.description.toLowerCase().includes(q)
  })

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <p className="hud-label text-muted-foreground/70">XGIT</p>
          <h2 className="font-display text-2xl font-semibold tracking-tight">Repositórios</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            {adminHost
              ? 'Todos os repositórios. ACL do app no Marketplace; membros de cada repo em Settings.'
              : 'Repositórios em que você participa. Clone só na VPN em xgit.corp.'}
          </p>
        </div>
      </div>

      {canCreate ? <CreateRepoForm onCreated={reload} useMemberApi={!canWrite} /> : null}

      <Input
        className="field-glass max-w-md"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="Filtrar repositórios"
        aria-label="Filtrar repositórios"
      />

      <div className="watch-complication overflow-hidden rounded-[18px]">
        {loading || !data ? (
          <p className="p-6 text-sm text-muted-foreground">Carregando…</p>
        ) : items.length === 0 ? (
          <p className="p-6 text-sm text-muted-foreground">Nenhum repositório visível para esta conta.</p>
        ) : (
          <ul className="divide-y divide-border/60">
            {items.map((p) => (
              <li key={p.slug}>
                <button
                  type="button"
                  onClick={() => navigate(xgitPath(p.slug))}
                  className="flex w-full items-start justify-between gap-4 px-5 py-4 text-left hover:bg-muted/30"
                >
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <GitBranch className="size-4 text-muted-foreground" />
                      <span className="font-medium text-primary">{p.slug}</span>
                      <Badge variant="outline">{p.visibility}</Badge>
                      {p.network === 'vpn' ? (
                        <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
                          <Lock className="size-3" /> vpn
                        </span>
                      ) : null}
                    </div>
                    <p className="mt-1 truncate text-sm text-muted-foreground">{p.description || p.name}</p>
                  </div>
                  <span className="shrink-0 text-xs text-muted-foreground">{p.member_count} membros</span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}

function CreateRepoForm({ onCreated, useMemberApi }: { onCreated: () => void; useMemberApi: boolean }) {
  const [slug, setSlug] = useState('')
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      const body = {
        slug: slug.trim().toLowerCase(),
        name: name.trim() || slug.trim().toLowerCase(),
        description: description.trim(),
        network: 'vpn' as const,
      }
      const created = useMemberApi ? await api.createXgitRepo(body) : await api.createProject(body)
      setSlug('')
      setName('')
      setDescription('')
      toast.success(`Repositório ${created.slug} criado`)
      onCreated()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao criar')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Novo repositório</CardTitle>
        <CardDescription>Bare em xgit.corp, grupo no XGROUP, MRs e pipeline. Membros e regras no detalhe.</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={submit} className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <div className="space-y-1.5">
            <Label htmlFor="repo-slug">Slug</Label>
            <Input
              id="repo-slug"
              className="field-glass"
              value={slug}
              onChange={(e) => setSlug(e.target.value.toLowerCase())}
              placeholder="xchat"
              required
              pattern="[a-z0-9][a-z0-9-]{0,18}[a-z0-9]"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="repo-name">Nome</Label>
            <Input id="repo-name" className="field-glass" value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="space-y-1.5 sm:col-span-2">
            <Label htmlFor="repo-desc">Descrição</Label>
            <Input id="repo-desc" className="field-glass" value={description} onChange={(e) => setDescription(e.target.value)} />
          </div>
          <div className="sm:col-span-2 lg:col-span-4">
            <Button type="submit" disabled={busy || slug.trim().length < 2}>
              <Plus className="size-4" />
              {busy ? 'Criando…' : 'Criar repositório'}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  )
}

export function repoHref(p: Project) {
  return xgitPath(p.slug)
}
