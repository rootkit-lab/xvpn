import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { Bell, Mic, Shield, Volume2, X } from 'lucide-react'
import { ChatButton, ChatRoot } from '@chat/messenger/ui'
import { cn } from '@chat/lib/utils'

const KEY = 'xvpn-chat-settings'

export type ChatSettings = {
  notify: boolean
  soundIn: boolean
  soundOut: boolean
  soundCall: boolean
  volume: number
  micId: string
  readReceipts: boolean
  sendTyping: boolean
  sharePresence: boolean
}

const DEFAULTS: ChatSettings = {
  notify: true,
  soundIn: true,
  soundOut: true,
  soundCall: true,
  volume: 0.7,
  micId: '',
  readReceipts: true,
  sendTyping: true,
  sharePresence: true,
}

function readSettings(): ChatSettings {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return DEFAULTS
    const parsed = JSON.parse(raw) as Partial<ChatSettings>
    return { ...DEFAULTS, ...parsed, volume: Math.min(1, Math.max(0, Number(parsed.volume ?? DEFAULTS.volume))) }
  } catch {
    return DEFAULTS
  }
}

type SettingsCtx = {
  settings: ChatSettings
  patch: (partial: Partial<ChatSettings>) => void
  open: boolean
  setOpen: (v: boolean) => void
}

const Ctx = createContext<SettingsCtx | null>(null)

export function ChatSettingsProvider({ children }: { children: ReactNode }) {
  const [settings, setSettings] = useState<ChatSettings>(readSettings)
  const [open, setOpen] = useState(false)

  const patch = useCallback((partial: Partial<ChatSettings>) => {
    setSettings((prev) => {
      const next = { ...prev, ...partial }
      try {
        localStorage.setItem(KEY, JSON.stringify(next))
      } catch {
        // storage indisponível
      }
      return next
    })
  }, [])

  const value = useMemo(() => ({ settings, patch, open, setOpen }), [settings, patch, open])
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

export function useChatSettings(): SettingsCtx {
  const ctx = useContext(Ctx)
  if (!ctx) throw new Error('useChatSettings fora do ChatSettingsProvider')
  return ctx
}

function Toggle({
  label,
  hint,
  checked,
  onChange,
}: {
  label: string
  hint?: string
  checked: boolean
  onChange: (v: boolean) => void
}) {
  return (
    <label className="flex items-start justify-between gap-3 py-2">
      <span>
        <span className="block font-display text-sm">{label}</span>
        {hint && <span className="block text-[11px] text-muted-foreground">{hint}</span>}
      </span>
      <input
        type="checkbox"
        className="mt-1 size-4 accent-[var(--safe)]"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
      />
    </label>
  )
}

export function ChatSettingsPanel({ theme }: { theme: string }) {
  const { settings, patch, open, setOpen } = useChatSettings()
  const [mics, setMics] = useState<MediaDeviceInfo[]>([])
  const [perm, setPerm] = useState(typeof Notification === 'undefined' ? 'unsupported' : Notification.permission)
  const [tab, setTab] = useState<'notify' | 'audio' | 'privacy'>('notify')

  useEffect(() => {
    if (!open) return
    void navigator.mediaDevices
      ?.enumerateDevices()
      .then((devs) => setMics(devs.filter((d) => d.kind === 'audioinput')))
      .catch(() => setMics([]))
  }, [open])

  if (!open) return null

  return (
    <ChatRoot theme={theme} className="fixed inset-0 z-[50] flex items-center justify-center bg-black/65 p-4">
      <div className="relative w-full max-w-md rounded-[22px] p-5 watch-complication">
        <div className="mb-3 flex items-center justify-between">
          <h2 className="font-display text-lg font-semibold">Configurações</h2>
          <button
            type="button"
            className="inline-flex size-8 items-center justify-center rounded-[10px] text-muted-foreground hover:bg-white/10 hover:text-foreground"
            aria-label="Fechar configurações"
            onClick={() => setOpen(false)}
          >
            <X className="size-4" />
          </button>
        </div>
        <div className="mb-3 flex gap-1">
          {(
            [
              ['notify', 'Avisos', Bell],
              ['audio', 'Áudio', Volume2],
              ['privacy', 'Privacidade', Shield],
            ] as const
          ).map(([id, label, Icon]) => (
            <button
              key={id}
              type="button"
              className={cn(
                'flex flex-1 items-center justify-center gap-1.5 rounded-[12px] px-2 py-1.5 font-display text-[12px]',
                tab === id ? 'bg-white/12 text-[var(--safe)]' : 'text-muted-foreground hover:bg-white/8',
              )}
              onClick={() => setTab(id)}
            >
              <Icon className="size-3.5" />
              {label}
            </button>
          ))}
        </div>

        {tab === 'notify' && (
          <div>
            <Toggle
              label="Notificações do sistema"
              hint="Aviso na bandeja quando a janela não está em foco"
              checked={settings.notify}
              onChange={(v) => patch({ notify: v })}
            />
            <Toggle label="Som de mensagem recebida" checked={settings.soundIn} onChange={(v) => patch({ soundIn: v })} />
            <Toggle label="Som de mensagem enviada" checked={settings.soundOut} onChange={(v) => patch({ soundOut: v })} />
            <Toggle label="Toque de chamada" checked={settings.soundCall} onChange={(v) => patch({ soundCall: v })} />
            <label className="mt-2 block font-display text-[10px] uppercase tracking-[0.14em] text-muted-foreground/75">
              Volume
            </label>
            <input
              type="range"
              min={0}
              max={1}
              step={0.05}
              value={settings.volume}
              onChange={(e) => patch({ volume: Number(e.target.value) })}
              className="mt-1 w-full accent-[var(--safe)]"
            />
            {perm !== 'unsupported' && perm !== 'granted' && (
              <ChatButton
                variant="safe"
                className="mt-3 w-full"
                onClick={() => {
                  void Notification.requestPermission().then((p) => setPerm(p))
                }}
              >
                Permitir notificações
              </ChatButton>
            )}
            {perm === 'granted' && <p className="mt-2 text-[11px] text-[var(--safe)]">Notificações permitidas neste dispositivo.</p>}
          </div>
        )}

        {tab === 'audio' && (
          <div>
            <label className="mb-1 flex items-center gap-1.5 font-display text-[10px] uppercase tracking-[0.14em] text-muted-foreground/75">
              <Mic className="size-3" />
              Microfone
            </label>
            <select
              className="h-10 w-full rounded-[14px] border-0 bg-foreground/[0.06] px-3 text-sm"
              value={settings.micId}
              onChange={(e) => patch({ micId: e.target.value })}
            >
              <option value="">Padrão do sistema</option>
              {mics.map((d) => (
                <option key={d.deviceId} value={d.deviceId}>
                  {d.label || `Microfone ${d.deviceId.slice(0, 6)}`}
                </option>
              ))}
            </select>
            <ChatButton
              variant="outline"
              className="mt-2 w-full"
              onClick={() => {
                void navigator.mediaDevices
                  .getUserMedia({ audio: true })
                  .then((s) => {
                    s.getTracks().forEach((t) => t.stop())
                    return navigator.mediaDevices.enumerateDevices()
                  })
                  .then((devs) => setMics(devs.filter((d) => d.kind === 'audioinput')))
                  .catch(() => {})
              }}
            >
              Liberar microfone e listar aparelhos
            </ChatButton>
            <label className="mt-3 block font-display text-[10px] uppercase tracking-[0.14em] text-muted-foreground/75">
              Volume dos sons
            </label>
            <input
              type="range"
              min={0}
              max={1}
              step={0.05}
              value={settings.volume}
              onChange={(e) => patch({ volume: Number(e.target.value) })}
              className="mt-1 w-full accent-[var(--safe)]"
            />
          </div>
        )}

        {tab === 'privacy' && (
          <div>
            <Toggle
              label="Confirmação de leitura"
              hint="Envia e mostra os ticks azuis quando a mensagem foi lida"
              checked={settings.readReceipts}
              onChange={(v) => patch({ readReceipts: v })}
            />
            <Toggle
              label="Mostrar que estou digitando"
              checked={settings.sendTyping}
              onChange={(v) => patch({ sendTyping: v })}
            />
            <Toggle
              label="Compartilhar presença"
              hint="Desligado: você aparece como invisível"
              checked={settings.sharePresence}
              onChange={(v) => patch({ sharePresence: v })}
            />
          </div>
        )}
      </div>
    </ChatRoot>
  )
}

export function audioConstraints(micId: string): MediaTrackConstraints {
  return micId ? { deviceId: { exact: micId } } : { echoCancellation: true }
}
