import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { Link, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import { api, ApiError, type Project, type ProjectRole, type User } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { canWriteAdminProduct, isAdminRole } from '@/lib/roles'
import { XGROUP_CORP_ORIGIN } from '@/lib/product-host'
import { UserPicker } from '@/components/user-picker'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Checkbox } from '@/components/ui/checkbox'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'

const PROJECT_ROLES: ProjectRole[] = ['guest', 'reporter', 'developer', 'maintainer', 'owner']

const ROLE_LABELS: Record<ProjectRole, string> = {
  guest: 'Guest',
  reporter: 'Reporter',
  developer: 'Developer',
  maintainer: 'Maintainer',
  owner: 'Owner',
}

export function ProjectDetailPage() {
  const { slug = '' } = useParams()
  const { user } = useAuth()
  const canWrite = isAdminRole(user?.role) && canWriteAdminProduct(user?.role, user?.products, 'forge')
  const fetchProject = useCallback(() => api.getProject(slug), [slug])
  const { data, loading, error, reload } = usePollingData(fetchProject, 20_000)

  if (loading || !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-48 w-full" />
  }

  return (
    <div className="flex flex-col gap-6">
      <p className="text-sm text-muted-foreground">
        <Link to="/admin/projects" className="hover:underline">
          Projetos
        </Link>
        <span className="px-1.5">/</span>
        <span className="text-foreground">{data.slug}</span>
      </p>

      {canWrite ? <RulesForm project={data} onSaved={reload} /> : <RulesRead project={data} />}
      {canWrite ? <MembersForm project={data} onSaved={reload} /> : <MembersRead project={data} />}
    </div>
  )
}

function RulesRead({ project }: { project: Project }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{project.name}</CardTitle>
        <CardDescription>{project.description || 'Sem descrição.'}</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-wrap gap-2">
        <Badge variant="outline">{project.visibility}</Badge>
        <Badge variant="outline">{project.network}</Badge>
        <Badge variant={project.files_enabled ? 'secondary' : 'outline'}>
          arquivos {project.files_enabled ? 'on' : 'off'}
        </Badge>
        {project.runners.length > 0 ? <Badge variant="outline">runners: {project.runners.join(', ')}</Badge> : null}
        <GroupLink id={project.social_group_id} />
      </CardContent>
    </Card>
  )
}

function RulesForm({ project, onSaved }: { project: Project; onSaved: () => void }) {
  const [name, setName] = useState(project.name)
  const [description, setDescription] = useState(project.description)
  const [visibility, setVisibility] = useState(project.visibility)
  const [network, setNetwork] = useState(project.network)
  const [files, setFiles] = useState(project.files_enabled)
  const [runners, setRunners] = useState(project.runners.join(', '))
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    setName(project.name)
    setDescription(project.description)
    setVisibility(project.visibility)
    setNetwork(project.network)
    setFiles(project.files_enabled)
    setRunners(project.runners.join(', '))
  }, [project])

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      await api.updateProject(project.slug, {
        name: name.trim(),
        description: description.trim(),
        visibility,
        network,
        files_enabled: files,
        runners: runners
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean),
      })
      toast.success('Regras salvas')
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
        <CardTitle className="text-base">Regras</CardTitle>
        <CardDescription>
          Visibility e network do projeto. Arquivos abrem o share no XDRIVER — sem FileBrowser.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={submit} className="grid gap-4 sm:grid-cols-2">
          <div className="space-y-1.5">
            <Label htmlFor="name">Nome</Label>
            <Input id="name" value={name} onChange={(e) => setName(e.target.value)} required />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="desc">Descrição</Label>
            <Input id="desc" value={description} onChange={(e) => setDescription(e.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label>Visibility</Label>
            <Select value={visibility} onValueChange={(v) => setVisibility(v as Project['visibility'])}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="global">global</SelectItem>
                <SelectItem value="restricted">restricted</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1.5">
            <Label>Network</Label>
            <Select value={network} onValueChange={(v) => setNetwork(v as Project['network'])}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="vpn">vpn</SelectItem>
                <SelectItem value="public">public</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1.5 sm:col-span-2">
            <Label htmlFor="runners">Runners (vírgula)</Label>
            <Input id="runners" value={runners} onChange={(e) => setRunners(e.target.value)} placeholder="runner" />
          </div>
          <label className="flex items-center gap-2 text-sm">
            <Checkbox checked={files} onCheckedChange={(v) => setFiles(v === true)} />
            Arquivos no XDRIVER
          </label>
          <div className="flex flex-wrap items-center gap-3 sm:col-span-2">
            <Button type="submit" disabled={busy}>
              {busy ? 'Salvando…' : 'Salvar regras'}
            </Button>
            <GroupLink id={project.social_group_id} />
          </div>
        </form>
      </CardContent>
    </Card>
  )
}

function MembersRead({ project }: { project: Project }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Membros</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-2">
        {(project.members ?? []).map((m) => (
          <div key={m.user_id} className="flex items-center justify-between gap-2 text-sm">
            <span>{m.username}</span>
            <Badge variant="outline">{ROLE_LABELS[m.role]}</Badge>
          </div>
        ))}
      </CardContent>
    </Card>
  )
}

function MembersForm({ project, onSaved }: { project: Project; onSaved: () => void }) {
  const [users, setUsers] = useState<User[]>([])
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const [roles, setRoles] = useState<Record<number, ProjectRole>>({})
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    const next = new Set<number>()
    const nextRoles: Record<number, ProjectRole> = {}
    for (const m of project.members ?? []) {
      next.add(m.user_id)
      nextRoles[m.user_id] = m.role
    }
    setSelected(next)
    setRoles(nextRoles)
  }, [project])

  useEffect(() => {
    api
      .listUsers({ per_page: 100 })
      .then((page) => setUsers(page.items))
      .catch(() => setUsers([]))
  }, [])

  function toggle(id: number) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
        setRoles((r) => ({ ...r, [id]: r[id] ?? 'developer' }))
      }
      return next
    })
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    const members = Array.from(selected).map((user_id) => ({
      user_id,
      role: roles[user_id] ?? 'developer',
    }))
    if (!members.some((m) => m.role === 'owner')) {
      toast.error('O projeto precisa de pelo menos um owner')
      return
    }
    setBusy(true)
    try {
      await api.setProjectMembers(project.slug, members)
      toast.success('Membros atualizados')
      onSaved()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao salvar membros')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Membros</CardTitle>
        <CardDescription>O mesmo conjunto entra no grupo XGROUP do slug.</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={submit} className="flex flex-col gap-4">
          <UserPicker users={users} selected={selected} onToggle={toggle} />
          <div className="flex flex-col gap-2">
            {Array.from(selected).map((id) => {
              const u = users.find((row) => row.id === id)
              return (
                <div key={id} className="flex items-center gap-3">
                  <span className="min-w-0 flex-1 truncate text-sm">{u?.username ?? `#${id}`}</span>
                  <Select
                    value={roles[id] ?? 'developer'}
                    onValueChange={(v) => setRoles((r) => ({ ...r, [id]: v as ProjectRole }))}
                  >
                    <SelectTrigger className="w-40">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {PROJECT_ROLES.map((r) => (
                        <SelectItem key={r} value={r}>
                          {ROLE_LABELS[r]}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              )
            })}
          </div>
          <Button type="submit" disabled={busy}>
            {busy ? 'Salvando…' : 'Salvar membros'}
          </Button>
        </form>
      </CardContent>
    </Card>
  )
}

function GroupLink({ id }: { id: number }) {
  return (
    <a
      href={`${XGROUP_CORP_ORIGIN}/social/groups`}
      className="text-sm text-primary hover:underline"
      target="_blank"
      rel="noreferrer"
    >
      Grupo XGROUP #{id}
    </a>
  )
}
