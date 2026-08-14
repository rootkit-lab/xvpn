import { useEffect, useState, type ReactNode } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import {
  Power,
  ArrowDown,
  ArrowUp,
  Loader2,
  FolderOpen,
  Globe,
  Settings,
  Stethoscope,
  ShieldCheck,
} from 'lucide-react'

import type { StatusView } from '../../bindings/github.com/rootkit-lab/xvpn/client'
import { VPNService } from '../../bindings/github.com/rootkit-lab/xvpn/client'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { NetworkGlobe } from '@/components/network-globe'
import { formatBytes, formatElapsedSince, formatRelativeTime } from '@/lib/format'

interface MainPageProps {
  status: StatusView
  onChange: () => void
  error: string | null
  onOpenSettings: () => void
  onOpenDiagnostics: () => void
}

export function MainPage({ status, onChange, error, onOpenSettings, onOpenDiagnostics }: MainPageProps) {
  const [busy, setBusy] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)
  // Re-renderiza 1x/s só pra o timer "conectado há" contar em tempo real,
  // sem depender do intervalo de polling do status (2s, ver App.tsx) —
  // formatElapsedSince recalcula a partir de status.connectedSince a cada
  // chamada, então um tick local aqui já é suficiente.
  const [, setTick] = useState(0)

  useEffect(() => {
    if (!status.connected) return
    const id = setInterval(() => setTick((t) => t + 1), 1000)
    return () => clearInterval(id)
  }, [status.connected])

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

  async function openFiles(kind: 'smb' | 'filebrowser') {
    setActionError(null)
    try {
      await VPNService.OpenServerFiles(kind)
    } catch (err) {
      setActionError(err instanceof Error ? err.message : String(err))
    }
  }

  const glowVar = status.reconnecting ? 'var(--glow-amber)' : status.connected ? 'var(--glow)' : undefined

  return (
    <div className="dot-grid relative flex h-full flex-col gap-4 overflow-y-auto p-6">
      <div className="glow-blob pointer-events-none absolute -top-24 left-1/2 h-72 w-72 -translate-x-1/2" />

      <header className="relative z-10 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <img src="/logo-192.png" alt="" className="size-6 drop-shadow-[0_0_10px_var(--color-glow)]" />
          <h1 className="text-lg font-semibold tracking-tight">XVPN</h1>
        </div>
        <div className="flex items-center gap-2">
          <Badge
            variant={status.connected ? 'default' : 'outline'}
            className="rounded-md font-mono tracking-wide"
          >
            {status.reconnecting
              ? `Reconectando (${status.reconnectAttempt + 1})`
              : status.connected
                ? 'Conectado'
                : 'Desconectado'}
          </Badge>
          <button
            onClick={onOpenDiagnostics}
            aria-label="Diagnóstico"
            className="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
          >
            <Stethoscope className="h-4 w-4" />
          </button>
          <button
            onClick={onOpenSettings}
            aria-label="Preferências"
            className="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
          >
            <Settings className="h-4 w-4" />
          </button>
        </div>
      </header>

      <div className="relative z-10 flex flex-1 flex-col items-center justify-center gap-3">
        <NetworkGlobe className="pointer-events-none absolute inset-x-0 top-1/2 z-0 h-56 w-full -translate-y-1/2 opacity-60" />

        {status.connected && (
          <motion.div
            initial={{ opacity: 0, y: -6 }}
            animate={{ opacity: 1, y: 0 }}
            className="relative z-10 flex flex-col items-center gap-1.5"
          >
            <div className="flex items-center gap-2">
              <span className="cyber-diamond size-2 bg-primary" />
              <span className="hud-label text-muted-foreground/80">Conexão segura</span>
            </div>
            <p className="font-mono text-3xl font-semibold tabular-nums text-glow">
              {status.connectedSince ? formatElapsedSince(status.connectedSince) : '--:--'}
            </p>
          </motion.div>
        )}

        <div className="relative z-20 flex size-36 items-center justify-center">
          {/* Anel decorativo: pointer-events-none obrigatório — com
              animate-spin (transform) o SVG cria stacking context e, no
              WebKitGTK do Wails, interceptava cliques no botão por baixo. */}
          <svg
            viewBox="0 0 100 100"
            className="pointer-events-none absolute inset-0 h-full w-full animate-spin-slow text-primary/40"
            aria-hidden="true"
          >
            <circle
              cx="50"
              cy="50"
              r="47"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeDasharray="2 8"
              strokeLinecap="round"
            />
          </svg>
          <motion.button
            type="button"
            onClick={toggle}
            disabled={busy}
            aria-label={status.connected ? 'Desconectar' : 'Conectar'}
            whileTap={{ scale: 0.94 }}
            animate={status.connected && !busy ? { scale: [1, 1.02, 1] } : { scale: 1 }}
            transition={status.connected ? { duration: 2.6, repeat: Infinity, ease: 'easeInOut' } : undefined}
            className={`relative z-10 flex h-32 w-32 cursor-pointer items-center justify-center rounded-full border-4 transition-colors disabled:cursor-wait disabled:opacity-60 ${
              status.connected
                ? 'animate-pulse-glow border-primary bg-primary/10 text-primary'
                : status.reconnecting
                  ? 'border-amber-500/60 bg-amber-500/10 text-amber-400'
                  : 'border-border bg-secondary text-muted-foreground hover:border-primary/50'
            }`}
            style={glowVar ? ({ '--glow': glowVar } as React.CSSProperties) : undefined}
          >
            <AnimatePresence mode="wait">
              {busy ? (
                <motion.div key="busy" initial={{ opacity: 0, scale: 0.7 }} animate={{ opacity: 1, scale: 1 }} exit={{ opacity: 0, scale: 0.7 }}>
                  <Loader2 className="h-10 w-10 animate-spin" />
                </motion.div>
              ) : (
                <motion.div key="power" initial={{ opacity: 0, scale: 0.7 }} animate={{ opacity: 1, scale: 1 }} exit={{ opacity: 0, scale: 0.7 }}>
                  <Power className="h-10 w-10" />
                </motion.div>
              )}
            </AnimatePresence>
          </motion.button>
        </div>

        <p className="relative z-10 text-sm text-muted-foreground">
          {busy
            ? 'Aplicando…'
            : status.reconnecting
              ? 'Túnel caiu, tentando reconectar automaticamente…'
              : status.connected
                ? 'Toque para desconectar'
                : 'Toque para conectar'}
        </p>
        {status.killSwitchActive && (
          <p className="relative z-10 flex items-center gap-1 text-xs text-muted-foreground">
            <ShieldCheck className="h-3.5 w-3.5" />
            Kill switch ativo — tráfego fora da VPN bloqueado
          </p>
        )}
      </div>

      {(error || actionError) && (
        <p className="relative z-10 text-center text-sm text-destructive">{actionError ?? error}</p>
      )}

      {status.connected && (
        <Card className="cyber-frame relative z-10 border-white/5 bg-card/70">
          <CardContent className="grid grid-cols-2 gap-3 p-4 text-sm">
            <InfoItem label="IP atribuído" value={status.assignedIP} />
            <InfoItem label="Servidor" value={status.serverEndpoint} />
            <InfoItem
              label="Último handshake"
              value={status.lastHandshake ? formatRelativeTime(status.lastHandshake) : '—'}
            />
            <InfoItem
              label="Recebido"
              value={formatBytes(status.receiveBytes)}
              icon={<ArrowDown className="h-3.5 w-3.5 text-primary" />}
            />
            <InfoItem
              label="Enviado"
              value={formatBytes(status.transmitBytes)}
              icon={<ArrowUp className="h-3.5 w-3.5 text-primary" />}
            />
          </CardContent>
        </Card>
      )}

      {status.connected && (
        <div className="relative z-10 flex gap-3">
          <button
            onClick={() => openFiles('smb')}
            className="flex flex-1 items-center justify-center gap-2 rounded-md border border-border bg-secondary px-3 py-2.5 font-mono text-sm font-medium tracking-wide text-secondary-foreground transition-all hover:border-primary/40 hover:bg-secondary/80 hover:shadow-[0_0_20px_-6px_var(--color-glow)]"
          >
            <FolderOpen className="h-4 w-4" />
            Unidade de rede
          </button>
          <button
            onClick={() => openFiles('filebrowser')}
            className="flex flex-1 items-center justify-center gap-2 rounded-md border border-border bg-secondary px-3 py-2.5 font-mono text-sm font-medium tracking-wide text-secondary-foreground transition-all hover:border-primary/40 hover:bg-secondary/80 hover:shadow-[0_0_20px_-6px_var(--color-glow)]"
          >
            <Globe className="h-4 w-4" />
            Arquivos (navegador)
          </button>
        </div>
      )}
    </div>
  )
}

function InfoItem({ label, value, icon }: { label: string; value: string; icon?: ReactNode }) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="hud-label text-muted-foreground/70">{label}</span>
      <span className="flex items-center gap-1 font-mono font-medium">
        {icon}
        {value}
      </span>
    </div>
  )
}
