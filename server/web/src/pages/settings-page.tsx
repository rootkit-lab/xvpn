import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { toast } from 'sonner'
import { api, ApiError, type CodespaceLLMProvider, type ConfigResponse } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { canWriteAdminProduct, isAdminRole } from '@/lib/roles'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { ProgressBar } from '@/components/ui/progress-bar'

export function SettingsPage() {
  const { user: caller } = useAuth()
  const canEdit = isAdminRole(caller?.role) && canWriteAdminProduct(caller?.role, caller?.products, 'core')
  const fetchConfig = useCallback(() => api.getConfig(), [])
  const { data: config, loading, error, reload } = usePollingData(fetchConfig, 60_000)

  return (
    <div className="flex flex-col gap-6">
      {error && <p className="text-sm text-destructive">{error}</p>}

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Rede WireGuard</CardTitle>
          <CardDescription>
            Alterar sub-rede, porta ou endpoint em runtime quebraria peers e firewall — continue via
            variáveis de ambiente do servidor (ver <code>server/README.md</code>) e reinicie o serviço.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {loading || !config ? (
            <Skeleton className="h-40 w-full" />
          ) : (
            <dl className="grid gap-4 sm:grid-cols-2">
              <SettingItem label="Interface" value={config.wireguard_interface} />
              <SettingItem label="Endereço da interface" value={config.wireguard_address} />
              <SettingItem label="Sub-rede de peers" value={config.wireguard_allowed_subnet} />
              <SettingItem label="Porta de escuta (UDP)" value={String(config.wireguard_listen_port)} />
              <SettingItem label="Endpoint público" value={config.wireguard_endpoint} />
              <SettingItem label="Chave pública do servidor" value={config.server_public_key} mono />
            </dl>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">DNS da intranet</CardTitle>
          <CardDescription>
            A zona <code>*.corp.ihuull.com</code> não se edita aqui. Use{' '}
            <Link to="/admin/dns" className="underline underline-offset-4">
              /admin/dns
            </Link>{' '}
            (forwarders, registros A, apply no dnsmasq). Bind fixo em{' '}
            <code>10.66.66.1:53</code>.
          </CardDescription>
        </CardHeader>
      </Card>

      <CodespaceLLMCard canEdit={canEdit} />

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Validades</CardTitle>
          <CardDescription>
            Persistidas no banco (sobrevivem a restart). Sessões JWT já emitidas mantêm a expiração
            original até renovar o login.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {loading || !config ? (
            <Skeleton className="h-28 w-full" />
          ) : canEdit ? (
            <TTLEditForm config={config} onSaved={reload} />
          ) : (
            <dl className="grid gap-4 sm:grid-cols-2">
              <SettingItem label="Validade do convite" value={`${config.invite_token_ttl_minutes} min`} />
              <SettingItem label="Validade da sessão do painel" value={`${config.jwt_token_ttl_minutes} min`} />
            </dl>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function TTLEditForm({ config, onSaved }: { config: ConfigResponse; onSaved: () => void }) {
  const [inviteTTL, setInviteTTL] = useState(String(config.invite_token_ttl_minutes))
  const [jwtTTL, setJwtTTL] = useState(String(config.jwt_token_ttl_minutes))
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    setError(null)
    const invite = Number(inviteTTL)
    const jwt = Number(jwtTTL)
    if (!Number.isFinite(invite) || !Number.isFinite(jwt)) {
      setError('Informe números válidos')
      return
    }
    setSubmitting(true)
    try {
      await api.updateConfig({
        invite_token_ttl_minutes: invite,
        jwt_token_ttl_minutes: jwt,
      })
      toast.success('Validades atualizadas')
      onSaved()
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : 'Falha ao salvar'
      setError(msg)
      toast.error(msg)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex max-w-md flex-col gap-4">
      <div className="flex flex-col gap-2">
        <Label htmlFor="invite-ttl">Validade do convite (minutos)</Label>
        <Input
          id="invite-ttl"
          type="number"
          min={1}
          max={10080}
          value={inviteTTL}
          onChange={(e) => setInviteTTL(e.target.value)}
          disabled={submitting}
        />
      </div>
      <div className="flex flex-col gap-2">
        <Label htmlFor="jwt-ttl">Validade da sessão do painel (minutos)</Label>
        <Input
          id="jwt-ttl"
          type="number"
          min={5}
          max={10080}
          value={jwtTTL}
          onChange={(e) => setJwtTTL(e.target.value)}
          disabled={submitting}
        />
      </div>
      {submitting && <ProgressBar label="Salvando…" />}
      {error && <p className="text-sm text-destructive">{error}</p>}
      <Button type="submit" disabled={submitting}>
        {submitting ? 'Salvando…' : 'Salvar'}
      </Button>
    </form>
  )
}

function CodespaceLLMCard({ canEdit }: { canEdit: boolean }) {
  const fetchLLM = useCallback(() => api.getCodespaceLLMSettings(), [])
  const { data, loading, reload } = usePollingData(fetchLLM, 60_000)
  const [provider, setProvider] = useState<CodespaceLLMProvider>('glm')
  const [baseURL, setBaseURL] = useState('')
  const [model, setModel] = useState('')
  const [apiKey, setApiKey] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!data) return
    setProvider(data.provider)
    setBaseURL(data.base_url)
    setModel(data.model)
  }, [data])

  async function save(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      await api.updateCodespaceLLMSettings({
        provider,
        base_url: baseURL,
        model,
        api_key: apiKey,
      })
      setApiKey('')
      toast.success('Assistente XCODESPACES salvo')
      await reload()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao salvar o assistente')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Assistente XCODESPACES</CardTitle>
        <CardDescription>
          GLM / OpenAI-compatível / Anthropic. A key fica só no VPS — o GET nunca devolve plaintext.
          Chat e generate commit no codespace passam pelo proxy. Sem Copilot, sem Continue.
        </CardDescription>
      </CardHeader>
      <CardContent>
        {loading || !data ? (
          <Skeleton className="h-40 w-full" />
        ) : canEdit ? (
          <form onSubmit={save} className="flex max-w-lg flex-col gap-4">
            <div className="flex flex-col gap-2">
              <Label htmlFor="llm-provider">Provedor</Label>
              <select
                id="llm-provider"
                className="border-input bg-background h-9 rounded-md border px-3 text-sm"
                value={provider}
                onChange={(e) => setProvider(e.target.value as CodespaceLLMProvider)}
                disabled={busy}
              >
                <option value="glm">GLM (Zhipu)</option>
                <option value="openai">OpenAI</option>
                <option value="anthropic">Anthropic</option>
                <option value="compatible">OpenAI-compatível</option>
              </select>
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="llm-base">Base URL</Label>
              <Input
                id="llm-base"
                value={baseURL}
                onChange={(e) => setBaseURL(e.target.value)}
                placeholder="https://open.bigmodel.cn/api/paas/v4"
                disabled={busy}
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="llm-model">Modelo</Label>
              <Input id="llm-model" value={model} onChange={(e) => setModel(e.target.value)} disabled={busy} />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="llm-key">API key {data.has_key ? '(•••• gravada — vazio mantém)' : ''}</Label>
              <Input
                id="llm-key"
                type="password"
                autoComplete="off"
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                placeholder={data.has_key ? '••••' : ''}
                disabled={busy}
              />
            </div>
            {busy && <ProgressBar label="Salvando…" />}
            <Button type="submit" disabled={busy}>
              {busy ? 'Salvando…' : 'Salvar assistente'}
            </Button>
          </form>
        ) : (
          <dl className="grid gap-4 sm:grid-cols-2">
            <SettingItem label="Provedor" value={data.provider} />
            <SettingItem label="Modelo" value={data.model} />
            <SettingItem label="Base URL" value={data.base_url} mono />
            <SettingItem label="Key" value={data.has_key ? '••••' : 'não configurada'} />
          </dl>
        )}
      </CardContent>
    </Card>
  )
}

function SettingItem({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div>
      <dt className="text-sm text-muted-foreground">{label}</dt>
      <dd className={mono ? 'break-all font-mono text-sm' : 'font-medium'}>{value}</dd>
    </div>
  )
}
