import { useCallback, useState } from 'react'
import { Link } from 'react-router-dom'
import { toast } from 'sonner'
import { FolderOpen, Pencil, Store, Trash2, UserRound } from 'lucide-react'
import { api, ApiError, type Device } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { formatBytes, formatDateTime, formatRelativeTime } from '@/lib/format'
import { useAuth } from '@/lib/auth-context'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
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

const SHORTCUTS = [
  { to: '/app/files', label: 'Arquivos', description: 'Samba, SFTP e FileBrowser na VPN', icon: FolderOpen },
  { to: '/app/profile', label: 'Perfil', description: 'Papel, cota e resumo da conta', icon: UserRound },
  { to: '/app/account', label: 'Editar conta', description: 'Trocar senha e chave SSH', icon: Pencil },
  { to: '/app/marketplace', label: 'Apps', description: 'Catálogo interno de programas', icon: Store },
] as const

// PortalPage é o autosserviço (Fase 10 + Fase 18): dispositivos próprios e
// atalhos para as páginas da conta. Senha/SSH ficam em /app/account.
export function PortalPage() {
  const { user } = useAuth()
  const fetchDevices = useCallback(() => api.listMyDevices(), [])
  const { data: devices, loading, error, reload } = usePollingData(fetchDevices, 10_000)

  return (
    <div className="flex flex-col gap-6">
      <div>
        <p className="mb-1 text-[11px] font-medium uppercase tracking-[0.14em] text-muted-foreground/70">Meu espaço</p>
        <h1 className="text-2xl font-semibold tracking-tight">
          {user ? `Olá, ${user.username}` : 'Início'}
        </h1>
        <p className="text-muted-foreground">
          Seus dispositivos VPN. Para adicionar um dispositivo novo, peça um convite a um administrador.
        </p>
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        {SHORTCUTS.map(({ to, label, description, icon: Icon }) => (
          <Link key={to} to={to} className="group">
            <Card className="h-full transition-colors group-hover:border-primary/40 group-hover:bg-primary/5">
              <CardHeader className="pb-2">
                <Icon className="mb-1 size-5 text-muted-foreground group-hover:text-primary" />
                <CardTitle className="text-base">{label}</CardTitle>
                <CardDescription>{description}</CardDescription>
              </CardHeader>
            </Card>
          </Link>
        ))}
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
