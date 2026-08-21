import { useCallback, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { api, ApiError, type DNSRecord, type DNSResponse } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { canWriteAdminProduct, isAdminRole } from '@/lib/roles'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { ProgressBar } from '@/components/ui/progress-bar'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

export function DNSPage() {
  const { user: caller } = useAuth()
  const canEdit = isAdminRole(caller?.role) && canWriteAdminProduct(caller?.role, caller?.products, 'dns')
  const fetchDNS = useCallback(() => api.getDNS(), [])
  const { data, loading, error, reload } = usePollingData(fetchDNS, 20_000)

  return (
    <div className="flex flex-col gap-6">
      {error && <p className="text-sm text-destructive">{error}</p>}

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Resolvedor da intranet</CardTitle>
          <CardDescription>
            dnsmasq só em <code>10.66.66.1:53</code> (<code>wg0</code>). Sem A público para{' '}
            <code>*.corp</code>. O cliente aplica este DNS no túnel; o Chrome com DoH ainda usa o{' '}
            <code>/etc/hosts</code> gravado pelo helper.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {loading || !data ? (
            <Skeleton className="h-28 w-full" />
          ) : (
            <dl className="grid gap-4 sm:grid-cols-2">
              <StatusItem label="Bind" value={data.listen} />
              <StatusItem
                label="Processo"
                value={data.listening ? 'escutando em wg0' : 'não alcançável neste host'}
                ok={data.listening}
              />
              <StatusItem
                label="Consulta corp.ihuull.com"
                value={data.query_ok ? data.query_detail || '10.66.66.1' : data.query_detail || 'falhou'}
                ok={data.query_ok}
              />
              <StatusItem
                label="Último apply"
                value={
                  data.last_applied_at
                    ? new Date(data.last_applied_at).toLocaleString('pt-BR')
                    : 'ainda não aplicado pelo painel'
                }
              />
            </dl>
          )}
          {data?.last_apply_error && (
            <p className="mt-3 text-sm text-destructive">{data.last_apply_error}</p>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Forwarders e zona</CardTitle>
          <CardDescription>
            Consultas fora de <code>corp.ihuull.com</code> vão para estes resolvedores. Catch-all
            responde qualquer <code>*.corp</code> sem registro próprio com <code>10.66.66.1</code>.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {loading || !data ? (
            <Skeleton className="h-24 w-full" />
          ) : canEdit ? (
            <SettingsForm data={data} onSaved={reload} />
          ) : (
            <dl className="grid gap-4 sm:grid-cols-2">
              <StatusItem label="Forwarders" value={data.forwarders.join(', ')} />
              <StatusItem label="Cache" value={String(data.cache_size)} />
              <StatusItem label="Catch-all *.corp" value={data.catch_all ? 'ligado' : 'desligado'} />
            </dl>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Registros A</CardTitle>
          <CardDescription>
            Só IPv4 em <code>10.66.66.0/24</code>. Os oficiais (system) não podem ser apagados.
            Depois de editar, aplique para recarregar o dnsmasq.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          {loading || !data ? (
            <Skeleton className="h-40 w-full" />
          ) : (
            <RecordsTable data={data} canEdit={canEdit} onChanged={reload} />
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function SettingsForm({ data, onSaved }: { data: DNSResponse; onSaved: () => void }) {
  const [forwarders, setForwarders] = useState(data.forwarders.join(', '))
  const [cacheSize, setCacheSize] = useState(String(data.cache_size))
  const [catchAll, setCatchAll] = useState(data.catch_all)
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    const cache = Number(cacheSize)
    if (!Number.isFinite(cache)) {
      toast.error('Cache inválido')
      return
    }
    setSubmitting(true)
    try {
      await api.updateDNS({ forwarders, cache_size: cache, catch_all: catchAll })
      toast.success('Configuração salva — aplique para o dnsmasq recarregar')
      onSaved()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao salvar')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex max-w-xl flex-col gap-4">
      <div className="flex flex-col gap-2">
        <Label htmlFor="dns-fwd">Forwarders (IPv4 públicos)</Label>
        <Input
          id="dns-fwd"
          className="field-glass"
          value={forwarders}
          onChange={(e) => setForwarders(e.target.value)}
          disabled={submitting}
        />
      </div>
      <div className="flex flex-col gap-2">
        <Label htmlFor="dns-cache">Cache</Label>
        <Input
          id="dns-cache"
          type="number"
          min={0}
          max={10000}
          className="field-glass"
          value={cacheSize}
          onChange={(e) => setCacheSize(e.target.value)}
          disabled={submitting}
        />
      </div>
      <label className="flex items-center gap-2 text-sm">
        <input
          type="checkbox"
          checked={catchAll}
          onChange={(e) => setCatchAll(e.target.checked)}
          disabled={submitting}
        />
        Catch-all <code>*.corp.ihuull.com</code> → 10.66.66.1
      </label>
      {submitting && <ProgressBar label="Salvando…" />}
      <Button type="submit" disabled={submitting}>
        {submitting ? 'Salvando…' : 'Salvar zona'}
      </Button>
    </form>
  )
}

function RecordsTable({
  data,
  canEdit,
  onChanged,
}: {
  data: DNSResponse
  canEdit: boolean
  onChanged: () => void
}) {
  const [hostname, setHostname] = useState('')
  const [ipv4, setIPv4] = useState('10.66.66.1')
  const [comment, setComment] = useState('')
  const [busy, setBusy] = useState(false)

  async function addRecord(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    try {
      await api.createDNSRecord({ hostname, ipv4, comment })
      toast.success('Registro criado — aplique para publicar no dnsmasq')
      setHostname('')
      setComment('')
      onChanged()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao criar')
    } finally {
      setBusy(false)
    }
  }

  async function toggle(rec: DNSRecord) {
    setBusy(true)
    try {
      await api.updateDNSRecord(rec.id, {
        hostname: rec.hostname,
        ipv4: rec.ipv4,
        comment: rec.comment,
        enabled: !rec.enabled,
      })
      onChanged()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao atualizar')
    } finally {
      setBusy(false)
    }
  }

  async function remove(rec: DNSRecord) {
    if (!window.confirm(`Apagar ${rec.hostname}?`)) return
    setBusy(true)
    try {
      await api.deleteDNSRecord(rec.id)
      toast.success('Registro removido — aplique para publicar')
      onChanged()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao apagar')
    } finally {
      setBusy(false)
    }
  }

  async function apply() {
    setBusy(true)
    try {
      await api.applyDNS()
      toast.success('dnsmasq recarregado')
      onChanged()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao aplicar')
    } finally {
      setBusy(false)
    }
  }

  return (
    <>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Hostname</TableHead>
            <TableHead>IPv4</TableHead>
            <TableHead />
            <TableHead>Nota</TableHead>
            {canEdit && <TableHead className="text-right">Ações</TableHead>}
          </TableRow>
        </TableHeader>
        <TableBody>
          {data.records.map((r) => (
            <TableRow key={r.id}>
              <TableCell className="font-mono text-sm">{r.hostname}</TableCell>
              <TableCell className="font-mono text-sm">{r.ipv4}</TableCell>
              <TableCell>
                <span className="flex gap-1">
                  {r.system && <Badge variant="secondary">system</Badge>}
                  {!r.enabled && <Badge variant="destructive">off</Badge>}
                </span>
              </TableCell>
              <TableCell className="text-muted-foreground">{r.comment || '—'}</TableCell>
              {canEdit && (
                <TableCell className="text-right">
                  <span className="flex justify-end gap-2">
                    <Button type="button" variant="outline" size="sm" disabled={busy} onClick={() => void toggle(r)}>
                      {r.enabled ? 'Desligar' : 'Ligar'}
                    </Button>
                    {!r.system && (
                      <Button type="button" variant="outline" size="sm" disabled={busy} onClick={() => void remove(r)}>
                        Apagar
                      </Button>
                    )}
                  </span>
                </TableCell>
              )}
            </TableRow>
          ))}
        </TableBody>
      </Table>
      {canEdit && (
        <>
          <form onSubmit={addRecord} className="grid gap-3 sm:grid-cols-4">
            <div className="flex flex-col gap-2 sm:col-span-2">
              <Label htmlFor="dns-host">Novo hostname</Label>
              <Input
                id="dns-host"
                className="field-glass font-mono"
                placeholder="app.corp.ihuull.com"
                value={hostname}
                onChange={(e) => setHostname(e.target.value)}
                disabled={busy}
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="dns-ip">IPv4</Label>
              <Input
                id="dns-ip"
                className="field-glass font-mono"
                value={ipv4}
                onChange={(e) => setIPv4(e.target.value)}
                disabled={busy}
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="dns-note">Nota</Label>
              <Input
                id="dns-note"
                className="field-glass"
                value={comment}
                onChange={(e) => setComment(e.target.value)}
                disabled={busy}
              />
            </div>
            <div className="sm:col-span-4">
              <Button type="submit" variant="outline" disabled={busy}>
                Adicionar registro
              </Button>
            </div>
          </form>
          {busy && <ProgressBar label="Aplicando…" />}
          <Button type="button" disabled={busy} onClick={() => void apply()}>
            Aplicar no dnsmasq
          </Button>
        </>
      )}
    </>
  )
}

function StatusItem({ label, value, ok }: { label: string; value: string; ok?: boolean }) {
  return (
    <div>
      <dt className="text-sm text-muted-foreground">{label}</dt>
      <dd className={ok === false ? 'font-medium text-destructive' : 'font-medium'}>{value}</dd>
    </div>
  )
}
