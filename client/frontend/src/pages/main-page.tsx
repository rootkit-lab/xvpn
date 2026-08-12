import { useState, type ReactNode } from 'react'
import { Power, ArrowDown, ArrowUp, Loader2 } from 'lucide-react'

import type { StatusView } from '../../bindings/github.com/rootkit-lab/xvpn/client'
import { VPNService } from '../../bindings/github.com/rootkit-lab/xvpn/client'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { formatBytes, formatElapsedSince, formatRelativeTime } from '@/lib/format'

interface MainPageProps {
  status: StatusView
  onChange: () => void
  error: string | null
}

export function MainPage({ status, onChange, error }: MainPageProps) {
  const [busy, setBusy] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)

  async function toggle() {
    setBusy(true)
    setActionError(null)
    try {
      if (status.connected) {
        await VPNService.Disconnect()
      } else {
        await VPNService.Connect()
      }
      onChange()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex h-full flex-col gap-6 p-6">
      <header className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">XVPN</h1>
        <Badge variant={status.connected ? 'default' : 'outline'}>
          {status.connected ? 'Conectado' : 'Desconectado'}
        </Badge>
      </header>

      <div className="flex flex-1 flex-col items-center justify-center gap-4">
        <button
          onClick={toggle}
          disabled={busy}
          aria-label={status.connected ? 'Desconectar' : 'Conectar'}
          className={`flex h-32 w-32 items-center justify-center rounded-full border-4 transition-colors disabled:opacity-60 ${
            status.connected
              ? 'border-primary bg-primary/10 text-primary'
              : 'border-border bg-secondary text-muted-foreground hover:border-primary/50'
          }`}
        >
          {busy ? <Loader2 className="h-10 w-10 animate-spin" /> : <Power className="h-10 w-10" />}
        </button>
        <p className="text-sm text-muted-foreground">
          {busy ? 'Aplicando…' : status.connected ? 'Toque para desconectar' : 'Toque para conectar'}
        </p>
      </div>

      {(error || actionError) && (
        <p className="text-center text-sm text-destructive">{actionError ?? error}</p>
      )}

      {status.connected && (
        <Card>
          <CardContent className="grid grid-cols-2 gap-3 p-4 text-sm">
            <InfoItem label="IP atribuído" value={status.assignedIP} />
            <InfoItem label="Servidor" value={status.serverEndpoint} />
            <InfoItem
              label="Conectado há"
              value={status.connectedSince ? formatElapsedSince(status.connectedSince) : '—'}
            />
            <InfoItem
              label="Último handshake"
              value={status.lastHandshake ? formatRelativeTime(status.lastHandshake) : '—'}
            />
            <InfoItem
              label="Recebido"
              value={formatBytes(status.receiveBytes)}
              icon={<ArrowDown className="h-3.5 w-3.5" />}
            />
            <InfoItem
              label="Enviado"
              value={formatBytes(status.transmitBytes)}
              icon={<ArrowUp className="h-3.5 w-3.5" />}
            />
          </CardContent>
        </Card>
      )}
    </div>
  )
}

function InfoItem({ label, value, icon }: { label: string; value: string; icon?: ReactNode }) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-xs text-muted-foreground">{label}</span>
      <span className="flex items-center gap-1 font-medium">
        {icon}
        {value}
      </span>
    </div>
  )
}
