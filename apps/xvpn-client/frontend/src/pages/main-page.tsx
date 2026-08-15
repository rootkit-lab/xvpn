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
  Share2,
} from 'lucide-react'

import type { StatusView } from '../../bindings/github.com/rootkit-lab/xvpn/client'
import { VPNService } from '../../bindings/github.com/rootkit-lab/xvpn/client'
import { ConnectionRings } from '@/components/connection-rings'
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

  const statusLabel = status.reconnecting
    ? 'Reconectando'
    : status.connected
      ? 'Protegido'
      : 'Desligado'

  const elapsed = status.connectedSince ? formatElapsedSince(status.connectedSince) : '00:00'

  return (
    <div className="watch-face relative flex h-full flex-col overflow-hidden px-5 pb-5 pt-4">
      <div className="watch-vignette pointer-events-none absolute inset-0" aria-hidden="true" />

      <header className="relative z-10 flex items-center justify-between">
        <div className="flex items-center gap-2.5">
          <img src="/logo-192.png" alt="" className="size-7 rounded-full" />
          <span className="font-display text-[17px] font-semibold tracking-tight">XVPN</span>
        </div>
        <div className="flex items-center gap-1">
          <span
            className={`mr-1 inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px] font-medium ${
              status.connected
                ? 'bg-primary/15 text-primary'
                : status.reconnecting
                  ? 'bg-amber-500/15 text-amber-400'
                  : 'bg-white/5 text-muted-foreground'
            }`}
          >
            <span
              className={`size-1.5 rounded-full ${
                status.connected ? 'bg-primary shadow-[0_0_8px_var(--glow)]' : status.reconnecting ? 'bg-amber-400' : 'bg-muted-foreground/50'
              }`}
            />
            {statusLabel}
          </span>
          <IconBtn onClick={onOpenApps} label="Apps" title="Marketplace">
            <Store className="h-4 w-4" />
          </IconBtn>
          <IconBtn onClick={onOpenDiagnostics} label="Diagnóstico">
            <Stethoscope className="h-4 w-4" />
          </IconBtn>
          <IconBtn onClick={onOpenSettings} label="Preferências">
            <Settings className="h-4 w-4" />
          </IconBtn>
        </div>
      </header>

      {/* Face central — tipografia + anéis + botão */}
      <div className="relative z-10 flex min-h-0 flex-1 flex-col items-center justify-center">
        <AnimatePresence mode="wait">
          {status.connected ? (
            <motion.div
              key="on"
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -6 }}
              className="mb-1 flex flex-col items-center"
            >
              <p className="font-display text-[11px] font-medium uppercase tracking-[0.18em] text-muted-foreground/80">
                Conexão segura
              </p>
              <p className="font-display text-[44px] font-semibold leading-none tracking-tight tabular-nums text-foreground">
                {elapsed}
              </p>
            </motion.div>
          ) : (
            <motion.div
              key="off"
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -6 }}
              className="mb-1 flex flex-col items-center"
            >
              <p className="font-display text-[11px] font-medium uppercase tracking-[0.18em] text-muted-foreground/80">
                {status.reconnecting ? 'Reconectando' : 'Pronto'}
              </p>
              <p className="font-display text-[44px] font-semibold leading-none tracking-tight text-muted-foreground/35">
                —:—
              </p>
            </motion.div>
          )}
        </AnimatePresence>

        <div className="relative mt-1 flex size-[200px] items-center justify-center">
          <ConnectionRings
            className="absolute inset-0"
            active={status.connected && !busy}
            reconnecting={status.reconnecting}
          />
          <motion.button
            type="button"
            onClick={toggle}
            disabled={busy}
            aria-label={status.connected ? 'Desconectar' : 'Conectar'}
            whileTap={{ scale: 0.92 }}
            whileHover={busy ? undefined : { scale: 1.04 }}
            transition={{ type: 'spring', stiffness: 420, damping: 28 }}
            className={`relative z-10 flex size-[88px] cursor-pointer items-center justify-center rounded-full transition-colors duration-300 disabled:cursor-wait ${
              status.connected
                ? 'bg-primary text-primary-foreground shadow-[0_8px_32px_-4px_color-mix(in_oklch,var(--glow)_70%,transparent)]'
                : status.reconnecting
                  ? 'bg-amber-500/90 text-black shadow-[0_8px_28px_-6px_var(--glow-amber)]'
                  : 'bg-white/10 text-foreground backdrop-blur-md hover:bg-white/16'
            }`}
          >
            <AnimatePresence mode="wait">
              {busy ? (
                <motion.div key="busy" initial={{ opacity: 0, scale: 0.8 }} animate={{ opacity: 1, scale: 1 }} exit={{ opacity: 0, scale: 0.8 }}>
                  <Loader2 className="h-8 w-8 animate-spin" strokeWidth={2.25} />
                </motion.div>
              ) : (
                <motion.div key="power" initial={{ opacity: 0, scale: 0.8 }} animate={{ opacity: 1, scale: 1 }} exit={{ opacity: 0, scale: 0.8 }}>
                  <Power className="h-8 w-8" strokeWidth={2.25} />
                </motion.div>
              )}
            </AnimatePresence>
          </motion.button>
        </div>

        <p className="mt-3 font-display text-[13px] text-muted-foreground">
          {busy
            ? 'Aplicando…'
            : status.reconnecting
              ? 'Restaurando o túnel…'
              : status.connected
                ? 'Toque para desligar'
                : 'Toque para ligar'}
        </p>

        {status.killSwitchActive && (
          <p className="mt-1.5 flex items-center gap-1 font-display text-[11px] text-muted-foreground/80">
            <ShieldCheck className="h-3.5 w-3.5" />
            Kill switch ativo
          </p>
        )}
      </div>

      {(error || actionError) && (
        <p className="relative z-10 mb-2 text-center font-display text-[13px] text-destructive">{actionError ?? error}</p>
      )}

      {status.connected && (
        <div className="relative z-10 space-y-3">
          <div className="grid grid-cols-2 gap-2">
            <Complication label="IP" value={shortIP(status.assignedIP)} />
            <Complication label="Servidor" value={shortEndpoint(status.serverEndpoint)} />
            <Complication
              label="Handshake"
              value={status.lastHandshake ? formatRelativeTime(status.lastHandshake) : '—'}
            />
            <Complication
              label="Tráfego"
              value={`${formatBytes(status.receiveBytes)} ↓`}
              icon={<ArrowDown className="h-3 w-3 opacity-70" />}
              secondary={`${formatBytes(status.transmitBytes)} ↑`}
              secondaryIcon={<ArrowUp className="h-3 w-3 opacity-70" />}
            />
          </div>

          <div className="flex justify-center gap-5 pt-1">
            <AppSlot
              onClick={() => openFiles('smb-home')}
              disabled={!sambaReady}
              title={sambaHint ?? 'Meus arquivos'}
              label="Arquivos"
              icon={<FolderOpen className="h-5 w-5" />}
            />
            <AppSlot
              onClick={() => openFiles('smb-shared')}
              disabled={!sambaReady}
              title={sambaHint ?? 'Compartilhado'}
              label="Shared"
              icon={<Share2 className="h-5 w-5" />}
            />
            <AppSlot
              onClick={() => openFiles('filebrowser')}
              title="Navegador web"
              label="Browser"
              icon={<Globe className="h-5 w-5" />}
            />
          </div>

          {!status.sambaEnabled && (
            <p className="text-center font-display text-[11px] text-muted-foreground">
              Samba desabilitado para {status.username || 'seu usuário'}
            </p>
          )}
        </div>
      )}
    </div>
  )
}

function IconBtn({
  children,
  onClick,
  label,
  title,
}: {
  children: ReactNode
  onClick: () => void
  label: string
  title?: string
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      title={title ?? label}
      className="flex size-8 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-white/8 hover:text-foreground"
    >
      {children}
    </button>
  )
}

function Complication({
  label,
  value,
  icon,
  secondary,
  secondaryIcon,
}: {
  label: string
  value: string
  icon?: ReactNode
  secondary?: string
  secondaryIcon?: ReactNode
}) {
  return (
    <div className="watch-complication rounded-[18px] px-3.5 py-2.5">
      <p className="font-display text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground/75">{label}</p>
      <p className="mt-0.5 flex items-center gap-1 font-display text-[13px] font-semibold tabular-nums tracking-tight">
        {icon}
        <span className="truncate">{value}</span>
      </p>
      {secondary && (
        <p className="mt-0.5 flex items-center gap-1 font-display text-[12px] tabular-nums text-muted-foreground">
          {secondaryIcon}
          {secondary}
        </p>
      )}
    </div>
  )
}

function AppSlot({
  onClick,
  disabled,
  title,
  label,
  icon,
}: {
  onClick: () => void
  disabled?: boolean
  title: string
  label: string
  icon: ReactNode
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      title={title}
      className="group flex flex-col items-center gap-1.5 disabled:cursor-not-allowed disabled:opacity-40"
    >
      <span className="flex size-12 items-center justify-center rounded-[16px] bg-gradient-to-b from-white/14 to-white/6 text-foreground shadow-[inset_0_1px_0_color-mix(in_oklch,white_18%,transparent)] transition-transform group-hover:scale-105 group-active:scale-95">
        {icon}
      </span>
      <span className="font-display text-[10px] font-medium text-muted-foreground">{label}</span>
    </button>
  )
}

function shortIP(ip: string) {
  // 10.66.66.2/32 → 10.66.66.2
  return ip.replace(/\/\d+$/, '')
}

function shortEndpoint(ep: string) {
  // host:port — keep compact
  if (ep.length <= 18) return ep
  const [host, port] = ep.split(':')
  if (!port) return ep.slice(0, 16) + '…'
  return `${host.slice(0, 10)}…:${port}`
}
