import { useCallback, useMemo, useState } from 'react'
import { Link, NavLink, Outlet, useLocation, useNavigate, useParams } from 'react-router-dom'
import { ChevronRight, Copy, File, Folder, GitBranch, Lock } from 'lucide-react'
import { toast } from 'sonner'
import { api, type GitTreeEntry, type Project } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { canWriteAdminProduct, isAdminRole } from '@/lib/roles'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import {
  CiJobsCard,
  GitCard,
  MembersForm,
  MembersRead,
  MergeRequestsCard,
  ProjectServicesCard,
  RulesForm,
  RulesRead,
} from '@/pages/project-detail-page'

const TABS = [
  { to: '', label: 'Code', end: true },
  { to: 'mrs', label: 'Merge requests', end: false },
  { to: 'actions', label: 'Actions', end: false },
  { to: 'settings', label: 'Settings', end: false },
] as const

export function XgitRepoLayout() {
  const { slug = '' } = useParams()
  const location = useLocation()
  const fetchProject = useCallback(() => api.getProject(slug), [slug])
  const fetchMRs = useCallback(() => api.listMergeRequests(slug, 'open'), [slug])
  const { data, loading, error } = usePollingData(fetchProject, 20_000)
  const { data: mrs } = usePollingData(fetchMRs, 20_000)
  const openCount = mrs?.items?.length ?? 0

  if (loading || !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-48 w-full" />
  }

  return (
    <div className="flex flex-col gap-0">
      <div className="flex flex-wrap items-center justify-between gap-3 pb-3">
        <div className="flex flex-wrap items-center gap-2 text-sm">
          <Link to="/admin/xgit" className="text-primary hover:underline">
            ihuull
          </Link>
          <span className="text-muted-foreground">/</span>
          <span className="font-semibold">{data.slug}</span>
          <Badge variant="outline">{data.visibility}</Badge>
          {data.network === 'vpn' ? (
            <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
              <Lock className="size-3" /> vpn
            </span>
          ) : null}
        </div>
        <p className="text-sm text-muted-foreground">{data.name}</p>
      </div>
      <nav className="mb-6 flex flex-wrap gap-1 border-b border-border/60">
        {TABS.map((tab) => (
          <NavLink
            key={tab.label}
            to={tab.to}
            end={tab.end}
            className={({ isActive }) => {
              const path = location.pathname
              const base = `/admin/xgit/${data.slug}`
              const codeOn =
                tab.to === '' && (path === base || path.startsWith(`${base}/tree/`) || path.startsWith(`${base}/blob/`))
              const on = tab.to === '' ? codeOn : isActive
              return cn(
                'border-b-2 px-3 py-2 text-sm',
                on ? 'border-primary text-foreground' : 'border-transparent text-muted-foreground hover:text-foreground',
              )
            }}
          >
            {tab.label}
            {tab.to === 'mrs' && openCount > 0 ? (
              <span className="ml-1.5 rounded-full bg-muted px-1.5 text-xs">{openCount}</span>
            ) : null}
          </NavLink>
        ))}
      </nav>
      <Outlet context={{ slug, project: data }} />
    </div>
  )
}

export function XgitCodePage() {
  const { slug = '' } = useParams()
  const location = useLocation()
  const navigate = useNavigate()
  const { user } = useAuth()
  const canWrite = isAdminRole(user?.role) && canWriteAdminProduct(user?.role, user?.products, 'forge')
  const fetchBranches = useCallback(() => api.listProjectBranches(slug), [slug])
  const { data: branchData } = usePollingData(fetchBranches, 20_000)
  const branches = branchData?.items ?? []
  const [ref, setRef] = useState('')
  const activeRef = ref || (branches.includes('main') ? 'main' : branches[0] || 'HEAD')

  const filePath = useMemo(() => {
    const m = location.pathname.match(/\/blob\/(.*)$/)
    return m ? decodeURIComponent(m[1]) : ''
  }, [location.pathname])
  const dirPath = useMemo(() => {
    const m = location.pathname.match(/\/tree\/(.*)$/)
    return m ? decodeURIComponent(m[1]) : ''
  }, [location.pathname])

  const fetchTree = useCallback(
    () => api.listProjectTree(slug, activeRef, filePath ? parentPath(filePath) : dirPath),
    [slug, activeRef, filePath, dirPath],
  )
  const fetchBlob = useCallback(
    () => (filePath ? api.getProjectBlob(slug, filePath, activeRef) : Promise.resolve(null)),
    [slug, filePath, activeRef],
  )
  const fetchCommits = useCallback(() => api.listProjectCommits(slug, activeRef, dirPath || filePath), [slug, activeRef, dirPath, filePath])
  const { data: tree, loading: treeLoading } = usePollingData(fetchTree, 20_000)
  const { data: blob } = usePollingData(fetchBlob, 30_000)
  const { data: commits } = usePollingData(fetchCommits, 20_000)
  const fetchGit = useCallback(() => api.getProjectGit(slug), [slug])
  const { data: git } = usePollingData(fetchGit, 30_000)

  const readme = (tree?.items ?? []).find((e) => /^readme(\.md)?$/i.test(e.name) && e.type === 'blob')
  const fetchReadme = useCallback(
    () => (readme && !filePath ? api.getProjectBlob(slug, readme.path, activeRef) : Promise.resolve(null)),
    [slug, readme?.path, activeRef, filePath],
  )
  const { data: readmeBlob } = usePollingData(fetchReadme, 30_000)

  const crumbs = (filePath || dirPath).split('/').filter(Boolean)
  const lastCommit = commits?.items?.[0]
  const fetchProject = useCallback(() => api.getProject(slug), [slug])
  const { data: project } = usePollingData(fetchProject, 20_000)

  return (
    <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_260px]">
      <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-2">
        <Select value={activeRef} onValueChange={setRef}>
          <SelectTrigger className="field-glass w-44">
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
        <Button type="button" variant="outline" size="sm" onClick={() => navigate(`/admin/xgit/${slug}`)}>
          <GitBranch className="size-4" />
          Files
        </Button>
        {git?.clone_url ? (
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => {
              void navigator.clipboard.writeText(git.clone_url)
              toast.success('URL de clone copiada')
            }}
          >
            <Copy className="size-4" />
            Code
          </Button>
        ) : null}
      </div>

      <p className="flex flex-wrap items-center gap-1 text-sm">
        <Link to={`/admin/xgit/${slug}`} className="text-primary hover:underline">
          {slug}
        </Link>
        {crumbs.map((part, i) => {
          const sub = crumbs.slice(0, i + 1).join('/')
          const isLast = i === crumbs.length - 1
          return (
            <span key={sub} className="flex items-center gap-1">
              <ChevronRight className="size-3 text-muted-foreground" />
              {isLast ? (
                <span>{part}</span>
              ) : (
                <Link to={`/admin/xgit/${slug}/tree/${sub}`} className="text-primary hover:underline">
                  {part}
                </Link>
              )}
            </span>
          )
        })}
      </p>

      {lastCommit ? (
        <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
          <p>
            {lastCommit.author} · {lastCommit.subject} · {lastCommit.sha.slice(0, 7)}
          </p>
          <p>{commits?.items?.length ?? 0} commits</p>
        </div>
      ) : null}

      <div className="watch-complication overflow-hidden rounded-[18px]">
        {filePath && blob ? (
          blob.binary ? (
            <p className="p-5 text-sm text-muted-foreground">Arquivo binário — clone para abrir.</p>
          ) : (
            <pre className="overflow-x-auto p-5 font-mono text-xs leading-relaxed">{blob.content}</pre>
          )
        ) : treeLoading || !tree ? (
          <p className="p-5 text-sm text-muted-foreground">Carregando árvore…</p>
        ) : (tree.items ?? []).length === 0 ? (
          <p className="p-5 text-sm text-muted-foreground">
            Repositório vazio. Faça o primeiro push para <code className="font-mono text-xs">xgit.corp</code>.
          </p>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border/60 text-left text-xs text-muted-foreground">
                <th className="px-4 py-2 font-medium">Name</th>
                <th className="px-4 py-2 font-medium">Size</th>
              </tr>
            </thead>
            <tbody>
              {dirPath ? (
                <tr className="border-b border-border/40">
                  <td className="px-4 py-2" colSpan={2}>
                    <Link to={parentHref(slug, dirPath)} className="text-muted-foreground hover:underline">
                      ..
                    </Link>
                  </td>
                </tr>
              ) : null}
              {sortTree(tree.items).map((e) => (
                <tr key={e.path} className="border-b border-border/40 last:border-0">
                  <td className="px-4 py-2">
                    <Link
                      to={e.type === 'tree' ? `/admin/xgit/${slug}/tree/${e.path}` : `/admin/xgit/${slug}/blob/${e.path}`}
                      className="inline-flex items-center gap-2 hover:underline"
                    >
                      {e.type === 'tree' ? <Folder className="size-4 text-muted-foreground" /> : <File className="size-4 text-muted-foreground" />}
                      {e.name}
                    </Link>
                  </td>
                  <td className="px-4 py-2 text-xs text-muted-foreground">{e.type === 'tree' ? '—' : formatSize(e.size)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {readmeBlob && !filePath && !readmeBlob.binary ? (
        <div className="watch-complication rounded-[18px] p-5">
          <p className="hud-label mb-3 text-muted-foreground/70">README</p>
          <pre className="overflow-x-auto whitespace-pre-wrap font-mono text-xs leading-relaxed">{readmeBlob.content}</pre>
        </div>
      ) : null}

      {canWrite && git && !git.exists ? <GitCard slug={slug} username={user?.username ?? ''} canWrite={canWrite} /> : null}
      </div>
      <RepoAbout project={project ?? undefined} cloneUrl={git?.clone_url} />
    </div>
  )
}

function RepoAbout({ project, cloneUrl }: { project?: Project; cloneUrl?: string }) {
  if (!project) return null
  return (
    <aside className="flex flex-col gap-4">
      <div className="watch-complication rounded-[18px] p-4">
        <p className="hud-label mb-2 text-muted-foreground/70">About</p>
        <p className="text-sm">{project.description || project.name}</p>
        <div className="mt-3 flex flex-wrap gap-2">
          <Badge variant="outline">{project.visibility}</Badge>
          <Badge variant="outline">{project.network}</Badge>
        </div>
        <p className="mt-3 text-xs text-muted-foreground">{project.member_count} contributors</p>
        {cloneUrl ? <p className="mt-2 break-all font-mono text-[11px] text-muted-foreground">{cloneUrl}</p> : null}
      </div>
    </aside>
  )
}

export function XgitMrsPage() {
  const { slug = '' } = useParams()
  const { user } = useAuth()
  const canWrite = isAdminRole(user?.role) && canWriteAdminProduct(user?.role, user?.products, 'forge')
  const fetchProject = useCallback(() => api.getProject(slug), [slug])
  const { data } = usePollingData(fetchProject, 20_000)
  if (!data) return <Skeleton className="h-32 w-full" />
  return <MergeRequestsCard slug={slug} members={data.members ?? []} userId={user?.id} canWrite={canWrite} />
}

export function XgitActionsPage() {
  const { slug = '' } = useParams()
  return (
    <div className="flex flex-col gap-6">
      <CiJobsCard slug={slug} />
      <ProjectServicesCard slug={slug} />
    </div>
  )
}

export function XgitRepoSettingsPage() {
  const { slug = '' } = useParams()
  const { user } = useAuth()
  const canWrite = isAdminRole(user?.role) && canWriteAdminProduct(user?.role, user?.products, 'forge')
  const fetchProject = useCallback(() => api.getProject(slug), [slug])
  const { data, reload } = usePollingData(fetchProject, 20_000)
  if (!data) return <Skeleton className="h-32 w-full" />
  return (
    <div className="grid gap-6 lg:grid-cols-[220px_1fr]">
      <aside className="flex flex-col gap-1 text-sm">
        <p className="hud-label px-2 text-muted-foreground/70">Settings</p>
        <a href="#general" className="rounded-md bg-muted/40 px-2 py-1.5 hover:bg-muted/60">
          General
        </a>
        <a href="#collaborators" className="px-2 py-1.5 text-muted-foreground hover:text-foreground">
          Collaborators
        </a>
        <a href="#branches" className="px-2 py-1.5 text-muted-foreground hover:text-foreground">
          Branches
        </a>
      </aside>
      <div className="flex flex-col gap-6">
        <section id="general">{canWrite ? <RulesForm project={data} onSaved={reload} /> : <RulesRead project={data} />}</section>
        <section id="collaborators">
          {canWrite ? <MembersForm project={data} onSaved={reload} /> : <MembersRead project={data} />}
        </section>
        <section id="branches">
          <GitCard slug={slug} username={user?.username ?? ''} canWrite={canWrite} />
        </section>
      </div>
    </div>
  )
}

function sortTree(items: GitTreeEntry[]) {
  return [...items].sort((a, b) => {
    if (a.type !== b.type) return a.type === 'tree' ? -1 : 1
    return a.name.localeCompare(b.name)
  })
}

function parentPath(path: string) {
  const i = path.lastIndexOf('/')
  return i < 0 ? '' : path.slice(0, i)
}

function parentHref(slug: string, dir: string) {
  const p = parentPath(dir)
  return p ? `/admin/xgit/${slug}/tree/${p}` : `/admin/xgit/${slug}`
}

function formatSize(n: number) {
  if (!n) return '—'
  if (n < 1024) return `${n} B`
  return `${(n / 1024).toFixed(1)} KB`
}
