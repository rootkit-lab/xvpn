import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { Trash2 } from 'lucide-react'
import { api, ApiError, type Device } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { formatBytes, formatDateTime, formatRelativeTime } from '@/lib/format'
import { useAuth } from '@/lib/auth-context'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Skeleton } from '@/components/ui/skeleton'
import { ProgressBar } from '@/components/ui/progress-bar'
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

// PortalPage é o autosserviço (Fase 10 + Fase 15): dispositivos próprios e
// chave SSH manual (escape hatch para máquina sem XVPN).
export function PortalPage() {
  const fetchDevices = useCallback(() => api.listMyDevices(), [])
  const { data: devices, loading, error, reload } = usePollingData(fetchDevices, 10_000)

  return (
    <div className="flex flex-col gap-6">
      <div>
        <p className="mb-1 text-[11px] font-medium uppercase tracking-[0.14em] text-muted-foreground/70">Meu espaço</p>
        <h1 className="text-2xl font-semibold tracking-tight">Início</h1>
        <p className="text-muted-foreground">
          Seus dispositivos VPN e chave SSH. Para adicionar um dispositivo novo, peça um convite a um administrador.
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

      <ManualSSHKeyCard />
    </div>
  )
}

function ManualSSHKeyCard() {
  const { user, isLoadingUser } = useAuth()
  const [sshKey, setSshKey] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    if (user) setSshKey(user.ssh_public_key ?? '')
  }, [user])

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      await api.updateMySSHPublicKey(sshKey)
      toast.success(
        user?.sftp_enabled
          ? 'Chave SSH atualizada e aplicada no SFTP'
          : 'Chave salva — passa a valer quando o admin ligar o SFTP',
      )
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : 'Falha ao salvar chave'
      setError(msg)
      toast.error(msg)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Chave SSH manual (SFTP)</CardTitle>
        <CardDescription>
          Escape hatch para celular ou máquina sem o cliente XVPN. As chaves dos seus dispositivos
          VPN entram sozinhas quando você abre o app conectado. Esta caixa é só a chave extra.
        </CardDescription>
      </CardHeader>
      <CardContent>
        {isLoadingUser || !user ? (
          <Skeleton className="h-28 w-full" />
        ) : (
          <form onSubmit={handleSubmit} className="flex flex-col gap-3">
            {!user.sftp_enabled && (
              <p className="text-xs text-muted-foreground">
                Seu SFTP ainda não está ligado — a chave fica guardada e o admin ativa o acesso.
              </p>
            )}
            <div className="flex flex-col gap-2">
              <Label htmlFor="portal-ssh-key">Chave pública</Label>
              <textarea
                id="portal-ssh-key"
                className="min-h-24 w-full rounded-md border border-input bg-background px-3 py-2 font-mono text-sm"
                placeholder="ssh-ed25519 AAAAC3NzaC1lZDI1NTE5... user@host"
                value={sshKey}
                onChange={(e) => setSshKey(e.target.value)}
                spellCheck={false}
                disabled={submitting}
              />
            </div>
            {submitting && <ProgressBar label="Salvando chave…" />}
            {error && <p className="text-sm text-destructive">{error}</p>}
            <div>
              <Button type="submit" disabled={submitting}>
                {submitting ? 'Salvando…' : 'Salvar chave'}
              </Button>
            </div>
          </form>
        )}
      </CardContent>
    </Card>
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
