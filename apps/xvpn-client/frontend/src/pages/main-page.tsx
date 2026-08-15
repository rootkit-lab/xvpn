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
import { CredentialPrompt, saveLastUsername } from '@/components/credential-prompt'
import { WatchIconButton, WatchShell } from '@/components/watch-chrome'
import { formatBytes, formatElapsedSince, formatRelativeTime } from '@/lib/format'

/** Evento emitido pela bandeja quando Conectar exige login na janela. */
export const REQUEST_CONNECT_AUTH_EVENT = 'xvpn:request-connect-auth'

interface MainPageProps {
  status: StatusView
  onChange: () => void
  error: string | null
  /** Incrementado pelo App quando a bandeja pede o sheet de credenciais. */
  connectAuthNonce?: number
  onOpenSettings: () => void
  onOpenDiagnostics: () => void
  onOpenApps: () => void
}

export function MainPage({
  status,
  onChange,
  error,
  connectAuthNonce = 0,
  onOpenSettings,
  onOpenDiagnostics,
  onOpenApps,
}: MainPageProps) {
  const [busy, setBusy] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)
  const [authOpen, setAuthOpen] = useState(false)
  const [authSubmitting, setAuthSubmitting] = useState(false)
  const [authError, setAuthError] = useState<string | null>(null)
  const [, setTick] = useState(0)

  useEffect(() => {
    if (!status.connected) return
    const id = setInterval(() => setTick((t) => t + 1), 1000)
    return () => clearInterval(id)
  }, [status.connected])

  // Túnel já up → sheet nunca pode ficar por cima da face conectada.
  useEffect(() => {
    if (!status.connected) return
    setAuthOpen(false)
    setAuthSubmitting(false)
    setAuthError(null)
  }, [status.connected])

  useEffect(() => {
    if (connectAuthNonce === 0 || status.connected || busy || authSubmitting) return
    setAuthError(null)
    setAuthOpen(true)
  }, [connectAuthNonce, status.connected, busy, authSubmitting])

  async function connectWithSession() {
    await VPNService.Connect()
    onChange()
  }

  async function toggle() {
    setActionError(null)
    if (status.connected) {
      setBusy(true)
      try {
        await VPNService.Disconnect()
        onChange()
      } catch (err) {
        setActionError(err instanceof Error ? err.message : String(err))
      } finally {
        setBusy(false)
      }
      return
    }

    if (authOpen || authSubmitting) return

    setBusy(true)
    try {
      const session = await VPNService.MarketplaceSessionStatus()
      if (session.loggedIn) {
        await connectWithSession()
        return
      }
      setAuthError(null)
      setAuthOpen(true)
    } catch (err) {
      setActionError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  async function handleAuthSubmit(username: string, password: string) {
    setAuthSubmitting(true)
    setAuthError(null)
    try {
      await VPNService.MarketplaceLogin({
        serverBaseURL: status.serverBaseURL,
        username,
        password,
      })
      saveLastUsername(username)
      // Fecha o sheet ANTES do Connect — evita face "Protegido" + modal
      // "Conectando…" sobrepostos enquanto o status atualiza.
      setAuthOpen(false)
      setBusy(true)
      await connectWithSession()
    } catch (err) {
      setAuthError(err instanceof Error ? err.message : String(err))
      // Mantém o sheet aberto só em falha de login/connect.
      setAuthOpen(true)
    } finally {
      setAuthSubmitting(false)
      setBusy(false)
    }
  }

  function cancelAuth() {
    if (authSubmitting) return
    setAuthOpen(false)
    setAuthError(null)
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

  const statusTone = status.connected
    ? 'text-safe'
    : status.reconnecting
      ? 'text-amber-400'
      : 'text-muted-foreground'

  const elapsed = status.connectedSince ? formatElapsedSince(status.connectedSince) : '00:00'

  return (
    <WatchShell>
      <CredentialPrompt
        open={authOpen}
        serverBaseURL={status.serverBaseURL}
        submitting={authSubmitting}
        error={authError}
        onCancel={cancelAuth}
        onSubmit={handleAuthSubmit}
      />

      <motion.div
        className="relative z-10 flex min-h-0 flex-1 flex-col"
        animate={{
          opacity: authOpen ? 0.35 : 1,
          scale: authOpen ? 0.985 : 1,
          filter: authOpen ? 'blur(2px)' : 'blur(0px)',
        }}
        transition={{ duration: 0.28, ease: [0.22, 1, 0.36, 1] }}
        style={{ pointerEvents: authOpen ? 'none' : 'auto' }}
      >
      <header className="flex items-center justify-between">
        <div className="flex items-center gap-2.5">
          <img
            src="/logo-192.png"
            alt=""
            className="size-7 rounded-[9px] shadow-[inset_0_1px_0_color-mix(in_oklch,white_20%,transparent)]"
          />
          <span className="font-display text-[17px] font-semibold tracking-tight">XVPN</span>
        </div>
        <div className="flex items-center gap-2">
          <WatchIconButton onClick={onOpenApps} label="Apps" title="Marketplace" filled>
            <Store className="h-4 w-4" strokeWidth={2} />
          </WatchIconButton>
          <WatchIconButton onClick={onOpenDiagnostics} label="Diagnóstico" filled>
            <Stethoscope className="h-4 w-4" strokeWidth={2} />
          </WatchIconButton>
          <WatchIconButton onClick={onOpenSettings} label="Preferências" filled>
            <Settings className="h-4 w-4" strokeWidth={2} />
          </WatchIconButton>
        </div>
      </header>

      <div className="flex min-h-0 flex-1 flex-col items-center justify-center">
        <AnimatePresence mode="wait">
          {status.connected ? (
            <motion.div
              key="on"
              initial={{ opacity: 0, y: 10, filter: 'blur(4px)' }}
              animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
              exit={{ opacity: 0, y: -8, filter: 'blur(4px)' }}
              transition={{ duration: 0.28, ease: [0.22, 1, 0.36, 1] }}
              className="mb-1 flex flex-col items-center"
            >
              <p className="font-display text-[52px] font-semibold leading-none tracking-tight tabular-nums text-foreground">
                {elapsed}
              </p>
              <p className={`mt-2 flex items-center gap-1.5 font-display text-[13px] font-medium ${statusTone}`}>
                <span className="status-safe-dot size-1.5 rounded-full" />
                {statusLabel}
              </p>
            </motion.div>
          ) : (
            <motion.div
              key="off"
              initial={{ opacity: 0, y: 10, filter: 'blur(4px)' }}
              animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
              exit={{ opacity: 0, y: -8, filter: 'blur(4px)' }}
              transition={{ duration: 0.28, ease: [0.22, 1, 0.36, 1] }}
              className="mb-1 flex flex-col items-center"
            >
              <p className="font-display text-[52px] font-semibold leading-none tracking-tight text-muted-foreground/30">
                —:—
              </p>
              <p className={`mt-2 flex items-center gap-1.5 font-display text-[13px] font-medium ${statusTone}`}>
                <span
                  className={`size-1.5 rounded-full ${
                    status.reconnecting ? 'bg-amber-400' : 'bg-muted-foreground/50'
                  }`}
                />
                {status.reconnecting ? 'Reconectando' : 'Pronto'}
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
            disabled={busy || authOpen || authSubmitting}
            aria-label={status.connected ? 'Desconectar' : 'Conectar'}
            whileTap={{ scale: 0.92 }}
            whileHover={busy || status.connected ? undefined : { scale: 1.04 }}
            transition={{ type: 'spring', stiffness: 420, damping: 28 }}
            className={`relative z-10 flex size-[88px] cursor-pointer items-center justify-center rounded-full transition-colors duration-300 disabled:cursor-wait ${
              status.connected
                ? 'power-safe'
                : status.reconnecting
                  ? 'bg-amber-500/90 text-black shadow-[0_8px_28px_-6px_var(--glow-amber)]'
                  : 'bg-white/10 text-foreground backdrop-blur-md hover:bg-white/16'
            }`}
          >
            <AnimatePresence mode="wait">
              {busy ? (
                <motion.div
                  key="busy"
                  initial={{ opacity: 0, scale: 0.8 }}
                  animate={{ opacity: 1, scale: 1 }}
                  exit={{ opacity: 0, scale: 0.8 }}
                  transition={{ duration: 0.15 }}
                >
                  <Loader2 className="h-8 w-8 animate-spin" strokeWidth={2.25} />
                </motion.div>
              ) : (
                <motion.div
                  key="power"
                  initial={{ opacity: 0, scale: 0.8 }}
                  animate={{ opacity: 1, scale: 1 }}
                  exit={{ opacity: 0, scale: 0.8 }}
                  transition={{ duration: 0.15 }}
                >
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
                : 'Toque para conectar'}
        </p>

        {status.killSwitchActive && (
          <p className="mt-1.5 flex items-center gap-1 font-display text-[11px] text-muted-foreground/80">
            <ShieldCheck className="h-3.5 w-3.5" />
            Kill switch ativo
          </p>
        )}
      </div>

      {(error || actionError) && (
        <p className="mb-2 text-center font-display text-[13px] text-destructive">{actionError ?? error}</p>
      )}

      <AnimatePresence>
        {status.connected && (
          <motion.div
            key="complications"
            initial={{ opacity: 0, y: 16 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 10 }}
            transition={{ duration: 0.32, ease: [0.22, 1, 0.36, 1] }}
            className="space-y-3"
          >
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
          </motion.div>
        )}
      </AnimatePresence>
      </motion.div>
    </WatchShell>
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
  return ip.replace(/\/\d+$/, '')
}

function shortEndpoint(ep: string) {
  if (ep.length <= 18) return ep
  const [host, port] = ep.split(':')
  if (!port) return ep.slice(0, 16) + '…'
  return `${host.slice(0, 10)}…:${port}`
}
