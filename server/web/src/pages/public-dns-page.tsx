import { useCallback, useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Plus, RefreshCw } from 'lucide-react'
import { toast } from 'sonner'
import { api, ApiError, type PublicZone } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { canWriteAdminProduct, isAdminRole } from '@/lib/roles'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { DataTable, type DataTableColumn } from '@/components/data-table'

export function PublicDNSPage() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const canWrite = isAdminRole(user?.role) && canWriteAdminProduct(user?.role, user?.products, 'dns')
  const fetchZones = useCallback(() => api.listPublicZones(), [])
  const { data, loading, reload } = usePollingData(fetchZones, 20_000)

  const columns: DataTableColumn<PublicZone>[] = [
    { key: 'name', header: 'Zona', cell: (z) => <span className="font-medium">{z.name}</span> },
    { key: 'status', header: 'Status', cell: (z) => <span className="text-muted-foreground">{z.status}</span> },
    {
      key: 'ns',
      header: 'Nameservers do stack',
      cell: (z) => <span className="hud-mono text-xs">{z.name_servers.join(' ') || '—'}</span>,
    },
  ]

  return (
    <div className="flex flex-col gap-6">
      {canWrite && !data?.cloudflare ? (
        <p className="text-sm text-muted-foreground">
          Cadastre a API em{' '}
          <Link to="/admin/dns/settings" className="underline underline-offset-4">
            DNS → Configurações
          </Link>
          .
        </p>
      ) : null}
      {canWrite ? (
        <div className="flex flex-wrap gap-2">
          <Button
            type="button"
            variant="outline"
            onClick={async () => {
              try {
                await api.importPublicZones()
                toast.success('Zonas importadas')
                reload()
              } catch (err) {
                toast.error(err instanceof ApiError ? err.message : 'Falha ao importar')
              }
            }}
          >
            <RefreshCw className="size-4" />
            Importar do Cloudflare
          </Button>
        </div>
      ) : null}
      {canWrite && data?.cloudflare ? <AddZoneForm onCreated={reload} /> : null}
      <DataTable
        columns={columns}
        rows={data?.items ?? []}
        rowKey={(z) => String(z.id)}
        loading={loading || !data}
        emptyTitle="Nenhuma zona. Importe ihuull.com ou adicione um domínio novo."
        onRowClick={(z) => navigate(`/admin/dns/public/${z.id}`)}
        page={1}
        perPage={50}
        total={data?.items.length ?? 0}
        onPageChange={() => undefined}
      />
    </div>
  )
}

function AddZoneForm({ onCreated }: { onCreated: () => void }) {
  const [name, setName] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      const z = await api.createPublicZone({ name: name.trim().toLowerCase() })
      setName('')
      toast.success(`Aponte o registrador para: ${z.name_servers.join(', ')}`)
      onCreated()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao criar zona')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Adicionar domínio ao stack</CardTitle>
        <CardDescription>
          Cria a zona na Cloudflare e devolve os NS do stack. Sem :53 neste VPS. ldpops e *.corp
          não entram.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={submit} className="flex flex-wrap items-end gap-2">
          <div className="space-y-1.5">
            <Label htmlFor="zone-name">Domínio</Label>
            <Input
              id="zone-name"
              value={name}
              onChange={(e) => setName(e.target.value.toLowerCase())}
              placeholder="app.ihuull.com"
              required
            />
          </div>
          <Button type="submit" disabled={busy || !name.includes('.')}>
            <Plus className="size-4" />
            {busy ? 'Criando…' : 'Criar zona'}
          </Button>
        </form>
      </CardContent>
    </Card>
  )
}
