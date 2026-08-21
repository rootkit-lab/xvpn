import { useCallback, useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Plus, RefreshCw } from 'lucide-react'
import { toast } from 'sonner'
import { api, ApiError, type BitLaunchAccount, type MeshServer, type ServerGroup } from '@/lib/api'
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

export function ServersPage() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const canWrite = isAdminRole(user?.role) && canWriteAdminProduct(user?.role, user?.products, 'compute')
  const fetchServers = useCallback(() => api.listServers(), [])
  const fetchGroups = useCallback(() => api.listServerGroups(), [])
  const { data, loading, reload } = usePollingData(fetchServers, 20_000)
  const { data: groups, reload: reloadGroups } = usePollingData(fetchGroups, 60_000)

  const columns: DataTableColumn<MeshServer>[] = [
    { key: 'name', header: 'Servidor', cell: (s) => <span className="font-medium">{s.name}</span> },
    {
      key: 'hostname',
      header: 'Hostname',
      cell: (s) => (
        <span className="text-muted-foreground">{s.role === 'external' ? s.hostname : `${s.hostname}.corp`}</span>
      ),
    },
    {
      key: 'role',
      header: 'Papel',
      cell: (s) => (
        <Badge variant={s.role === 'control' || s.role === 'external' ? 'secondary' : 'outline'}>
          {s.role === 'external' ? 'externo' : s.role}
        </Badge>
      ),
    },
    {
      key: 'provider',
      header: 'Origem',
      cell: (s) => <span className="text-muted-foreground">{s.provider ?? '—'}</span>,
    },
    { key: 'wg', header: 'wg0', cell: (s) => <span className="text-muted-foreground">{s.wg_ip || '—'}</span> },
    { key: 'status', header: 'Status', cell: (s) => <span className="text-muted-foreground">{s.status}</span> },
    {
      key: 'account',
      header: 'Conta',
      cell: (s) => {
        const acc = (data?.accounts ?? []).find((a) => a.id === s.account_id)
        return <span className="text-muted-foreground">{acc?.email ?? '—'}</span>
      },
    },
  ]

  return (
    <div className="flex flex-col gap-6">
      {canWrite && (
        <div className="flex flex-wrap gap-2">
          <Button
            type="button"
            variant="outline"
            onClick={async () => {
              try {
                await api.importServers()
                toast.success('Inventário atualizado')
                reload()
              } catch (err) {
                toast.error(err instanceof ApiError ? err.message : 'Falha ao importar')
              }
            }}
          >
            <RefreshCw className="size-4" />
            Importar VPS atual
          </Button>
        </div>
      )}

      {canWrite && data && !data.bitlaunch ? (
        <p className="text-sm text-muted-foreground">
          Cadastre uma API em{' '}
          <Link to="/admin/compute/settings" className="underline underline-offset-4">
            Compute → Configurações
          </Link>{' '}
          para criar VPS BitLaunch. VPS já existentes (nó data) usam o formulário manual abaixo — sem token.
        </p>
      ) : null}

      {canWrite ? <RegisterManualForm onCreated={reload} /> : null}
      {canWrite && data?.bitlaunch ? (
        <CreateServerForm accounts={data.accounts ?? []} onCreated={reload} />
      ) : null}
      {canWrite ? <CreateGroupForm onCreated={reloadGroups} /> : null}

      {(groups?.items.length ?? 0) > 0 ? (
        <p className="text-sm text-muted-foreground">
          Grupos: {groups?.items.map((g: ServerGroup) => g.name).join(', ')}
        </p>
      ) : null}

      <DataTable
        columns={columns}
        rows={data?.items ?? []}
        rowKey={(s) => String(s.id)}
        loading={loading || !data}
        emptyTitle="Nenhum servidor ainda. Importe o VPS atual."
        onRowClick={(s) => navigate(`/admin/servers/${s.id}`)}
        page={1}
        perPage={50}
        total={data?.items.length ?? 0}
        onPageChange={() => undefined}
      />
    </div>
  )
}

function RegisterManualForm({ onCreated }: { onCreated: () => void }) {
  const [hostname, setHostname] = useState('data')
  const [name, setName] = useState('data')
  const [ipv4, setIpv4] = useState('66.29.147.100')
  const [notes, setNotes] = useState('')
  const [busy, setBusy] = useState(false)
  const [enrollToken, setEnrollToken] = useState('')
  const [bootstrap, setBootstrap] = useState('')

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setEnrollToken('')
    setBootstrap('')
    try {
      const created = await api.registerServer({
        hostname: hostname.trim().toLowerCase(),
        name: name.trim() || hostname.trim().toLowerCase(),
        ipv4: ipv4.trim(),
        role: 'mesh',
        notes: notes.trim() || undefined,
      })
      setEnrollToken(created.enroll_token ?? '')
      setBootstrap(created.bootstrap ?? '')
      toast.success(`Servidor ${created.hostname} cadastrado — rode o bootstrap no host (SSH do laptop)`)
      onCreated()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao cadastrar servidor')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Cadastrar VPS existente</CardTitle>
        <CardDescription>
          Sem BitLaunch. Entra na malha WireGuard após enroll. A chave SSH privada fica no laptop — o
          control-plane não faz SSH. Use para o nó de dados (<code className="text-xs">66.29.147.100</code>
          ): Mongo, git e containers. Não é inventário do XGIT.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        <form onSubmit={submit} className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          <div className="space-y-1.5">
            <Label htmlFor="man-host">Hostname</Label>
            <Input
              id="man-host"
              value={hostname}
              onChange={(e) => setHostname(e.target.value.toLowerCase())}
              required
              pattern="[a-z0-9][a-z0-9-]{0,18}[a-z0-9]"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="man-name">Nome</Label>
            <Input id="man-name" value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="man-ip">IPv4 público</Label>
            <Input id="man-ip" value={ipv4} onChange={(e) => setIpv4(e.target.value)} required />
          </div>
          <div className="space-y-1.5 sm:col-span-2">
            <Label htmlFor="man-notes">Notas</Label>
            <Input id="man-notes" value={notes} onChange={(e) => setNotes(e.target.value)} />
          </div>
          <div className="flex items-end">
            <Button type="submit" disabled={busy}>
              {busy ? 'Cadastrando…' : 'Cadastrar + enroll'}
            </Button>
          </div>
        </form>
        {enrollToken ? (
          <p className="rounded-md border border-border/60 bg-muted/30 p-3 font-mono text-xs break-all">
            enroll_token (uma vez): {enrollToken}
          </p>
        ) : null}
        {bootstrap ? (
          <pre className="max-h-48 overflow-auto rounded-md border border-border/60 bg-muted/30 p-3 text-xs">
            {bootstrap}
          </pre>
        ) : null}
      </CardContent>
    </Card>
  )
}

function CreateServerForm({
  accounts,
  onCreated,
}: {
  accounts: BitLaunchAccount[]
  onCreated: () => void
}) {
  const [hostname, setHostname] = useState('')
  const [name, setName] = useState('')
  const [hostID, setHostID] = useState('4')
  const [hostImageID, setHostImageID] = useState('')
  const [sizeID, setSizeID] = useState('')
  const [regionID, setRegionID] = useState('')
  const [labels, setLabels] = useState('')
  const [accountID, setAccountID] = useState(accounts[0] ? String(accounts[0].id) : '')
  const [busy, setBusy] = useState(false)
  const [enrollToken, setEnrollToken] = useState('')

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setEnrollToken('')
    try {
      const created = await api.createServer({
        hostname: hostname.trim().toLowerCase(),
        name: name.trim() || hostname.trim().toLowerCase(),
        host_id: Number(hostID),
        host_image_id: hostImageID.trim(),
        size_id: sizeID.trim(),
        region_id: regionID.trim(),
        labels: labels
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean),
        role: 'mesh',
        account_id: accountID ? Number(accountID) : undefined,
      })
      setHostname('')
      setName('')
      setEnrollToken(created.enroll_token ?? '')
      toast.success(`Servidor ${created.hostname} pedido no BitLaunch`)
      onCreated()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao criar servidor')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Novo VPS</CardTitle>
        <CardDescription>
          Cloud-init gera a chave WireGuard no host novo e faz enroll. IDs vêm do BitLaunch. Isso cria um VPS pago.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        <form onSubmit={submit} className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          <div className="space-y-1.5">
            <Label htmlFor="srv-host">Hostname</Label>
            <Input
              id="srv-host"
              value={hostname}
              onChange={(e) => setHostname(e.target.value.toLowerCase())}
              placeholder="edge-ams"
              required
              pattern="[a-z0-9][a-z0-9-]{0,18}[a-z0-9]"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="srv-name">Nome</Label>
            <Input id="srv-name" value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="srv-labels">Labels</Label>
            <Input
              id="srv-labels"
              value={labels}
              onChange={(e) => setLabels(e.target.value)}
              placeholder="edge, runner"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="srv-hid">host_id</Label>
            <Input id="srv-hid" value={hostID} onChange={(e) => setHostID(e.target.value)} required />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="srv-img">host_image_id</Label>
            <Input id="srv-img" value={hostImageID} onChange={(e) => setHostImageID(e.target.value)} required />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="srv-size">size_id</Label>
            <Input id="srv-size" value={sizeID} onChange={(e) => setSizeID(e.target.value)} required />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="srv-region">region_id</Label>
            <Input id="srv-region" value={regionID} onChange={(e) => setRegionID(e.target.value)} required />
          </div>
          {accounts.length > 0 ? (
            <div className="space-y-1.5">
              <Label>Conta BitLaunch</Label>
              <Select value={accountID} onValueChange={setAccountID}>
                <SelectTrigger>
                  <SelectValue placeholder="Conta" />
                </SelectTrigger>
                <SelectContent>
                  {accounts.map((a) => (
                    <SelectItem key={a.id} value={String(a.id)}>
                      {a.name} ({a.email})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          ) : null}
          <div className="sm:col-span-2 lg:col-span-3">
            <Button type="submit" disabled={busy || hostname.trim().length < 2}>
              <Plus className="size-4" />
              {busy ? 'Criando…' : 'Criar no BitLaunch'}
            </Button>
          </div>
        </form>
        {enrollToken ? (
          <p className="text-sm">
            Token de enroll (uma vez): <code className="break-all">{enrollToken}</code>
          </p>
        ) : null}
      </CardContent>
    </Card>
  )
}

function CreateGroupForm({ onCreated }: { onCreated: () => void }) {
  const [name, setName] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      await api.createServerGroup(name.trim())
      setName('')
      toast.success('Grupo criado')
      onCreated()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao criar grupo')
    } finally {
      setBusy(false)
    }
  }

  return (
    <form onSubmit={submit} className="flex flex-wrap items-end gap-2">
      <div className="space-y-1.5">
        <Label htmlFor="grp-name">Grupo</Label>
        <Input id="grp-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="edge" required />
      </div>
      <Button type="submit" variant="outline" disabled={busy || !name.trim()}>
        Criar grupo
      </Button>
    </form>
  )
}
