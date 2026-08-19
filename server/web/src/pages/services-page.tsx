import { useCallback, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus } from 'lucide-react'
import { toast } from 'sonner'
import { api, ApiError, type ManagedService, type MeshServer, type ServiceBind, type ServiceHost, type ServiceKind } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { canWriteAdminProduct, isAdminRole } from '@/lib/roles'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { DataTable, type DataTableColumn } from '@/components/data-table'

const KINDS: ServiceKind[] = ['redis', 'mongo', 'rabbitmq', 'lb']

export function ServicesPage() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const canWrite = isAdminRole(user?.role) && canWriteAdminProduct(user?.role, user?.products, 'managed')
  const fetchServices = useCallback(() => api.listServices(), [])
  const { data, loading, reload } = usePollingData(fetchServices, 15_000)

  const columns: DataTableColumn<ManagedService>[] = [
    { key: 'slug', header: 'Slug', cell: (s) => <span className="font-medium">{s.slug}</span> },
    { key: 'kind', header: 'Kind', cell: (s) => <Badge variant="outline">{s.kind}</Badge> },
    {
      key: 'host',
      header: 'Host',
      cell: (s) => <span className="text-muted-foreground">{s.host === 'local' ? 'local' : s.mesh_hostname || 'mesh'}</span>,
    },
    { key: 'endpoint', header: 'Endpoint', cell: (s) => <span className="font-mono text-xs">{s.endpoint}</span> },
    {
      key: 'status',
      header: 'Status',
      cell: (s) => <ServiceStatusBadge status={s.status} />,
    },
  ]

  return (
    <div className="flex flex-col gap-6">
      {canWrite ? <CreateServiceForm onCreated={reload} /> : null}
      <DataTable
        columns={columns}
        rows={data?.items ?? []}
        rowKey={(s) => s.slug}
        loading={loading || !data}
        emptyTitle="Nenhum serviço ainda."
        onRowClick={(s) => navigate(`/admin/services/${s.slug}`)}
        page={1}
        perPage={50}
        total={data?.items.length ?? 0}
        onPageChange={() => undefined}
      />
    </div>
  )
}

export function ServiceStatusBadge({ status }: { status: ManagedService['status'] }) {
  const variant = status === 'ready' ? 'secondary' : status === 'error' ? 'destructive' : 'outline'
  return <Badge variant={variant}>{status}</Badge>
}

function CreateServiceForm({ onCreated }: { onCreated: () => void }) {
  const fetchServers = useCallback(() => api.listServers(), [])
  const { data: servers } = usePollingData(fetchServers, 60_000)
  const [slug, setSlug] = useState('')
  const [kind, setKind] = useState<ServiceKind>('redis')
  const [host, setHost] = useState<ServiceHost>('local')
  const [bind, setBind] = useState<ServiceBind>('wg0')
  const [project, setProject] = useState('')
  const [meshId, setMeshId] = useState('')
  const [backends, setBackends] = useState('')
  const [busy, setBusy] = useState(false)
  const [once, setOnce] = useState('')

  const peers = (servers?.items ?? []).filter((s: MeshServer) => s.role === 'mesh' || s.role === 'runner')

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      const created = await api.createService({
        slug: slug.trim().toLowerCase(),
        kind,
        host,
        bind,
        project_slug: project.trim() || undefined,
        mesh_server_id: host === 'mesh' && meshId ? Number(meshId) : undefined,
        backends:
          kind === 'lb'
            ? backends
                .split(/[\s,]+/)
                .map((b) => b.trim())
                .filter(Boolean)
            : undefined,
      })
      setSlug('')
      setOnce(created.password ?? '')
      toast.success(`Serviço ${created.slug} criado`)
      onCreated()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao criar serviço')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Novo serviço</CardTitle>
        <CardDescription>
          Bind só em wg0 ou loopback. Mongo do control-plane (127.0.0.1:27017) não entra aqui. Redis/Rabbit não são o
          hub do XCHAT.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={submit} className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          <div className="space-y-1.5">
            <Label htmlFor="svc-slug">Slug</Label>
            <Input
              id="svc-slug"
              className="field-glass"
              value={slug}
              onChange={(e) => setSlug(e.target.value.toLowerCase())}
              placeholder="cache"
              required
              pattern="[a-z0-9][a-z0-9-]{0,18}[a-z0-9]"
            />
          </div>
          <div className="space-y-1.5">
            <Label>Kind</Label>
            <Select value={kind} onValueChange={(v) => setKind(v as ServiceKind)}>
              <SelectTrigger className="field-glass">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {KINDS.map((k) => (
                  <SelectItem key={k} value={k}>
                    {k}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1.5">
            <Label>Host</Label>
            <Select value={host} onValueChange={(v) => setHost(v as ServiceHost)}>
              <SelectTrigger className="field-glass">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="local">local (este VPS)</SelectItem>
                <SelectItem value="mesh">peer da malha</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1.5">
            <Label>Bind</Label>
            <Select value={bind} onValueChange={(v) => setBind(v as ServiceBind)}>
              <SelectTrigger className="field-glass">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="wg0">wg0 (svc-*.corp)</SelectItem>
                <SelectItem value="loopback">127.0.0.1 (só o host)</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="svc-proj">Projeto (opcional)</Label>
            <Input id="svc-proj" className="field-glass" value={project} onChange={(e) => setProject(e.target.value)} placeholder="lab" />
          </div>
          {host === 'mesh' ? (
            <div className="space-y-1.5">
              <Label>Peer</Label>
              <Select value={meshId} onValueChange={setMeshId}>
                <SelectTrigger className="field-glass">
                  <SelectValue placeholder="escolha o host" />
                </SelectTrigger>
                <SelectContent>
                  {peers.map((s) => (
                    <SelectItem key={s.id} value={String(s.id)}>
                      {s.hostname} {s.wg_ip ? `(${s.wg_ip})` : ''}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          ) : null}
          {kind === 'lb' ? (
            <div className="space-y-1.5 sm:col-span-2">
              <Label htmlFor="svc-be">Backends (IPv4:porta)</Label>
              <Input
                id="svc-be"
                className="field-glass"
                value={backends}
                onChange={(e) => setBackends(e.target.value)}
                placeholder="10.66.66.1:9000"
              />
            </div>
          ) : null}
          <div className="sm:col-span-2 lg:col-span-3">
            <Button type="submit" disabled={busy || slug.trim().length < 2}>
              <Plus className="size-4" />
              {busy ? 'Criando…' : 'Provisionar'}
            </Button>
          </div>
        </form>
        {once ? (
          <pre className="watch-complication mt-4 overflow-x-auto rounded-[18px] p-4 font-mono text-xs">
            {`senha (copie agora):\n${once}`}
          </pre>
        ) : null}
      </CardContent>
    </Card>
  )
}
