import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import { api, ApiError, type MeshServer, type MeshServerRole, type ServerGroup, type User } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { canWriteAdminProduct, isAdminRole } from '@/lib/roles'
import { UserPicker } from '@/components/user-picker'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'

export function ServerDetailPage() {
  const { id = '' } = useParams()
  const serverID = Number(id)
  const { user } = useAuth()
  const canWrite = isAdminRole(user?.role) && canWriteAdminProduct(user?.role, user?.products, 'compute')
  const fetchServer = useCallback(() => api.getServer(serverID), [serverID])
  const { data, loading, error, reload } = usePollingData(fetchServer, 20_000)

  if (loading || !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-48 w-full" />
  }

  return (
    <div className="flex flex-col gap-6">
      <p className="text-sm text-muted-foreground">
        <Link to="/admin/servers" className="hover:underline">
          Servidores
        </Link>
        <span className="px-1.5">/</span>
        <span className="text-foreground">{data.hostname}</span>
      </p>

      {canWrite ? <ServerForm server={data} onSaved={reload} /> : <ServerRead server={data} />}
      {canWrite ? <AccessForm server={data} onSaved={reload} /> : null}
      {canWrite && data.role !== 'control' ? <DangerZone server={data} /> : null}
    </div>
  )
}

function ServerRead({ server }: { server: MeshServer }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{server.name}</CardTitle>
        <CardDescription>
          {server.hostname}.corp.ihuull.com · {server.wg_ip || 'sem peer ainda'}
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-wrap gap-2">
        <Badge variant="outline">{server.role}</Badge>
        <Badge variant="outline">{server.status}</Badge>
        {server.labels.map((l) => (
          <Badge key={l} variant="secondary">
            {l}
          </Badge>
        ))}
      </CardContent>
    </Card>
  )
}

function ServerForm({ server, onSaved }: { server: MeshServer; onSaved: () => void }) {
  const [name, setName] = useState(server.name)
  const [labels, setLabels] = useState(server.labels.join(', '))
  const [role, setRole] = useState<MeshServerRole>(server.role === 'runner' ? 'runner' : 'mesh')
  const [groupID, setGroupID] = useState(server.group_id ? String(server.group_id) : '0')
  const [busy, setBusy] = useState(false)
  const fetchGroups = useCallback(() => api.listServerGroups(), [])
  const { data: groups } = usePollingData(fetchGroups, 60_000)

  useEffect(() => {
    setName(server.name)
    setLabels(server.labels.join(', '))
    setRole(server.role === 'runner' ? 'runner' : 'mesh')
    setGroupID(server.group_id ? String(server.group_id) : '0')
  }, [server])

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      await api.updateServer(server.id, {
        name: name.trim(),
        labels: labels
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean),
        role: server.role === 'control' ? undefined : role,
        group_id: Number(groupID),
      })
      toast.success('Servidor atualizado')
      onSaved()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao salvar')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{server.hostname}.corp.ihuull.com</CardTitle>
        <CardDescription>
          IPv4 {server.ipv4 || '—'} · wg0 {server.wg_ip || 'aguardando enroll'} · {server.status}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={submit} className="grid gap-3 sm:grid-cols-2">
          <div className="space-y-1.5">
            <Label htmlFor="srv-name">Nome</Label>
            <Input id="srv-name" value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="srv-labels">Labels</Label>
            <Input id="srv-labels" value={labels} onChange={(e) => setLabels(e.target.value)} />
          </div>
          {server.role !== 'control' ? (
            <div className="space-y-1.5">
              <Label>Papel</Label>
              <Select value={role} onValueChange={(v) => setRole(v as MeshServerRole)}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="mesh">mesh</SelectItem>
                  <SelectItem value="runner">runner</SelectItem>
                </SelectContent>
              </Select>
            </div>
          ) : null}
          <div className="space-y-1.5">
            <Label>Grupo</Label>
            <Select value={groupID} onValueChange={setGroupID}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="0">nenhum</SelectItem>
                {(groups?.items ?? []).map((g: ServerGroup) => (
                  <SelectItem key={g.id} value={String(g.id)}>
                    {g.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="sm:col-span-2">
            <Button type="submit" disabled={busy}>
              {busy ? 'Salvando…' : 'Salvar'}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  )
}

function AccessForm({ server, onSaved }: { server: MeshServer; onSaved: () => void }) {
  const fetchUsers = useCallback(() => api.listUsers({ per_page: 200 }), [])
  const { data } = usePollingData(fetchUsers, 60_000)
  const [selected, setSelected] = useState<Set<number>>(new Set(server.access_user_ids ?? []))
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    setSelected(new Set(server.access_user_ids ?? []))
  }, [server])

  async function save() {
    setBusy(true)
    try {
      await api.setServerAccess(server.id, [...selected])
      toast.success('Acesso atualizado')
      onSaved()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao gravar acesso')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">ServerAccess</CardTitle>
        <CardDescription>Quem opera este host. Resolver o nome *.corp não basta.</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        <UserPicker
          users={(data?.items ?? []) as User[]}
          selected={selected}
          onToggle={(id) => {
            setSelected((prev) => {
              const next = new Set(prev)
              if (next.has(id)) next.delete(id)
              else next.add(id)
              return next
            })
          }}
        />
        <Button type="button" onClick={save} disabled={busy}>
          {busy ? 'Salvando…' : 'Salvar acesso'}
        </Button>
      </CardContent>
    </Card>
  )
}

function DangerZone({ server }: { server: MeshServer }) {
  const navigate = useNavigate()
  const [busy, setBusy] = useState(false)

  async function destroy() {
    setBusy(true)
    try {
      await api.destroyServer(server.id)
      toast.success('Servidor destruído')
      navigate('/admin/servers')
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao destruir')
      setBusy(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Destruir</CardTitle>
        <CardDescription>Remove o VPS no BitLaunch, o peer wg0 e o A corp. O node de controle não entra aqui.</CardDescription>
      </CardHeader>
      <CardContent>
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button variant="destructive" disabled={busy}>
              Destruir servidor
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Destruir {server.hostname}?</AlertDialogTitle>
              <AlertDialogDescription>Irreversível. Confirme o hostname antes.</AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancelar</AlertDialogCancel>
              <AlertDialogAction onClick={destroy}>Destruir</AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </CardContent>
    </Card>
  )
}
