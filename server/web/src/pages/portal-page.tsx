import { useCallback, useState } from 'react'
import { Link } from 'react-router-dom'
import { QRCodeSVG } from 'qrcode.react'
import { toast } from 'sonner'
import {
  Copy,
  Laptop,
  MonitorSmartphone,
  Pencil,
  Plus,
  Trash2,
  UserRound,
} from 'lucide-react'
import { api, ApiError, type Device, type InviteResponse } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { formatBytes, formatDateTime, formatRelativeTime } from '@/lib/format'
import { CopyField } from '@/components/copy-field'
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
  { to: '/my/profile', label: 'Perfil', description: 'Papel, cota e resumo da conta', icon: UserRound },
  { to: '/my/account', label: 'Editar conta', description: 'Trocar senha e chave SSH', icon: Pencil },
] as const

const ENROLL_STEPS = [
  'Clique em Gerar código abaixo e copie o convite (válido por poucos minutos).',
  'Abra o app XVPN no computador onde quer usar a VPN.',
  'Cole o código, escolha um nome para o dispositivo e confirme.',
] as const

// PortalPage é o autosserviço (Fase 10 + Fase 18): dispositivos próprios,
// geração de convite e atalhos da conta.
export function PortalPage() {
  const fetchDevices = useCallback(() => api.listMyDevices(), [])
  const { data: devices, loading, error, reload } = usePollingData(fetchDevices, 10_000)

  return (
    <div className="flex flex-col gap-6">
      <AddDeviceCard onDeviceEnrolled={reload} />

      <div className="grid gap-3 sm:grid-cols-2">
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
          <CardDescription>
            Cada notebook ou celular registrado recebe um IP fixo na VPN. Revogue dispositivos que você não usa mais.
          </CardDescription>
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
                    <TableCell colSpan={6} className="py-10 text-center">
                      <p className="text-muted-foreground">Nenhum dispositivo registrado ainda.</p>
                      <p className="mt-1 text-sm text-muted-foreground">
                        Use o cartão <span className="text-foreground">Registrar novo dispositivo</span> acima para gerar um convite.
                      </p>
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

function AddDeviceCard({ onDeviceEnrolled }: { onDeviceEnrolled: () => void }) {
  const [invite, setInvite] = useState<InviteResponse | null>(null)
  const [generating, setGenerating] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function generate() {
    setGenerating(true)
    setError(null)
    try {
      setInvite(await api.createMyInvite())
      onDeviceEnrolled()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Falha ao gerar convite')
    } finally {
      setGenerating(false)
    }
  }

  return (
    <Card className="overflow-hidden border-primary/25 bg-gradient-to-br from-primary/10 via-card to-card">
      <CardHeader className="pb-3">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="flex items-start gap-3">
            <div className="flex size-11 shrink-0 items-center justify-center rounded-2xl bg-primary/15 text-primary">
              <MonitorSmartphone className="size-5" />
            </div>
            <div>
              <CardTitle className="text-lg">Registrar novo dispositivo</CardTitle>
              <CardDescription className="mt-1 max-w-xl">
                Gere um código de convite aqui e use no app desktop XVPN (Linux ou Windows). Um convite = um dispositivo.
              </CardDescription>
            </div>
          </div>
          <Button onClick={generate} disabled={generating} className="shrink-0 gap-2">
            <Plus className="size-4" />
            {generating ? 'Gerando…' : invite ? 'Gerar outro código' : 'Gerar código de convite'}
          </Button>
        </div>
      </CardHeader>
      <CardContent className="flex flex-col gap-5">
        <ol className="grid gap-2 sm:grid-cols-3">
          {ENROLL_STEPS.map((step, i) => (
            <li
              key={step}
              className="watch-complication flex gap-3 rounded-[16px] border border-white/8 p-3 text-sm text-muted-foreground"
            >
              <span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-primary/20 text-xs font-semibold text-primary">
                {i + 1}
              </span>
              <span>{step}</span>
            </li>
          ))}
        </ol>

        {error && <p className="text-sm text-destructive">{error}</p>}

        {invite && (
          <div className="watch-complication grid gap-4 rounded-[18px] border border-white/10 p-4 sm:grid-cols-[auto_1fr] sm:items-start">
            <div className="flex flex-col items-center gap-2">
              <div className="rounded-xl border border-white/10 bg-white p-3">
                <QRCodeSVG value={JSON.stringify({ invite_token: invite.token })} size={148} />
              </div>
              <p className="text-center text-xs text-muted-foreground">QR para o app (futuro)</p>
            </div>
            <div className="flex flex-col gap-3">
              <CopyField label="Código de convite" value={invite.token} />
              <p className="text-xs text-muted-foreground">
                Expira em <span className="text-foreground">{formatDateTime(invite.expires_at)}</span> — copie agora, não
                será exibido de novo.
              </p>
              <div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
                <Laptop className="size-4 shrink-0" />
                <span>
                  No app: servidor <code className="rounded bg-muted px-1.5 py-0.5 text-foreground">https://xvpn.ihuull.com</code>
                  , cole o código e escolha um nome (ex.: notebook-casa).
                </span>
              </div>
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="w-fit gap-2"
                onClick={() => {
                  navigator.clipboard.writeText(invite.token)
                  toast.success('Código copiado')
                }}
              >
                <Copy className="size-4" />
                Copiar código
              </Button>
            </div>
          </div>
        )}

        {!invite && !generating && (
          <p className="text-sm text-muted-foreground">
            Ainda não tem o app? Baixe na <Link to="/my/marketplace" className="text-primary underline-offset-4 hover:underline">loja</Link> depois de gerar o convite.
          </p>
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
