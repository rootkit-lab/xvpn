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
  Store,
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
  onOpenApps: () => void
}

export function MainPage({ status, onChange, error, onOpenSettings, onOpenDiagnostics, onOpenApps }: MainPageProps) {
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

  async function openFiles(kind: 'smb-home' | 'smb-shared' | 'filebrowser') {
    setActionError(null)
    try {
      await VPNService.OpenServerFiles(kind)
    } catch (err) {
      setActionError(err instanceof Error ? err.message : String(err))
    }
  }

  const sambaReady = status.connected && status.sambaEnabled
  const sambaHint = !status.connected
    ? 'Conecte-se à VPN para abrir os arquivos do servidor'
    : !status.sambaEnabled
      ? 'Seu usuário ainda não tem Samba habilitado no painel'
      : undefined

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
            onClick={onOpenApps}
            aria-label="Apps"
            title="Marketplace de programas"
            className="rounded-full p-1.5 text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
          >
            <Store className="h-4 w-4" />
          </button>
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

        <div className="relative z-20 flex size-40 items-center justify-center">
          {/* Halos suaves (blur) — evitam o aliasing do anel SVG único. */}
          <div
            className={`pointer-events-none absolute inset-0 rounded-full transition-opacity ${
              status.connected ? 'opacity-100' : 'opacity-40'
            }`}
            aria-hidden="true"
          >
            <div className="absolute inset-[-18%] rounded-full bg-[radial-gradient(circle,color-mix(in_oklch,var(--glow)_40%,transparent)_0%,transparent_70%)] blur-xl" />
            <div className="absolute inset-2 rounded-full border border-primary/20" />
            <div className="absolute inset-0 rounded-full border border-primary/10" />
          </div>
          <svg
            viewBox="0 0 100 100"
            className="pointer-events-none absolute inset-0 h-full w-full animate-spin-slow text-primary/50"
            aria-hidden="true"
            shapeRendering="geometricPrecision"
          >
            <circle
              cx="50"
              cy="50"
              r="46.5"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.25"
              strokeDasharray="1.5 7"
              strokeLinecap="round"
              vectorEffect="non-scaling-stroke"
            />
          </svg>
          <svg
            viewBox="0 0 100 100"
            className="pointer-events-none absolute inset-[6%] h-[88%] w-[88%] animate-spin-slow text-primary/25"
            style={{ animationDirection: 'reverse', animationDuration: '18s' }}
            aria-hidden="true"
            shapeRendering="geometricPrecision"
          >
            <circle
              cx="50"
              cy="50"
              r="46"
              fill="none"
              stroke="currentColor"
              strokeWidth="0.8"
              strokeDasharray="12 10"
              strokeLinecap="round"
              vectorEffect="non-scaling-stroke"
            />
          </svg>
          <motion.button
            type="button"
            onClick={toggle}
            disabled={busy}
            aria-label={status.connected ? 'Desconectar' : 'Conectar'}
            whileTap={{ scale: 0.94 }}
            whileHover={busy ? undefined : { scale: 1.03 }}
            animate={status.connected && !busy ? { scale: [1, 1.025, 1] } : { scale: 1 }}
            transition={status.connected ? { duration: 2.8, repeat: Infinity, ease: 'easeInOut' } : { type: 'spring', stiffness: 380, damping: 24 }}
            className={`power-btn relative z-10 flex h-[7.25rem] w-[7.25rem] cursor-pointer items-center justify-center rounded-full border-[3px] transition-[border-color,background-color,color,box-shadow] duration-300 disabled:cursor-wait disabled:opacity-60 ${
              status.connected
                ? 'power-btn--on border-primary bg-primary/15 text-primary'
                : status.reconnecting
                  ? 'border-amber-500/70 bg-amber-500/10 text-amber-400 shadow-[0_0_28px_-6px_var(--glow-amber)]'
                  : 'border-border/80 bg-secondary/90 text-muted-foreground hover:border-primary/55 hover:bg-secondary hover:text-primary'
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
                  <Power className="h-10 w-10 drop-shadow-[0_0_12px_color-mix(in_oklch,var(--glow)_55%,transparent)]" strokeWidth={1.75} />
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
        <div className="relative z-10 flex flex-col gap-2">
          <div className="grid grid-cols-3 gap-2">
            <button
              onClick={() => openFiles('smb-home')}
              disabled={!sambaReady}
              title={sambaHint ?? 'Meus arquivos (Samba)'}
              className="file-action-btn"
            >
              <FolderOpen className="h-4 w-4 shrink-0 opacity-90" />
              <span className="w-full truncate text-center leading-tight">Meus arquivos</span>
            </button>
            <button
              onClick={() => openFiles('smb-shared')}
              disabled={!sambaReady}
              title={sambaHint ?? 'Compartilhado'}
              className="file-action-btn"
            >
              <FolderOpen className="h-4 w-4 shrink-0 opacity-90" />
              <span className="w-full truncate text-center leading-tight">Compartilhado</span>
            </button>
            <button
              onClick={() => openFiles('filebrowser')}
              title="Navegador de arquivos (web)"
              className="file-action-btn"
            >
              <Globe className="h-4 w-4 shrink-0 opacity-90" />
              <span className="w-full truncate text-center leading-tight">Navegador</span>
            </button>
          </div>
          {status.connected && !status.sambaEnabled && (
            <p className="text-center text-xs text-muted-foreground">
              Samba desabilitado para {status.username || 'seu usuário'} — peça ao admin no painel.
            </p>
          )}
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
