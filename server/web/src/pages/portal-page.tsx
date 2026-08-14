import { useCallback, useState } from 'react'
import { toast } from 'sonner'
import { Trash2 } from 'lucide-react'
import { api, ApiError, type Device } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { formatBytes, formatDateTime, formatRelativeTime } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Skeleton } from '@/components/ui/skeleton'
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

// PortalPage é o autosserviço mínimo da Fase 10 (ver PLAN.md §6.7):
// qualquer papel autenticado — mas sobretudo member, que não tem acesso às
// telas administrativas — consegue ver e revogar os próprios dispositivos
// sem precisar de um admin. Para registrar um dispositivo *novo*, ainda é
// preciso pedir um convite a um admin/super_admin (tela Usuários);
// autosserviço aqui é só leitura + revogação.
export function PortalPage() {
  const fetchDevices = useCallback(() => api.listMyDevices(), [])
  const { data: devices, loading, error, reload } = usePollingData(fetchDevices, 10_000)

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="text-2xl font-semibold">Meus dispositivos</h1>
        <p className="text-muted-foreground">
          Dispositivos VPN registrados na sua conta. Para adicionar um novo, peça um convite a um administrador.
        </p>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Seus dispositivos</CardTitle>
        </CardHeader>
        <CardContent>
          {loading || !devices ? (
            <Skeleton className="h-32 w-full" />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Nome</TableHead>
                  <TableHead>IP</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Último handshake</TableHead>
                  <TableHead>Tráfego</TableHead>
                  <TableHead className="text-right">Ações</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {devices.map((device) => (
                  <MyDeviceRow key={device.id} device={device} onChanged={reload} />
                ))}
                {devices.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={6} className="text-center text-muted-foreground">
                      Nenhum dispositivo registrado ainda.
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function MyDeviceRow({ device, onChanged }: { device: Device; onChanged: () => void }) {
  const [revoking, setRevoking] = useState(false)
  const online = isOnline(device)

  async function handleRevoke() {
    setRevoking(true)
    try {
      await api.deleteMyDevice(device.id)
      toast.success(`Dispositivo "${device.name}" revogado`)
      onChanged()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao revogar dispositivo')
    } finally {
      setRevoking(false)
    }
  }

  return (
    <TableRow>
      <TableCell className="font-medium">{device.name}</TableCell>
      <TableCell className="text-muted-foreground">{device.allowed_ip}</TableCell>
      <TableCell>
        <Badge variant={online ? 'default' : 'secondary'}>{online ? 'Online' : 'Offline'}</Badge>
      </TableCell>
      <TableCell className="text-muted-foreground">
        {device.last_handshake ? formatRelativeTime(device.last_handshake) : 'nunca'}
      </TableCell>
      <TableCell className="text-muted-foreground">
        ↓ {formatBytes(device.receive_bytes)} / ↑ {formatBytes(device.transmit_bytes)}
      </TableCell>
      <TableCell className="text-right">
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button variant="ghost" size="icon" disabled={revoking}>
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
      </TableCell>
    </TableRow>
  )
}
