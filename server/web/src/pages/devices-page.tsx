import { useCallback, useState } from 'react'
import { toast } from 'sonner'
import { Trash2 } from 'lucide-react'
import { api, ApiError, type Device } from '@/lib/api'
import { useAuth } from '@/lib/auth-context'
import { canWriteAdminProduct } from '@/lib/roles'
import { usePollingData } from '@/hooks/use-polling-data'
import { formatBytes, formatDateTime, formatRelativeTime } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { FilterBar } from '@/components/filter-bar'
import { DataTable, type DataTableColumn } from '@/components/data-table'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'

const HANDSHAKE_RECENT_THRESHOLD_MS = 3 * 60 * 1000

function isOnline(device: Device): boolean {
  if (!device.last_handshake) return false
  return Date.now() - new Date(device.last_handshake).getTime() < HANDSHAKE_RECENT_THRESHOLD_MS
}

export function DevicesPage() {
  const { user: caller } = useAuth()
  const canRevoke = canWriteAdminProduct(caller?.role, caller?.products, 'core')
  const [page, setPage] = useState(1)
  const [q, setQ] = useState('')
  const fetchDevices = useCallback(() => api.listDevices({ page, per_page: 25, q }), [page, q])
  const { data, loading, reload } = usePollingData(fetchDevices, 10_000)

  const columns: DataTableColumn<Device>[] = [
    { key: 'name', header: 'Nome', cell: (d) => <span className="font-medium">{d.name}</span> },
    { key: 'ip', header: 'IP', cell: (d) => <span className="text-muted-foreground">{d.allowed_ip}</span> },
    {
      key: 'status',
      header: 'Status',
      cell: (d) => <Badge variant={isOnline(d) ? 'default' : 'secondary'}>{isOnline(d) ? 'Online' : 'Offline'}</Badge>,
    },
    {
      key: 'hs',
      header: 'Último handshake',
      cell: (d) => (
        <span className="text-muted-foreground">{d.last_handshake ? formatRelativeTime(d.last_handshake) : 'nunca'}</span>
      ),
    },
    {
      key: 'traffic',
      header: 'Tráfego',
      cell: (d) => (
        <span className="text-muted-foreground">
          ↓ {formatBytes(d.receive_bytes)} / ↑ {formatBytes(d.transmit_bytes)}
        </span>
      ),
    },
    ...(canRevoke
      ? [
          {
            key: 'actions',
            header: 'Ações',
            className: 'text-right',
            cell: (d: Device) => <RevokeDevice device={d} onChanged={reload} />,
          } satisfies DataTableColumn<Device>,
        ]
      : []),
  ]

  return (
    <div className="flex flex-col gap-6">
      <FilterBar
        q={q}
        onQChange={(next) => {
          setQ(next)
          setPage(1)
        }}
        placeholder="Buscar por nome ou IP"
      />
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Todos os dispositivos</CardTitle>
        </CardHeader>
        <CardContent>
          <DataTable
            columns={columns}
            rows={data?.items ?? []}
            rowKey={(d) => d.id}
            loading={loading || !data}
            emptyTitle="Nenhum dispositivo registrado ainda."
            page={data?.page ?? page}
            perPage={data?.per_page ?? 25}
            total={data?.total ?? 0}
            onPageChange={setPage}
          />
        </CardContent>
      </Card>
    </div>
  )
}

function RevokeDevice({ device, onChanged }: { device: Device; onChanged: () => void }) {
  const [revoking, setRevoking] = useState(false)

  async function handleRevoke() {
    setRevoking(true)
    try {
      await api.deleteDevice(device.id)
      toast.success(`Dispositivo "${device.name}" revogado`)
      onChanged()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao revogar dispositivo')
    } finally {
      setRevoking(false)
    }
  }

  return (
    <AlertDialog>
      <AlertDialogTrigger asChild>
        <Button variant="ghost" size="icon" disabled={revoking} onClick={(e) => e.stopPropagation()}>
          <Trash2 className="size-4 text-destructive" />
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Revogar dispositivo "{device.name}"?</AlertDialogTitle>
          <AlertDialogDescription>
            Registrado em {formatDateTime(device.created_at)}. Revogar remove o peer da interface WireGuard
            imediatamente — a próxima tentativa de handshake falhará. Essa ação não pode ser desfeita.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancelar</AlertDialogCancel>
          <AlertDialogAction onClick={handleRevoke}>Revogar</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
