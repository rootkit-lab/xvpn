import { useCallback, useEffect, useState } from 'react'
import { motion } from 'framer-motion'
import { ArrowLeft, Loader2 } from 'lucide-react'

import type { Preferences } from '../../bindings/github.com/rootkit-lab/xvpn/client'
import { VPNService } from '../../bindings/github.com/rootkit-lab/xvpn/client'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'

interface SettingsPageProps {
  onBack: () => void
}

export function SettingsPage({ onBack }: SettingsPageProps) {
  const [prefs, setPrefs] = useState<Preferences | null>(null)
  const [autostart, setAutostart] = useState<boolean | null>(null)
  const [mtu, setMtu] = useState(0)
  const [mtuDraft, setMtuDraft] = useState('')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [p, a, m] = await Promise.all([
        VPNService.GetPreferences(),
        VPNService.GetAutostart(),
        VPNService.GetMTU(),
      ])
      setPrefs(p)
      setAutostart(a)
      setMtu(m)
      setMtuDraft(m === 0 ? '' : String(m))
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  async function updatePreferences(patch: Partial<Preferences>) {
    if (!prefs) return
    const previous = prefs
    const next = { ...prefs, ...patch }
    setPrefs(next)
    setSaving(true)
    setError(null)
    try {
      const saved = await VPNService.SetPreferences(next)
      setPrefs(saved)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
      setPrefs(previous)
    } finally {
      setSaving(false)
    }
  }

  async function toggleAutostart(enabled: boolean) {
    const previous = autostart
    setAutostart(enabled)
    setError(null)
    try {
      await VPNService.SetAutostart(enabled)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
      setAutostart(previous)
    }
  }

  async function saveMTU() {
    const trimmed = mtuDraft.trim()
    const next = trimmed === '' ? 0 : Number(trimmed)
    if (!Number.isInteger(next) || (next !== 0 && (next < 1280 || next > 1500))) {
      setError('MTU deve ser vazio (automático) ou um inteiro entre 1280 e 1500')
      return
    }
    const previous = mtu
    setMtu(next)
    setSaving(true)
    setError(null)
    try {
      await VPNService.SetMTU(next)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
      setMtu(previous)
      setMtuDraft(previous === 0 ? '' : String(previous))
    } finally {
      setSaving(false)
    }
  }

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: 0.2 }}
      className="flex h-full flex-col gap-4 p-6"
    >
      <header className="flex items-center gap-3">
        <button
          onClick={onBack}
          aria-label="Voltar"
          className="rounded-full p-1.5 text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
        >
          <ArrowLeft className="h-4 w-4" />
        </button>
        <h1 className="text-lg font-semibold">Preferências</h1>
        {saving && <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />}
      </header>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {loading || !prefs ? (
        <p className="text-sm text-muted-foreground">Carregando…</p>
      ) : (
        <div className="flex flex-col gap-3">
          <PreferenceRow
            title="Kill switch"
            description="Bloqueia todo tráfego fora da VPN (fail-closed) se o túnel cair inesperadamente."
            checked={prefs.kill_switch}
            onCheckedChange={(checked) => updatePreferences({ kill_switch: checked })}
          />
          <PreferenceRow
            title="Túnel dividido (split-tunnel)"
            description="Só o tráfego para a rede da VPN (10.66.66.0/24) passa pelo túnel — o resto sai direto pela sua rede local."
            checked={prefs.split_tunnel}
            onCheckedChange={(checked) => updatePreferences({ split_tunnel: checked })}
          />
          <PreferenceRow
            title="Reconexão automática"
            description="Tenta reconectar sozinho (com espera crescente) se o túnel cair sem você ter pedido para desconectar."
            checked={prefs.auto_reconnect}
            onCheckedChange={(checked) => updatePreferences({ auto_reconnect: checked })}
          />
          <PreferenceRow
            title="Iniciar com o sistema"
            description="Abre o XVPN automaticamente (minimizado na bandeja) ao entrar no sistema."
            checked={autostart ?? false}
            onCheckedChange={toggleAutostart}
          />
          <Card className="border-white/5 bg-card/70">
            <CardContent className="flex flex-col gap-3 p-4">
              <div>
                <p className="text-sm font-medium">MTU do túnel</p>
                <p className="text-xs text-muted-foreground">
                  Deixe vazio para o padrão (1420). Use um valor menor (1280–1500) se HTTP/TLS
                  travar atrás de outra VPN ou rede com PMTU reduzido.
                </p>
              </div>
              <div className="flex items-end gap-2">
                <div className="flex flex-1 flex-col gap-1.5">
                  <Label htmlFor="mtu-override" className="hud-label text-muted-foreground/80">
                    MTU
                  </Label>
                  <Input
                    id="mtu-override"
                    type="number"
                    min={1280}
                    max={1500}
                    placeholder="automático"
                    value={mtuDraft}
                    onChange={(e) => setMtuDraft(e.target.value)}
                    className="font-mono"
                  />
                </div>
                <button
                  type="button"
                  onClick={saveMTU}
                  disabled={saving}
                  className="rounded-md border border-border bg-secondary px-3 py-2 font-mono text-sm disabled:opacity-50"
                >
                  Salvar
                </button>
              </div>
            </CardContent>
          </Card>
        </div>
      )}
    </motion.div>
  )
}

function PreferenceRow({
  title,
  description,
  checked,
  onCheckedChange,
}: {
  title: string
  description: string
  checked: boolean
  onCheckedChange: (checked: boolean) => void
}) {
  return (
    <Card className="border-white/5 bg-card/70 transition-shadow hover:shadow-[0_0_24px_-12px_var(--color-glow)]">
      <CardContent className="flex items-start justify-between gap-4 p-4">
        <div>
          <p className="text-sm font-medium">{title}</p>
          <p className="text-xs text-muted-foreground">{description}</p>
        </div>
        <Switch checked={checked} onCheckedChange={onCheckedChange} />
      </CardContent>
    </Card>
  )
}
