import { useCallback, useMemo, useState } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { toast } from 'sonner'
import {
  Box,
  Clock,
  Globe,
  Package,
  Search,
  Server,
  Shield,
  Terminal,
  Wand2,
} from 'lucide-react'
import { api, ApiError, type WorkflowTemplate } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { xgitPath } from '@/lib/xgit'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

const ICONS: Record<string, typeof Server> = {
  go: Terminal,
  node: Box,
  python: Terminal,
  rust: Terminal,
  shell: Terminal,
  server: Server,
  box: Box,
  globe: Globe,
  shield: Shield,
  wand: Wand2,
  clock: Clock,
  package: Package,
}

export function XgitActionsNewPage() {
  const { org = '', slug = '' } = useParams()
  const navigate = useNavigate()
  const [search, setSearch] = useSearchParams()
  const category = search.get('category') || ''
  const [q, setQ] = useState(search.get('q') || '')
  const [busy, setBusy] = useState<string | null>(null)
  const fetchTemplates = useCallback(
    () => api.listWorkflowTemplates(category || undefined, q || undefined),
    [category, q],
  )
  const { data, loading, error } = usePollingData(fetchTemplates, 60_000)
  const categories = data?.categories ?? []
  const items = data?.items ?? []

  const heading = useMemo(() => {
    const row = categories.find((c) => c.id === category)
    return row ? row.label : 'Choose a workflow'
  }, [categories, category])

  async function apply(tpl: WorkflowTemplate) {
    setBusy(tpl.id)
    try {
      const res = await api.applyWorkflowTemplate(`${org}/${slug}`, tpl.id)
      toast.success(res.unchanged ? 'Workflow já estava aplicado' : `Workflow ${tpl.name} criado`)
      navigate(xgitPath(`${org}/${slug}/blob/main/.xvpn-ci.sh`))
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao aplicar o template')
    } finally {
      setBusy(null)
    }
  }

  if (loading && !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-48 w-full" />
  }

  return (
    <div className="grid gap-6 lg:grid-cols-[220px_1fr]">
      <aside className="flex flex-col gap-4 text-sm">
        <div className="relative">
          <Search className="pointer-events-none absolute left-2.5 top-2.5 size-3.5 text-muted-foreground" />
          <Input
            value={q}
            onChange={(e) => {
              const next = e.target.value
              setQ(next)
              const nextParams = new URLSearchParams(search)
              if (next) nextParams.set('q', next)
              else nextParams.delete('q')
              setSearch(nextParams, { replace: true })
            }}
            placeholder="Search workflows"
            className="h-9 pl-8"
            aria-label="Search workflows"
          />
        </div>
        <div>
          <p className="hud-label px-2 text-muted-foreground/70">Categories</p>
          <button
            type="button"
            className={cn(
              'mt-1 flex w-full rounded-md px-2 py-1.5 text-left',
              !category ? 'bg-muted/50 text-foreground' : 'text-muted-foreground hover:text-foreground',
            )}
            onClick={() => {
              const next = new URLSearchParams(search)
              next.delete('category')
              setSearch(next)
            }}
          >
            All
          </button>
          {categories.map((cat) => (
            <button
              key={cat.id}
              type="button"
              className={cn(
                'flex w-full rounded-md px-2 py-1.5 text-left',
                category === cat.id ? 'bg-muted/50 text-foreground' : 'text-muted-foreground hover:text-foreground',
              )}
              onClick={() => {
                const next = new URLSearchParams(search)
                next.set('category', cat.id)
                setSearch(next)
              }}
            >
              {cat.label}
            </button>
          ))}
        </div>
        <Link to={xgitPath(`${org}/${slug}/actions`)} className="px-2 text-xs text-muted-foreground hover:text-foreground">
          ← Actions
        </Link>
      </aside>

      <div className="min-w-0">
        <div className="mb-4">
          <p className="hud-label text-muted-foreground/70">Actions</p>
          <h2 className="font-display text-lg font-semibold">{heading}</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            Galeria no estilo GitHub Actions. Aplicar grava <code className="text-xs">.xvpn-ci.sh</code> — um job{' '}
            <code className="text-xs">ci</code>, sem YAML de múltiplos workflows.
          </p>
        </div>
        {error ? <p className="text-sm text-destructive">{error}</p> : null}
        {items.length === 0 ? (
          <p className="text-sm text-muted-foreground">Nenhum template nesta categoria.</p>
        ) : (
          <ul className="grid gap-3 sm:grid-cols-2">
            {items.map((tpl) => (
              <li key={tpl.id}>
                <WorkflowCard template={tpl} busy={busy === tpl.id} onConfigure={() => void apply(tpl)} />
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}

function WorkflowCard({
  template,
  busy,
  onConfigure,
}: {
  template: WorkflowTemplate
  busy: boolean
  onConfigure: () => void
}) {
  const Icon = ICONS[template.icon] ?? Terminal
  return (
    <article className="watch-complication group flex h-full flex-col gap-3 p-4">
      <div className="flex items-start gap-3">
        <span className="icon-well flex size-9 shrink-0 items-center justify-center">
          <Icon className="size-4" />
        </span>
        <div className="min-w-0">
          <h3 className="font-medium leading-tight">{template.name}</h3>
          <p className="mt-1 text-sm text-muted-foreground">{template.description}</p>
        </div>
      </div>
      <div className="mt-auto flex flex-wrap items-center justify-between gap-2">
        <p className="text-xs text-muted-foreground">{template.languages.join(', ')}</p>
        <Button type="button" size="sm" variant="outline" disabled={busy} onClick={onConfigure}>
          {busy ? 'A aplicar…' : 'Configure'}
        </Button>
      </div>
    </article>
  )
}
