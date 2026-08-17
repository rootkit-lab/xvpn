import { useCallback, useMemo, useState } from 'react'
import { Link, NavLink, Outlet, useLocation, useNavigate, useParams } from 'react-router-dom'
import { BookOpen, ChevronRight, Clock, Code2, Copy, Download, File, Folder, GitBranch, Lock, Pencil, Scale, Shield, Tag } from 'lucide-react'
import { toast } from 'sonner'
import { api, type GitLangStat, type GitTreeEntry, type Project } from '@/lib/api'
import { formatRelativeTime } from '@/lib/format'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { canWriteAdminProduct, isAdminRole } from '@/lib/roles'
import { xgitPath, xgitReposPath } from '@/lib/xgit'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { cn } from '@/lib/utils'
import {
  GitCard,
  MembersForm,
  MembersRead,
  RulesForm,
  RulesRead,
} from '@/pages/project-detail-page'

const TABS = [
  { to: '', label: 'Code', end: true },
  { to: 'issues', label: 'Issues', end: false },
  { to: 'pulls', label: 'Pull requests', end: false },
  { to: 'actions', label: 'Actions', end: false },
  { to: 'settings', label: 'Settings', end: false },
] as const

export function XgitRepoLayout() {
  const { slug = '' } = useParams()
  const location = useLocation()
  const fetchProject = useCallback(() => api.getProject(slug), [slug])
  const fetchMRs = useCallback(() => api.listMergeRequests(slug, 'open'), [slug])
  const fetchIssues = useCallback(() => api.listIssues(slug, { status: 'open' }), [slug])
  const { data, loading, error } = usePollingData(fetchProject, 20_000)
  const { data: mrs } = usePollingData(fetchMRs, 20_000)
  const { data: issues } = usePollingData(fetchIssues, 20_000)
  const openCount = mrs?.items?.length ?? 0
  const issueCount = issues?.items?.length ?? 0

  if (loading || !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-48 w-full" />
  }

  return (
    <div className="flex flex-col gap-0">
      <div className="flex flex-wrap items-center justify-between gap-3 pb-3">
        <div className="flex flex-wrap items-center gap-2 text-sm">
          <Link to={xgitReposPath()} className="text-primary hover:underline">
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
              const base = xgitPath(data.slug)
              const codeOn =
                tab.to === '' &&
                (path === base ||
                  path.startsWith(`${base}/tree/`) ||
                  path.startsWith(`${base}/blob/`) ||
                  path.startsWith(`${base}/edit/`) ||
                  path.startsWith(`${base}/commits`))
              const pullsOn =
                tab.to === 'pulls' && (path.includes('/pulls') || path.includes('/mrs'))
              const on = tab.to === '' ? codeOn : tab.to === 'pulls' ? pullsOn : isActive
              return cn(
                'border-b-2 px-3 py-2 text-sm',
                on ? 'border-primary text-foreground' : 'border-transparent text-muted-foreground hover:text-foreground',
              )
            }}
          >
            {tab.label}
            {tab.to === 'issues' && issueCount > 0 ? (
              <span className="ml-1.5 rounded-full bg-muted px-1.5 text-xs">{issueCount}</span>
            ) : null}
            {tab.to === 'pulls' && openCount > 0 ? (
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
  const [goTo, setGoTo] = useState('')
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
  const fetchCommits = useCallback(
    () => api.listProjectCommits(slug, activeRef, dirPath || filePath),
    [slug, activeRef, dirPath, filePath],
  )
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
  const tags = tree?.tags ?? []
  const commitCount = tree?.commit_count ?? commits?.items?.length ?? 0
  const goMatches = (tree?.items ?? []).filter((e) => e.name.toLowerCase().includes(goTo.trim().toLowerCase()))

  function jumpToFile(name: string) {
    const hit = (tree?.items ?? []).find((e) => e.name === name) ?? goMatches[0]
    if (!hit) return
    setGoTo('')
    navigate(hit.type === 'tree' ? xgitPath(`${slug}/tree/${hit.path}`) : xgitPath(`${slug}/blob/${hit.path}`))
  }

  return (
    <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_280px]">
      <div className="flex min-w-0 flex-col gap-4">
        <div className="flex flex-wrap items-center gap-2">
          <Select value={activeRef} onValueChange={setRef}>
            <SelectTrigger className="field-glass w-44">
              <span className="inline-flex items-center gap-1.5">
                <GitBranch className="size-3.5" />
                <SelectValue placeholder="branch" />
              </span>
            </SelectTrigger>
            <SelectContent>
              {branches.map((b) => (
                <SelectItem key={b} value={b}>
                  {b}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button type="button" variant="ghost" size="sm" className="text-muted-foreground">
                <GitBranch className="size-3.5" />
                {branches.length} {branches.length === 1 ? 'branch' : 'branches'}
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start">
              {branches.map((b) => (
                <DropdownMenuItem key={b} onClick={() => setRef(b)}>
                  {b}
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button type="button" variant="ghost" size="sm" className="text-muted-foreground">
                <Tag className="size-3.5" />
                {tags.length} {tags.length === 1 ? 'tag' : 'tags'}
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start">
              {tags.length === 0 ? (
                <DropdownMenuItem disabled>Nenhuma tag</DropdownMenuItem>
              ) : (
                tags.map((t) => (
                  <DropdownMenuItem key={t} onClick={() => setRef(t)}>
                    {t}
                  </DropdownMenuItem>
                ))
              )}
            </DropdownMenuContent>
          </DropdownMenu>
          <div className="relative min-w-[12rem] flex-1">
            <Input
              className="field-glass h-8"
              value={goTo}
              onChange={(e) => setGoTo(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && goTo.trim()) jumpToFile(goTo.trim())
              }}
              placeholder="Go to file"
              aria-label="Go to file"
            />
            {goTo.trim() && goMatches.length > 0 ? (
              <ul className="watch-complication absolute z-10 mt-1 max-h-56 w-full overflow-auto rounded-xl py-1 text-sm">
                {goMatches.slice(0, 12).map((e) => (
                  <li key={e.path}>
                    <button
                      type="button"
                      className="flex w-full items-center gap-2 px-3 py-1.5 text-left hover:bg-muted/40"
                      onClick={() => jumpToFile(e.name)}
                    >
                      {e.type === 'tree' ? <Folder className="size-3.5" /> : <File className="size-3.5" />}
                      {e.name}
                    </button>
                  </li>
                ))}
              </ul>
            ) : null}
          </div>
          {git?.clone_url ? (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button type="button" size="sm" className="btn-glow">
                  <Code2 className="size-4" />
                  Code
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-80">
                <DropdownMenuLabel>Local</DropdownMenuLabel>
                <p className="px-2 pb-1 text-xs text-muted-foreground">Clone HTTPS</p>
                <div className="flex items-center gap-2 px-2 pb-2">
                  <code className="min-w-0 flex-1 truncate font-mono text-[11px]">{git.clone_url}</code>
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    onClick={() => {
                      void navigator.clipboard.writeText(git.clone_url)
                      toast.success('URL de clone copiada')
                    }}
                  >
                    <Copy className="size-3.5" />
                  </Button>
                </div>
                <p className="px-2 pb-2 font-mono text-[11px] text-muted-foreground">git clone {git.clone_url}</p>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  onClick={() => {
                    void api.downloadProjectArchive(slug, activeRef).then(
                      () => toast.success('Download iniciado'),
                      (err: unknown) => toast.error(err instanceof Error ? err.message : 'Falha no ZIP'),
                    )
                  }}
                >
                  <Download className="size-3.5" />
                  Download ZIP
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          ) : null}
        </div>

        <p className="flex flex-wrap items-center gap-1 text-sm">
          <Link to={xgitPath(slug)} className="text-primary hover:underline">
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
                  <Link to={xgitPath(`${slug}/tree/${sub}`)} className="text-primary hover:underline">
                    {part}
                  </Link>
                )}
              </span>
            )
          })}
        </p>

        <div className="watch-complication overflow-hidden rounded-[18px]">
          {lastCommit ? (
            <div className="flex flex-wrap items-center justify-between gap-2 border-b border-border/60 px-4 py-2.5 text-xs">
              <p className="min-w-0 truncate">
                <span className="font-medium text-foreground">{lastCommit.author}</span>
                <span className="text-muted-foreground"> {lastCommit.subject}</span>
                <span className="ml-2 font-mono text-muted-foreground">{lastCommit.sha.slice(0, 7)}</span>
              </p>
              <Link
                to={xgitPath(`${slug}/commits`)}
                className="inline-flex shrink-0 items-center gap-1 text-muted-foreground hover:text-foreground"
              >
                <Clock className="size-3.5" />
                {commitCount} commits
              </Link>
            </div>
          ) : null}
          {filePath && blob ? (
            blob.binary ? (
              <p className="p-5 text-sm text-muted-foreground">Arquivo binário — clone para abrir.</p>
            ) : (
              <div>
                <div className="flex justify-end border-b border-border/60 px-3 py-1.5">
                  <Button
                    type="button"
                    size="sm"
                    variant="ghost"
                    onClick={() => navigate(xgitPath(`${slug}/edit/${activeRef}/${filePath}`))}
                  >
                    <Pencil className="size-3.5" />
                    Edit
                  </Button>
                </div>
                <pre className="overflow-x-auto p-5 font-mono text-xs leading-relaxed">{blob.content}</pre>
              </div>
            )
          ) : treeLoading || !tree ? (
            <p className="p-5 text-sm text-muted-foreground">Carregando árvore…</p>
          ) : (tree.items ?? []).length === 0 ? (
            <p className="p-5 text-sm text-muted-foreground">
              Repositório vazio. Faça o primeiro push para <code className="font-mono text-xs">xgit.corp</code>.
            </p>
          ) : (
            <table className="w-full text-sm">
              <tbody>
                {dirPath ? (
                  <tr className="border-b border-border/40">
                    <td className="px-4 py-2" colSpan={3}>
                      <Link to={parentHref(slug, dirPath)} className="text-muted-foreground hover:underline">
                        ..
                      </Link>
                    </td>
                  </tr>
                ) : null}
                {sortTree(tree.items).map((e) => (
                  <tr key={e.path} className="border-b border-border/40 last:border-0 hover:bg-muted/20">
                    <td className="w-[32%] px-4 py-2">
                      <span className="inline-flex items-center gap-2">
                        <Link
                          to={e.type === 'tree' ? xgitPath(`${slug}/tree/${e.path}`) : xgitPath(`${slug}/blob/${e.path}`)}
                          className="inline-flex items-center gap-2 hover:underline"
                        >
                          {e.type === 'tree' ? (
                            <Folder className="size-4 text-muted-foreground" />
                          ) : (
                            <File className="size-4 text-muted-foreground" />
                          )}
                          {e.name}
                        </Link>
                        {e.type === 'blob' ? (
                          <button
                            type="button"
                            className="text-muted-foreground hover:text-foreground"
                            title="Edit"
                            onClick={() => navigate(xgitPath(`${slug}/edit/${activeRef}/${e.path}`))}
                          >
                            <Pencil className="size-3.5" />
                          </button>
                        ) : null}
                      </span>
                    </td>
                    <td className="px-4 py-2 text-xs text-muted-foreground">
                      {e.last_commit ? (
                        <span className="line-clamp-1">{e.last_commit.subject}</span>
                      ) : (
                        '—'
                      )}
                    </td>
                    <td className="w-[18%] px-4 py-2 text-right text-xs text-muted-foreground">
                      {e.last_commit?.date ? formatRelativeTime(e.last_commit.date) : '—'}
                    </td>
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
      <RepoAbout
        project={project ?? undefined}
        cloneUrl={git?.clone_url}
        files={tree?.items ?? []}
        languages={tree?.languages ?? []}
        slug={slug}
      />
    </div>
  )
}

const LANG_COLOR: Record<string, string> = {
  Go: '#00add8',
  TypeScript: '#3178c6',
  JavaScript: '#f1e05a',
  Shell: '#89e051',
  SCSS: '#c6538c',
  CSS: '#563d7c',
  Markdown: '#083fa1',
  YAML: '#cb171e',
  JSON: '#292929',
  Python: '#3572a5',
  HTML: '#e34c26',
  NSIS: '#01ff70',
}

function RepoAbout({
  project,
  cloneUrl,
  files,
  languages,
  slug,
}: {
  project?: Project
  cloneUrl?: string
  files: GitTreeEntry[]
  languages: GitLangStat[]
  slug: string
}) {
  if (!project) return null
  const readmeFile = files.find((e) => /^readme(\.md)?$/i.test(e.name))
  const licenseFile = files.find((e) => /^license(\.md)?$/i.test(e.name))
  const securityFile = files.find((e) => /^security\.md$/i.test(e.name))
  return (
    <aside className="flex flex-col gap-5">
      <div>
        <p className="hud-label mb-2 text-muted-foreground/70">About</p>
        <p className="text-sm leading-relaxed">{project.description || project.name}</p>
        <div className="mt-3 flex flex-col gap-1.5 text-sm">
          {readmeFile ? (
            <Link to={xgitPath(`${slug}/blob/${readmeFile.path}`)} className="inline-flex items-center gap-2 text-muted-foreground hover:text-foreground">
              <BookOpen className="size-3.5" />
              README
            </Link>
          ) : null}
          {licenseFile ? (
            <Link to={xgitPath(`${slug}/blob/${licenseFile.path}`)} className="inline-flex items-center gap-2 text-muted-foreground hover:text-foreground">
              <Scale className="size-3.5" />
              License
            </Link>
          ) : null}
          {securityFile ? (
            <Link to={xgitPath(`${slug}/blob/${securityFile.path}`)} className="inline-flex items-center gap-2 text-muted-foreground hover:text-foreground">
              <Shield className="size-3.5" />
              Security policy
            </Link>
          ) : null}
        </div>
        <div className="mt-3 flex flex-wrap gap-2">
          <Badge variant="outline">{project.visibility}</Badge>
          <Badge variant="outline">{project.network}</Badge>
        </div>
        {cloneUrl ? <p className="mt-3 break-all font-mono text-[11px] text-muted-foreground">{cloneUrl}</p> : null}
      </div>
      <div>
        <p className="hud-label mb-2 text-muted-foreground/70">Contributors</p>
        <ul className="flex flex-col gap-1.5 text-sm">
          {(project.members ?? []).slice(0, 8).map((m) => (
            <li key={m.user_id} className="flex items-center justify-between gap-2">
              <span>{m.username}</span>
              <span className="text-xs text-muted-foreground">{m.role}</span>
            </li>
          ))}
        </ul>
        <p className="mt-2 text-xs text-muted-foreground">{project.member_count} contributors</p>
      </div>
      {languages.length > 0 ? (
        <div>
          <p className="hud-label mb-2 text-muted-foreground/70">Languages</p>
          <div className="mb-2 flex h-2 overflow-hidden rounded-full">
            {languages.map((l) => (
              <span
                key={l.name}
                className="h-full"
                style={{ width: `${Math.max(l.pct, 1)}%`, background: LANG_COLOR[l.name] ?? '#8b949e' }}
                title={`${l.name} ${l.pct.toFixed(1)}%`}
              />
            ))}
          </div>
          <ul className="flex flex-col gap-1 text-xs text-muted-foreground">
            {languages.map((l) => (
              <li key={l.name} className="flex items-center justify-between gap-2">
                <span className="inline-flex items-center gap-1.5">
                  <span className="size-2 rounded-full" style={{ background: LANG_COLOR[l.name] ?? '#8b949e' }} />
                  {l.name}
                </span>
                <span>{l.pct.toFixed(1)}%</span>
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </aside>
  )
}

export function XgitCommitsPage() {
  const { slug = '' } = useParams()
  const fetchBranches = useCallback(() => api.listProjectBranches(slug), [slug])
  const { data: branchData } = usePollingData(fetchBranches, 30_000)
  const branches = branchData?.items ?? []
  const [ref, setRef] = useState('')
  const activeRef = ref || (branches.includes('main') ? 'main' : branches[0] || 'HEAD')
  const fetchCommits = useCallback(() => api.listProjectCommits(slug, activeRef), [slug, activeRef])
  const { data, loading, error } = usePollingData(fetchCommits, 20_000)

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 className="font-display text-lg font-semibold">Commits</h2>
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
      </div>
      <div className="watch-complication overflow-hidden rounded-[18px]">
        {loading || !data ? (
          error ? <p className="p-5 text-sm text-destructive">{error}</p> : <p className="p-5 text-sm text-muted-foreground">Carregando…</p>
        ) : (data.items ?? []).length === 0 ? (
          <p className="p-5 text-sm text-muted-foreground">Nenhum commit nesta ref.</p>
        ) : (
          <ul className="divide-y divide-border/60">
            {data.items.map((c) => (
              <li key={c.sha} className="flex flex-wrap items-start justify-between gap-3 px-4 py-3">
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium">{c.subject}</p>
                  <p className="mt-0.5 text-xs text-muted-foreground">
                    {c.author} · {formatRelativeTime(c.date)}
                  </p>
                </div>
                <code className="font-mono text-xs text-muted-foreground">{c.sha.slice(0, 7)}</code>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}

export { XgitIssuesPage, XgitIssuePage, XgitPullsPage } from '@/pages/xgit-issues-page'
export { XgitMrsPage } from '@/pages/xgit-issues-page'

export { XgitActionsPage } from '@/pages/xgit-actions-page'

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
  return p ? xgitPath(`${slug}/tree/${p}`) : xgitPath(slug)
}

