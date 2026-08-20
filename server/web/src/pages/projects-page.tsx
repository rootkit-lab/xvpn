import { useCallback, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus } from 'lucide-react'
import { toast } from 'sonner'
import { api, ApiError, type Project } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { canWriteAdminProduct, isAdminRole } from '@/lib/roles'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { DataTable, type DataTableColumn } from '@/components/data-table'

export function ProjectsPage() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const canWrite = isAdminRole(user?.role) && canWriteAdminProduct(user?.role, user?.products, 'forge')
  const fetchProjects = useCallback(() => api.listProjects(), [])
  const { data, loading, reload } = usePollingData(fetchProjects, 20_000)

  const columns: DataTableColumn<Project>[] = [
    { key: 'name', header: 'Projeto', cell: (p) => <span className="font-medium">{p.name}</span> },
    { key: 'slug', header: 'Slug', cell: (p) => <span className="text-muted-foreground">{p.slug}</span> },
    {
      key: 'rules',
      header: 'Regras',
      cell: (p) => (
        <span className="flex flex-wrap gap-1">
          <Badge variant="outline">{p.visibility}</Badge>
          <Badge variant="outline">{p.network}</Badge>
          {p.files_enabled ? <Badge variant="secondary">arquivos</Badge> : null}
        </span>
      ),
    },
    { key: 'n', header: 'Membros', cell: (p) => <span className="text-muted-foreground">{p.member_count}</span> },
  ]

  return (
    <div className="flex flex-col gap-6">
      {canWrite && <CreateProjectForm onCreated={reload} />}

      <DataTable
        columns={columns}
        rows={data?.items ?? []}
        rowKey={(p) => p.slug}
        loading={loading || !data}
        emptyTitle="Nenhum projeto ainda."
        onRowClick={(p) => navigate(`/admin/projects/${p.org}/${p.slug}`)}
        page={1}
        perPage={50}
        total={data?.items.length ?? 0}
        onPageChange={() => undefined}
      />
    </div>
  )
}

function CreateProjectForm({ onCreated }: { onCreated: () => void }) {
  const [slug, setSlug] = useState('')
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      const created = await api.createProject({
        org: 'xcorp',
        slug: slug.trim().toLowerCase(),
        name: name.trim() || slug.trim().toLowerCase(),
        description: description.trim(),
        network: 'vpn',
      })
      setSlug('')
      setName('')
      setDescription('')
      toast.success(`Projeto ${created.slug} criado`)
      onCreated()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao criar projeto')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Novo projeto</CardTitle>
        <CardDescription>
          Um slug, um grupo no XGROUP, bare em xgit.corp e MRs. Membros e regras ficam no detalhe.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={submit} className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <div className="space-y-1.5">
            <Label htmlFor="proj-slug">Slug</Label>
            <Input
              id="proj-slug"
              value={slug}
              onChange={(e) => setSlug(e.target.value.toLowerCase())}
              placeholder="xchat"
              required
              pattern="[a-z0-9][a-z0-9-]{0,18}[a-z0-9]"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="proj-name">Nome</Label>
            <Input id="proj-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="XCHAT" />
          </div>
          <div className="space-y-1.5 sm:col-span-2">
            <Label htmlFor="proj-desc">Descrição</Label>
            <Input id="proj-desc" value={description} onChange={(e) => setDescription(e.target.value)} />
          </div>
          <div className="sm:col-span-2 lg:col-span-4">
            <Button type="submit" disabled={busy || slug.trim().length < 2}>
              <Plus className="size-4" />
              {busy ? 'Criando…' : 'Criar projeto'}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  )
}
