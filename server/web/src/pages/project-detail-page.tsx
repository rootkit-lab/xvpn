import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { Link, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import { api, ApiError, getToken, type MergeRequest, type Project, type ProjectRole, type User } from '@/lib/api'
import { ServiceStatusBadge } from '@/pages/services-page'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { canWriteAdminProduct, isAdminRole } from '@/lib/roles'
import { XGROUP_CORP_ORIGIN } from '@/lib/product-host'
import { xgitPath, xgitReposPath } from '@/lib/xgit'
import { UserPicker } from '@/components/user-picker'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Checkbox } from '@/components/ui/checkbox'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { StatusBadge } from '@/pages/merge-request-page'
import { CiJobStatusBadge } from '@/pages/ci-job-page'

const PROJECT_ROLES: ProjectRole[] = ['guest', 'reporter', 'developer', 'maintainer', 'owner']

const ROLE_LABELS: Record<ProjectRole, string> = {
  guest: 'Guest',
  reporter: 'Reporter',
  developer: 'Developer',
  maintainer: 'Maintainer',
  owner: 'Owner',
}

export function ProjectDetailPage() {
  const { org = '', slug = '' } = useParams()
  const { user } = useAuth()
  const canWrite = isAdminRole(user?.role) && canWriteAdminProduct(user?.role, user?.products, 'forge')
  const fetchProject = useCallback(() => api.getProject(`${org}/${slug}`), [slug])
  const { data, loading, error, reload } = usePollingData(fetchProject, 20_000)

  if (loading || !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-48 w-full" />
  }

  return (
    <div className="flex flex-col gap-6">
      <p className="text-sm text-muted-foreground">
        <Link to={xgitReposPath()} className="hover:underline">
          XGIT
        </Link>
        <span className="px-1.5">/</span>
        <span className="text-foreground">{data.slug}</span>
      </p>

      {canWrite ? <RulesForm project={data} onSaved={reload} /> : <RulesRead project={data} />}
      <GitCard slug={data.slug} username={user?.username ?? ''} canWrite={canWrite} />
      <MergeRequestsCard
        slug={data.slug}
        members={data.members ?? []}
        userId={user?.id}
        canWrite={canWrite}
      />
      <CiJobsCard slug={data.slug} />
      <ProjectServicesCard slug={data.slug} />
      {canWrite ? <MembersForm project={data} onSaved={reload} /> : <MembersRead project={data} />}
    </div>
  )
}

export function RulesRead({ project }: { project: Project }) {
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

export function RulesForm({ project, onSaved }: { project: Project; onSaved: () => void }) {
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

export function MembersRead({ project }: { project: Project }) {
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

export function MembersForm({ project, onSaved }: { project: Project; onSaved: () => void }) {
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

export function GitCard({ slug, username, canWrite }: { slug: string; username: string; canWrite: boolean }) {
  const fetchGit = useCallback(() => api.getProjectGit(`${org}/${slug}`), [slug])
  const { data, loading, error, reload } = usePollingData(fetchGit, 20_000)
  const [pattern, setPattern] = useState('main')
  const [busy, setBusy] = useState(false)

  async function initRepo() {
    setBusy(true)
    try {
      await api.initProjectGit(`${org}/${slug}`)
      toast.success('Repositório criado')
      reload()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao criar o repo')
    } finally {
      setBusy(false)
    }
  }

  async function copyClone() {
    if (!data) return
    const token = getToken()
    const url =
      username && token
        ? data.clone_url.replace('https://', `https://${encodeURIComponent(username)}@`)
        : data.clone_url
    const cmd = `git clone ${url}`
    try {
      await navigator.clipboard.writeText(cmd)
      toast.success(token ? 'Comando copiado — senha = a da conta ihuull' : 'URL copiada')
    } catch {
      toast.error('Não foi possível copiar')
    }
  }

  async function addProtected(e: FormEvent) {
    e.preventDefault()
    if (!data) return
    const next = pattern.trim()
    if (!next) return
    setBusy(true)
    try {
      await api.setProtectedBranches(`${org}/${slug}`, [...data.protected_branches, { pattern: next, min_push_role: 'maintainer' }])
      setPattern('')
      toast.success('Branch protegida')
      reload()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao proteger')
    } finally {
      setBusy(false)
    }
  }

  async function removeProtected(patternName: string) {
    if (!data) return
    setBusy(true)
    try {
      await api.setProtectedBranches(
        slug,
        data.protected_branches.filter((b) => b.pattern !== patternName),
      )
      toast.success('Proteção removida')
      reload()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao remover')
    } finally {
      setBusy(false)
    }
  }

  if (loading || !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-32 w-full" />
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Git</CardTitle>
        <CardDescription>
          Clone só em <code className="font-mono text-xs">xgit.corp</code> com VPN. Usuário e senha da conta.
          Guest/reporter leem; developer faz push; maintainer em branch protegida.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <div className="flex flex-wrap items-center gap-2">
          <code className="font-mono text-xs">{data.clone_url}</code>
          <Badge variant={data.exists ? 'secondary' : 'outline'}>{data.exists ? 'bare ok' : 'sem repo'}</Badge>
          <Button type="button" variant="outline" size="sm" onClick={() => void copyClone()}>
            Copiar clone
          </Button>
          {canWrite && !data.exists ? (
            <Button type="button" size="sm" disabled={busy} onClick={() => void initRepo()}>
              {busy ? 'Criando…' : 'Criar repositório'}
            </Button>
          ) : null}
        </div>
        <div className="flex flex-col gap-2">
          <p className="text-sm font-medium">Branches protegidas</p>
          {(data.protected_branches ?? []).map((b) => (
            <div key={b.pattern} className="flex items-center justify-between gap-2 text-sm">
              <span className="font-mono text-xs">
                {b.pattern}{' '}
                <span className="text-muted-foreground">({b.min_push_role}+)</span>
              </span>
              {canWrite ? (
                <Button type="button" variant="ghost" size="sm" disabled={busy} onClick={() => void removeProtected(b.pattern)}>
                  Remover
                </Button>
              ) : null}
            </div>
          ))}
          {canWrite ? (
            <form onSubmit={addProtected} className="flex flex-wrap items-end gap-2">
              <div className="space-y-1.5">
                <Label htmlFor="protect">Padrão</Label>
                <Input
                  id="protect"
                  value={pattern}
                  onChange={(e) => setPattern(e.target.value)}
                  placeholder="release/*"
                />
              </div>
              <Button type="submit" disabled={busy}>
                Proteger
              </Button>
            </form>
          ) : null}
        </div>
      </CardContent>
    </Card>
  )
}

const PROJECT_ROLE_RANK: Record<ProjectRole, number> = {
  guest: 0,
  reporter: 1,
  developer: 2,
  maintainer: 3,
  owner: 4,
}

export function MergeRequestsCard({
  slug,
  members,
  userId,
  canWrite,
}: {
  slug: string
  members: { user_id: number; role: ProjectRole }[]
  userId?: number
  canWrite: boolean
}) {
  const fetchMRs = useCallback(() => api.listMergeRequests(`${org}/${slug}`), [slug])
  const fetchBranches = useCallback(() => api.listProjectBranches(`${org}/${slug}`), [slug])
  const { data, loading, error, reload } = usePollingData(fetchMRs, 20_000)
  const { data: branchData } = usePollingData(fetchBranches, 20_000)
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [source, setSource] = useState('')
  const [target, setTarget] = useState('main')
  const [busy, setBusy] = useState(false)

  const myRole = members.find((m) => m.user_id === userId)?.role
  const canCreate = canWrite || (myRole != null && PROJECT_ROLE_RANK[myRole] >= PROJECT_ROLE_RANK.developer)
  const branches = branchData?.items ?? []

  useEffect(() => {
    const heads = branchData?.items
    if (!heads?.length) return
    setSource((s) => s || heads.find((b) => b !== 'main' && b !== 'master') || heads[0] || '')
    setTarget((t) => {
      if (t && heads.includes(t)) return t
      if (heads.includes('main')) return 'main'
      if (heads.includes('master')) return 'master'
      return heads[0] ?? ''
    })
  }, [branchData])

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      const mr = await api.createMergeRequest(`${org}/${slug}`, {
        title: title.trim(),
        description: description.trim() || undefined,
        source_branch: source,
        target_branch: target,
      })
      toast.success(`MR !${mr.number} aberto`)
      setTitle('')
      setDescription('')
      reload()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao abrir MR')
    } finally {
      setBusy(false)
    }
  }

  if (loading || !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-32 w-full" />
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Pull requests</CardTitle>
        <CardDescription>
          Review no XCHAT (uma thread por MR). Comentários de issue no XGROUP. Merge em branch protegida exige
          maintainer+.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {(data.items ?? []).length === 0 ? (
          <p className="text-sm text-muted-foreground">Nenhum MR ainda.</p>
        ) : (
          <div className="flex flex-col gap-2">
            {data.items.map((mr: MergeRequest) => (
              <Link
                key={mr.number}
                to={xgitPath(`${org}/${slug}/pulls/${mr.number}`)}
                className="flex items-center justify-between gap-2 text-sm hover:underline"
              >
                <span className="min-w-0 truncate">
                  !{mr.number} {mr.title}{' '}
                  <span className="font-mono text-xs text-muted-foreground">
                    {mr.source_branch} → {mr.target_branch}
                  </span>
                </span>
                <StatusBadge status={mr.status} />
              </Link>
            ))}
          </div>
        )}
        {canCreate ? (
          branches.length < 2 ? (
            <p className="text-sm text-muted-foreground">Faça push de pelo menos duas branches para abrir um MR.</p>
          ) : (
            <form onSubmit={submit} className="grid gap-3 sm:grid-cols-2">
              <div className="space-y-1.5 sm:col-span-2">
                <Label htmlFor="mr-title">Título</Label>
                <Input
                  id="mr-title"
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                  required
                  maxLength={120}
                />
              </div>
              <div className="space-y-1.5 sm:col-span-2">
                <Label htmlFor="mr-desc">Descrição</Label>
                <Textarea id="mr-desc" value={description} onChange={(e) => setDescription(e.target.value)} rows={3} />
              </div>
              <div className="space-y-1.5">
                <Label>Source</Label>
                <Select value={source} onValueChange={setSource}>
                  <SelectTrigger>
                    <SelectValue placeholder="branch" />
                  </SelectTrigger>
                  <SelectContent>
                    {branches.map((b) => (
                      <SelectItem key={b} value={b}>
                        {b}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label>Target</Label>
                <Select value={target} onValueChange={setTarget}>
                  <SelectTrigger>
                    <SelectValue placeholder="branch" />
                  </SelectTrigger>
                  <SelectContent>
                    {branches.map((b) => (
                      <SelectItem key={b} value={b}>
                        {b}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="sm:col-span-2">
                <Button type="submit" disabled={busy || !source || source === target}>
                  {busy ? 'Abrindo…' : 'Abrir MR'}
                </Button>
              </div>
            </form>
          )
        ) : null}
      </CardContent>
    </Card>
  )
}

export function CiJobsCard({ slug }: { slug: string }) {
  const fetchJobs = useCallback(() => api.listCiJobs(`${org}/${slug}`), [slug])
  const { data, loading, error } = usePollingData(fetchJobs, 10_000)

  if (loading || !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-24 w-full" />
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Pipeline</CardTitle>
        <CardDescription>
          Push e merge disparam um job. A execução é num peer <code className="font-mono text-xs">runner</code>, não
          no xvpn-server. Log e artifact ficam no XDRIVER do projeto.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-2">
        {(data.items ?? []).length === 0 ? (
          <p className="text-sm text-muted-foreground">Nenhum job ainda. Faça push ou mergeie um MR.</p>
        ) : (
          data.items.map((job) => (
            <Link
              key={job.number}
              to={xgitPath(`${org}/${slug}/actions/${job.number}`)}
              className="flex items-center justify-between gap-2 text-sm hover:underline"
            >
              <span className="min-w-0 truncate">
                #{job.number} {job.trigger}{' '}
                <span className="font-mono text-xs text-muted-foreground">{job.ref.replace('refs/heads/', '')}</span>
              </span>
              <CiJobStatusBadge status={job.status} />
            </Link>
          ))
        )}
      </CardContent>
    </Card>
  )
}

export function ProjectServicesCard({ slug }: { slug: string }) {
  const fetchServices = useCallback(() => api.listProjectServices(`${org}/${slug}`), [slug])
  const { data, loading, error } = usePollingData(fetchServices, 15_000)

  if (loading || !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-24 w-full" />
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Serviços</CardTitle>
        <CardDescription>
          Instâncias orquestradas para este projeto. DNS <code className="font-mono text-xs">svc-*.corp</code> só com
          bind wg0. Senha não aparece aqui.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-2">
        {(data.items ?? []).length === 0 ? (
          <p className="text-sm text-muted-foreground">
            Nenhum serviço ligado. Crie em{' '}
            <Link to="/admin/services" className="hover:underline">
              Serviços
            </Link>
            .
          </p>
        ) : (
          data.items.map((svc) => (
            <Link
              key={svc.slug}
              to={`/admin/services/${svc.slug}`}
              className="flex items-center justify-between gap-2 text-sm hover:underline"
            >
              <span className="min-w-0 truncate font-mono text-xs">{svc.endpoint || svc.slug}</span>
              <ServiceStatusBadge status={svc.status} />
            </Link>
          ))
        )}
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
