import { useCallback, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import {
  api,
  ApiError,
  type BackupDestination,
  type BackupJob,
  type BackupKind,
  type BackupSecret,
  type BackupSettings,
} from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { canWriteAdminProduct, isAdminRole } from '@/lib/roles'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { DataTable, type DataTableColumn } from '@/components/data-table'

const KINDS: { id: BackupKind; label: string }[] = [
  { id: 'sftp', label: 'SFTP' },
  { id: 'b2', label: 'Backblaze B2' },
  { id: 's3', label: 'S3 / MinIO' },
  { id: 'webdav', label: 'WebDAV' },
  { id: 'drive', label: 'Google Drive (rclone)' },
  { id: 'xdriver', label: 'XDRIVER (extra, sem Mongo)' },
]

export function BackupsPage() {
  const { user } = useAuth()
  const canWrite = isAdminRole(user?.role) && canWriteAdminProduct(user?.role, user?.products, 'core')
  const fetchSettings = useCallback(() => api.getBackupSettings(), [])
  const fetchDests = useCallback(() => api.listBackupDestinations(), [])
  const fetchJobs = useCallback(() => api.listBackupJobs(), [])
  const settings = usePollingData(fetchSettings, 60_000)
  const dests = usePollingData(fetchDests, 15_000)
  const jobs = usePollingData(fetchJobs, 10_000)

  const destColumns: DataTableColumn<BackupDestination>[] = [
    { key: 'name', header: 'Nome', cell: (d) => <span className="font-medium">{d.name}</span> },
    { key: 'kind', header: 'Tipo', cell: (d) => <Badge variant="outline">{d.kind}</Badge> },
    {
      key: 'target',
      header: 'Alvo',
      cell: (d) => <span className="font-mono text-xs">{[d.endpoint, d.path].filter(Boolean).join(' ')}</span>,
    },
    {
      key: 'offsite',
      header: 'Off-site',
      cell: (d) => (d.offsite ? 'sim' : 'não'),
    },
    {
      key: 'enabled',
      header: 'Estado',
      cell: (d) => <Badge variant={d.enabled ? 'secondary' : 'outline'}>{d.enabled ? 'ativo' : 'off'}</Badge>,
    },
  ]

  const jobColumns: DataTableColumn<BackupJob>[] = [
    { key: 'created', header: 'Quando', cell: (j) => <span className="font-mono text-xs">{j.created_at}</span> },
    { key: 'dest', header: 'Destino', cell: (j) => j.destination || String(j.destination_id) },
    { key: 'mode', header: 'Modo', cell: (j) => (j.dry_run ? 'dry-run' : 'snapshot') },
    {
      key: 'status',
      header: 'Status',
      cell: (j) => <Badge variant={j.status === 'ok' ? 'secondary' : j.status === 'error' ? 'destructive' : 'outline'}>{j.status}</Badge>,
    },
    { key: 'snap', header: 'Snapshot', cell: (j) => <span className="font-mono text-xs">{j.snapshot_id || '—'}</span> },
    { key: 'err', header: 'Erro', cell: (j) => <span className="text-destructive">{j.error || ''}</span> },
  ]

  return (
    <div className="flex flex-col gap-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-base">O que entra</CardTitle>
          <CardDescription>
            Off-site via restic + rclone. O <code>backup.sh</code> local no mesmo disco continua separado. Tokens
            ficam só no VPS.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {settings.data ? (
            <IncludeForm settings={settings.data} canWrite={canWrite} onSaved={settings.reload} />
          ) : (
            <p className="text-sm text-muted-foreground">Carregando…</p>
          )}
        </CardContent>
      </Card>

      {canWrite ? <CreateDestForm onCreated={dests.reload} /> : null}

      <DataTable
        columns={destColumns}
        rows={dests.data?.items ?? []}
        rowKey={(d) => String(d.id)}
        loading={dests.loading || !dests.data}
        emptyTitle="Nenhum destino ainda."
        page={1}
        perPage={20}
        total={dests.data?.items.length ?? 0}
        onPageChange={() => undefined}
      />

      {canWrite && (dests.data?.items.length ?? 0) > 0 ? (
        <RunBar destinations={dests.data?.items ?? []} onRan={() => void jobs.reload()} />
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Últimos jobs</CardTitle>
        </CardHeader>
        <CardContent>
          <DataTable
            columns={jobColumns}
            rows={jobs.data?.items ?? []}
            rowKey={(j) => String(j.id)}
            loading={jobs.loading || !jobs.data}
            emptyTitle="Nenhum job ainda."
            page={1}
            perPage={20}
            total={jobs.data?.items.length ?? 0}
            onPageChange={() => undefined}
          />
        </CardContent>
      </Card>
    </div>
  )
}

function IncludeForm({
  settings,
  canWrite,
  onSaved,
}: {
  settings: BackupSettings
  canWrite: boolean
  onSaved: () => void
}) {
  const [days, setDays] = useState(String(settings.retention_days))
  const [mongo, setMongo] = useState(settings.include_mongo)
  const [market, setMarket] = useState(settings.include_marketplace)
  const [git, setGit] = useState(settings.include_git)
  const [social, setSocial] = useState(settings.include_social)
  const [busy, setBusy] = useState(false)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      await api.updateBackupSettings({
        retention_days: Number(days),
        include_mongo: mongo,
        include_marketplace: market,
        include_git: git,
        include_social: social,
      })
      toast.success('Preferências salvas')
      onSaved()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao salvar')
    } finally {
      setBusy(false)
    }
  }

  return (
    <form className="grid gap-4 sm:grid-cols-2" onSubmit={onSubmit}>
      <div className="space-y-2">
        <Label htmlFor="retention">Retenção (dias)</Label>
        <Input id="retention" type="number" min={1} max={3650} value={days} disabled={!canWrite} onChange={(e) => setDays(e.target.value)} />
      </div>
      <fieldset className="space-y-2 text-sm">
        <legend className="text-muted-foreground">Incluir</legend>
        <label className="flex items-center gap-2">
          <input type="checkbox" checked={mongo} disabled={!canWrite} onChange={(e) => setMongo(e.target.checked)} />
          Mongo control-plane
        </label>
        <label className="flex items-center gap-2">
          <input type="checkbox" checked={market} disabled={!canWrite} onChange={(e) => setMarket(e.target.checked)} />
          Marketplace
        </label>
        <label className="flex items-center gap-2">
          <input type="checkbox" checked={git} disabled={!canWrite} onChange={(e) => setGit(e.target.checked)} />
          Git (forge)
        </label>
        <label className="flex items-center gap-2">
          <input type="checkbox" checked={social} disabled={!canWrite} onChange={(e) => setSocial(e.target.checked)} />
          Mídia social
        </label>
      </fieldset>
      {canWrite ? (
        <div>
          <Button type="submit" disabled={busy}>
            Salvar
          </Button>
        </div>
      ) : null}
    </form>
  )
}

function CreateDestForm({ onCreated }: { onCreated: () => void }) {
  const [name, setName] = useState('')
  const [kind, setKind] = useState<BackupKind>('sftp')
  const [endpoint, setEndpoint] = useState('')
  const [path, setPath] = useState('')
  const [password, setPassword] = useState('')
  const [extra, setExtra] = useState('')
  const [busy, setBusy] = useState(false)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      const secret: BackupSecret = { password }
      if (kind === 'sftp') {
        secret.sftp_host = endpoint
        secret.sftp_path = path
        secret.sftp_user = extra
      }
      if (kind === 'b2') {
        secret.b2_account_id = extra
        secret.b2_key = password
      }
      if (kind === 's3') {
        secret.s3_endpoint = endpoint
        secret.s3_bucket = path
        secret.s3_access = extra
        secret.s3_secret = password
      }
      if (kind === 'webdav') {
        secret.webdav_url = endpoint
        secret.webdav_user = extra
        secret.webdav_pass = password
      }
      if (kind === 'drive') {
        secret.rclone_conf = extra
      }
      await api.createBackupDestination({
        name,
        kind,
        endpoint,
        path,
        secret: kind === 'xdriver' ? undefined : secret,
      })
      toast.success('Destino criado')
      setName('')
      setPassword('')
      setExtra('')
      onCreated()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao criar')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Novo destino</CardTitle>
        <CardDescription>A senha do repositório restic e as chaves não voltam no GET.</CardDescription>
      </CardHeader>
      <CardContent>
        <form className="grid gap-4 sm:grid-cols-2" onSubmit={onSubmit}>
          <div className="space-y-2">
            <Label htmlFor="bk-name">Nome</Label>
            <Input id="bk-name" value={name} onChange={(e) => setName(e.target.value)} required />
          </div>
          <div className="space-y-2">
            <Label>Tipo</Label>
            <Select value={kind} onValueChange={(v) => setKind(v as BackupKind)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {KINDS.map((k) => (
                  <SelectItem key={k.id} value={k.id}>
                    {k.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label htmlFor="bk-end">Host / endpoint</Label>
            <Input id="bk-end" value={endpoint} onChange={(e) => setEndpoint(e.target.value)} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="bk-path">Path / bucket</Label>
            <Input id="bk-path" value={path} onChange={(e) => setPath(e.target.value)} />
          </div>
          {kind !== 'xdriver' ? (
            <>
              <div className="space-y-2">
                <Label htmlFor="bk-pass">Senha restic / chave</Label>
                <Input id="bk-pass" type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label htmlFor="bk-extra">Usuário / account / rclone.conf</Label>
                <Input id="bk-extra" value={extra} onChange={(e) => setExtra(e.target.value)} />
              </div>
            </>
          ) : null}
          <div>
            <Button type="submit" disabled={busy || !name}>
              Criar
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  )
}

function RunBar({ destinations, onRan }: { destinations: BackupDestination[]; onRan: () => void }) {
  const [id, setId] = useState(String(destinations[0]?.id ?? ''))
  const [busy, setBusy] = useState(false)

  async function run(dry: boolean) {
    const n = Number(id)
    if (!n) return
    setBusy(true)
    try {
      const job = await api.runBackup(n, dry)
      if (job.status === 'ok') toast.success(dry ? 'Dry-run ok' : `Snapshot ${job.snapshot_id}`)
      else toast.error(job.error || 'Job falhou')
      onRan()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha no job')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-wrap items-end gap-3">
      <div className="space-y-2">
        <Label>Rodar em</Label>
        <Select value={id} onValueChange={setId}>
          <SelectTrigger className="w-56">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {destinations
              .filter((d) => d.enabled)
              .map((d) => (
                <SelectItem key={d.id} value={String(d.id)}>
                  {d.name}
                </SelectItem>
              ))}
          </SelectContent>
        </Select>
      </div>
      <Button type="button" variant="outline" disabled={busy} onClick={() => void run(true)}>
        Dry-run
      </Button>
      <Button type="button" disabled={busy} onClick={() => void run(false)}>
        Snapshot
      </Button>
    </div>
  )
}
