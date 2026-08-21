import { useCallback, useMemo, useState, type FormEvent } from 'react'
import { Network } from 'lucide-react'
import { toast } from 'sonner'
import {
  api,
  ApiError,
  type NetworkMember,
  type NetworkRule,
  type OverlayNetwork,
} from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { canWriteAdminProduct, isAdminRole } from '@/lib/roles'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { DataTable, type DataTableColumn } from '@/components/data-table'

export function NetworksPage() {
  const { user } = useAuth()
  const canWrite = isAdminRole(user?.role) && canWriteAdminProduct(user?.role, user?.products, 'core')
  const fetchNets = useCallback(() => api.listNetworks(), [])
  const { data, loading, reload } = usePollingData(fetchNets, 30_000)
  const nets = data?.items ?? []
  const byID = useMemo(() => Object.fromEntries(nets.map((n) => [n.id, n])), [nets])

  const columns: DataTableColumn<OverlayNetwork>[] = [
    { key: 'name', header: 'Rede', cell: (n) => <span className="font-medium">{n.name}</span> },
    { key: 'slug', header: 'Slug', cell: (n) => <span className="text-muted-foreground">{n.slug}</span> },
    {
      key: 'kind',
      header: 'Kind',
      cell: (n) => <Badge variant={n.system ? 'secondary' : 'outline'}>{n.kind}</Badge>,
    },
    { key: 'cidr', header: 'CIDR', cell: (n) => <span className="font-mono text-sm">{n.cidr}</span> },
    { key: 'exit', header: 'Exit', cell: (n) => (n.exit ? 'sim' : 'não') },
  ]

  return (
    <div className="flex flex-col gap-6">
      <p className="text-sm text-muted-foreground">
        Um <code>wg0</code>, várias faixas. Pool de custom: {data?.pool ?? '10.66.80.0/20'}. Device
        enroll → <strong>users</strong>. Malha → <strong>infra</strong>. Sem peer de usuário em{' '}
        <code>10.66.66.0/24</code>.
      </p>

      {canWrite ? <CreateNetworkForm onCreated={reload} /> : null}

      <DataTable
        columns={columns}
        rows={nets}
        rowKey={(n) => String(n.id)}
        loading={loading || !data}
        emptyTitle="Redes ainda não semeadas."
        page={1}
        perPage={50}
        total={nets.length}
        onPageChange={() => undefined}
      />

      {canWrite && nets.length > 0 ? (
        <>
          <MembersCard
            nets={nets}
            members={data?.members ?? []}
            onChanged={reload}
          />
          <RulesCard nets={nets} rules={data?.rules ?? []} byID={byID} onChanged={reload} />
        </>
      ) : (
        <RulesRead rules={data?.rules ?? []} byID={byID} />
      )}
    </div>
  )
}

function CreateNetworkForm({ onCreated }: { onCreated: () => void }) {
  const [slug, setSlug] = useState('')
  const [name, setName] = useState('')
  const [cidr, setCidr] = useState('')
  const [corp, setCorp] = useState(true)
  const [busy, setBusy] = useState(false)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      await api.createNetwork({
        slug,
        name: name || undefined,
        cidr: cidr || undefined,
        corp_access: corp,
      })
      toast.success('Rede criada')
      setSlug('')
      setName('')
      setCidr('')
      onCreated()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao criar rede')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Network className="size-4" />
          Nova rede custom
        </CardTitle>
        <CardDescription>CIDR vazio pega o próximo /24 livre no pool.</CardDescription>
      </CardHeader>
      <CardContent>
        <form className="grid gap-3 sm:grid-cols-2" onSubmit={onSubmit}>
          <div className="space-y-1.5">
            <Label htmlFor="net-slug">Slug</Label>
            <Input id="net-slug" value={slug} onChange={(e) => setSlug(e.target.value)} required />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="net-name">Nome</Label>
            <Input id="net-name" value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="space-y-1.5 sm:col-span-2">
            <Label htmlFor="net-cidr">CIDR (opcional)</Label>
            <Input
              id="net-cidr"
              placeholder="10.66.81.0/24"
              value={cidr}
              onChange={(e) => setCidr(e.target.value)}
            />
          </div>
          <label className="flex items-center gap-2 text-sm sm:col-span-2">
            <input type="checkbox" checked={corp} onChange={(e) => setCorp(e.target.checked)} />
            Acesso corp (443/53 para infra)
          </label>
          <Button type="submit" disabled={busy || !slug}>
            Criar
          </Button>
        </form>
      </CardContent>
    </Card>
  )
}

function MembersCard({
  nets,
  members,
  onChanged,
}: {
  nets: OverlayNetwork[]
  members: NetworkMember[]
  onChanged: () => void
}) {
  const [networkID, setNetworkID] = useState(String(nets[0]?.id ?? ''))
  const [kind, setKind] = useState('user')
  const [subjectID, setSubjectID] = useState('')
  const [busy, setBusy] = useState(false)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      await api.addNetworkMember(Number(networkID), {
        subject_kind: kind,
        subject_id: Number(subjectID),
      })
      toast.success('Membro adicionado')
      setSubjectID('')
      onChanged()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao adicionar')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Membros</CardTitle>
        <CardDescription>Membership dá rota à CIDR e FORWARD implícito home↔rede.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <form className="flex flex-wrap items-end gap-2" onSubmit={onSubmit}>
          <select
            className="field-glass h-9 rounded-md px-2 text-sm"
            value={networkID}
            onChange={(e) => setNetworkID(e.target.value)}
          >
            {nets.map((n) => (
              <option key={n.id} value={n.id}>
                {n.slug}
              </option>
            ))}
          </select>
          <select className="field-glass h-9 rounded-md px-2 text-sm" value={kind} onChange={(e) => setKind(e.target.value)}>
            <option value="user">user</option>
            <option value="device">device</option>
            <option value="mesh_server">mesh_server</option>
          </select>
          <Input
            className="w-28"
            placeholder="id"
            value={subjectID}
            onChange={(e) => setSubjectID(e.target.value)}
            required
          />
          <Button type="submit" disabled={busy}>
            Adicionar
          </Button>
        </form>
        <ul className="space-y-1 text-sm text-muted-foreground">
          {members.map((m) => (
            <li key={m.id} className="flex items-center justify-between gap-2">
              <span>
                {m.subject_kind} #{m.subject_id} → {nets.find((n) => n.id === m.network_id)?.slug}
              </span>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={async () => {
                  if (!confirm('Remover membro?')) return
                  try {
                    await api.deleteNetworkMember(m.network_id, m.id)
                    onChanged()
                  } catch (err) {
                    toast.error(err instanceof ApiError ? err.message : 'Falha')
                  }
                }}
              >
                Remover
              </Button>
            </li>
          ))}
        </ul>
      </CardContent>
    </Card>
  )
}

function RulesCard({
  nets,
  rules,
  byID,
  onChanged,
}: {
  nets: OverlayNetwork[]
  rules: NetworkRule[]
  byID: Record<number, OverlayNetwork>
  onChanged: () => void
}) {
  const [slug, setSlug] = useState('')
  const [src, setSrc] = useState(String(nets.find((n) => n.kind === 'users')?.id ?? ''))
  const [dst, setDst] = useState(String(nets.find((n) => n.kind === 'infra')?.id ?? ''))
  const [ports, setPorts] = useState('443')
  const [proto, setProto] = useState('tcp')
  const [busy, setBusy] = useState(false)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      await api.createNetworkRule({
        slug,
        src_network_id: Number(src),
        dst_network_id: Number(dst),
        proto,
        ports,
      })
      toast.success('Regra criada')
      setSlug('')
      onChanged()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao criar regra')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Regras</CardTitle>
        <CardDescription>Default deny entre CIDRs. 27017 não entra no seed.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <form className="grid gap-2 sm:grid-cols-5" onSubmit={onSubmit}>
          <Input placeholder="slug" value={slug} onChange={(e) => setSlug(e.target.value)} required />
          <select className="field-glass h-9 rounded-md px-2 text-sm" value={src} onChange={(e) => setSrc(e.target.value)}>
            {nets.map((n) => (
              <option key={n.id} value={n.id}>
                de {n.slug}
              </option>
            ))}
          </select>
          <select className="field-glass h-9 rounded-md px-2 text-sm" value={dst} onChange={(e) => setDst(e.target.value)}>
            {nets.map((n) => (
              <option key={n.id} value={n.id}>
                para {n.slug}
              </option>
            ))}
          </select>
          <select className="field-glass h-9 rounded-md px-2 text-sm" value={proto} onChange={(e) => setProto(e.target.value)}>
            <option value="tcp">tcp</option>
            <option value="udp">udp</option>
            <option value="any">any</option>
          </select>
          <Input placeholder="portas" value={ports} onChange={(e) => setPorts(e.target.value)} />
          <Button type="submit" className="sm:col-span-5" disabled={busy || !slug}>
            Criar regra
          </Button>
        </form>
        <RulesRead rules={rules} byID={byID} onDelete={onChanged} />
      </CardContent>
    </Card>
  )
}

function RulesRead({
  rules,
  byID,
  onDelete,
}: {
  rules: NetworkRule[]
  byID: Record<number, OverlayNetwork>
  onDelete?: () => void
}) {
  if (rules.length === 0) return null
  return (
    <ul className="space-y-1 text-sm text-muted-foreground">
      {rules.map((r) => (
        <li key={r.id} className="flex items-center justify-between gap-2">
          <span>
            {r.slug}: {byID[r.src_network_id]?.slug} → {byID[r.dst_network_id]?.slug} {r.proto}{' '}
            {r.ports || '*'} {r.system ? '(sistema)' : ''}
          </span>
          {onDelete && !r.system ? (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={async () => {
                if (!confirm('Apagar regra?')) return
                try {
                  await api.deleteNetworkRule(r.id)
                  onDelete()
                } catch (err) {
                  toast.error(err instanceof ApiError ? err.message : 'Falha')
                }
              }}
            >
              Apagar
            </Button>
          ) : null}
        </li>
      ))}
    </ul>
  )
}
