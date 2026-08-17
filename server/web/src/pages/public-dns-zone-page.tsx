import { useCallback, useState, type FormEvent } from 'react'
import { Link, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import { api, ApiError, type PublicRecord } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { canWriteAdminProduct, isAdminRole } from '@/lib/roles'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { DataTable, type DataTableColumn } from '@/components/data-table'
import { Skeleton } from '@/components/ui/skeleton'

export function PublicDNSZonePage() {
  const { id = '' } = useParams()
  const zoneID = Number(id)
  const { user } = useAuth()
  const canWrite = isAdminRole(user?.role) && canWriteAdminProduct(user?.role, user?.products, 'dns')
  const fetchZone = useCallback(() => api.listPublicRecords(zoneID), [zoneID])
  const { data, loading, error, reload } = usePollingData(fetchZone, 20_000)

  const columns: DataTableColumn<PublicRecord>[] = [
    { key: 'type', header: 'Tipo', cell: (r) => <span className="hud-mono text-xs">{r.type}</span> },
    { key: 'name', header: 'Nome', cell: (r) => <span className="font-medium">{r.name}</span> },
    { key: 'content', header: 'Público', cell: (r) => <span className="text-muted-foreground">{r.content}</span> },
    { key: 'intra', header: 'VPN', cell: (r) => <span className="text-muted-foreground">{r.intranet_ipv4 || '—'}</span> },
    {
      key: 'act',
      header: '',
      cell: (r) =>
        canWrite ? (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={async () => {
              try {
                await api.deletePublicRecord(zoneID, r.id)
                toast.success('Registro removido')
                reload()
              } catch (err) {
                toast.error(err instanceof ApiError ? err.message : 'Falha ao remover')
              }
            }}
          >
            Remover
          </Button>
        ) : null,
    },
  ]

  if (loading || !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-48 w-full" />
  }

  return (
    <div className="flex flex-col gap-6">
      <p className="text-sm text-muted-foreground">
        <Link to="/admin/dns/public" className="hover:underline">
          Zonas
        </Link>
        <span className="px-1.5">/</span>
        <span className="text-foreground">{data.zone.name}</span>
      </p>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Nameservers do stack</CardTitle>
          <CardDescription>Aponte o registrador deste domínio para estes NS. Sem :53 neste VPS.</CardDescription>
        </CardHeader>
        <CardContent>
          <pre className="hud-mono text-xs leading-6">{data.zone.name_servers.join('\n') || '—'}</pre>
        </CardContent>
      </Card>

      {canWrite ? <AddRecordForm zoneId={zoneID} onCreated={reload} /> : null}

      <DataTable
        columns={columns}
        rows={data.items}
        rowKey={(r) => String(r.id)}
        loading={false}
        emptyTitle="Nenhum registro ainda."
        page={1}
        perPage={50}
        total={data.items.length}
        onPageChange={() => undefined}
      />
    </div>
  )
}

function AddRecordForm({ zoneId, onCreated }: { zoneId: number; onCreated: () => void }) {
  const [type, setType] = useState('A')
  const [name, setName] = useState('')
  const [content, setContent] = useState('206.189.224.72')
  const [intranet, setIntranet] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      await api.createPublicRecord(zoneId, {
        type,
        name: name.trim() || '@',
        content: content.trim(),
        intranet_ipv4: intranet.trim() || undefined,
      })
      setName('')
      setIntranet('')
      toast.success('Registro criado no Cloudflare')
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
        <CardTitle className="text-base">Novo registro</CardTitle>
        <CardDescription>
          Conteúdo público não pode ser RFC1918. Visão interna (opcional) é 10.66.66.x no dnsmasq.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={submit} className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <div className="space-y-1.5">
            <Label>Tipo</Label>
            <Select value={type} onValueChange={setType}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {['A', 'AAAA', 'CNAME', 'TXT', 'MX'].map((t) => (
                  <SelectItem key={t} value={t}>
                    {t}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="rr-name">Nome</Label>
            <Input id="rr-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="@ ou www" />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="rr-content">Público</Label>
            <Input id="rr-content" value={content} onChange={(e) => setContent(e.target.value)} required />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="rr-intra">VPN (10.66.66.x)</Label>
            <Input id="rr-intra" value={intranet} onChange={(e) => setIntranet(e.target.value)} placeholder="opcional" />
          </div>
          <div className="sm:col-span-2 lg:col-span-4">
            <Button type="submit" disabled={busy}>
              {busy ? 'Criando…' : 'Publicar'}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  )
}
