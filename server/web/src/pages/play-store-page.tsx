import { useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import { ArrowLeft, Download, Monitor, Package, Smartphone, Terminal } from 'lucide-react'
import {
  api,
  ApiError,
  type MarketplaceApp,
  type MarketplaceAsset,
  type MarketplacePlatform,
} from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { formatBytes } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { EmptyState } from '@/components/pagination'
import { StoreShell } from '@/components/layout/store-shell'

const PLATFORM_LABELS: Record<MarketplacePlatform, string> = {
  linux: 'Linux',
  windows: 'Windows',
  android: 'Android',
}

const PLATFORM_ICONS: Record<MarketplacePlatform, typeof Terminal> = {
  linux: Terminal,
  windows: Monitor,
  android: Smartphone,
}

type Filter = 'all' | MarketplacePlatform

function appPlatforms(app: MarketplaceApp): MarketplacePlatform[] {
  const set = new Set<MarketplacePlatform>()
  for (const version of app.versions) {
    for (const asset of version.assets) set.add(asset.platform)
  }
  return [...set]
}

function latestVersion(app: MarketplaceApp) {
  return app.versions[0]
}

export function PlayStoreSearch({ q, onQ }: { q: string; onQ: (v: string) => void }) {
  return (
    <Input
      value={q}
      onChange={(e) => onQ(e.target.value)}
      placeholder="Buscar apps e jogos"
      className="mx-auto max-w-xl"
      aria-label="Buscar no Marketplace"
    />
  )
}

export function PlayStoreLayout() {
  return <StoreShell kind="marketplace" />
}

export function PlayStoreHome() {
  const { data: apps, loading, error } = usePollingData(() => api.listMarketplaceApps(), 30_000)
  const [q, setQ] = useState('')
  const [filter, setFilter] = useState<Filter>('all')

  const filtered = useMemo(() => {
    const needle = q.trim().toLowerCase()
    return (apps ?? []).filter((app) => {
      if (needle && !app.name.toLowerCase().includes(needle) && !app.slug.toLowerCase().includes(needle)) {
        return false
      }
      if (filter === 'all') return true
      return appPlatforms(app).includes(filter)
    })
  }, [apps, q, filter])

  const featured = filtered[0]

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-8 px-4 py-6 md:px-8">
      <PlayStoreSearch q={q} onQ={setQ} />

      {error && <p className="text-sm text-destructive">{error}</p>}

      {loading || !apps ? (
        <Skeleton className="h-48 w-full rounded-[22px]" />
      ) : apps.length === 0 ? (
        <EmptyState title="Nenhum app publicado ainda." />
      ) : (
        <>
          {featured && <FeaturedHero app={featured} />}

          <div className="flex flex-wrap gap-2">
            {(['all', 'linux', 'windows', 'android'] as const).map((key) => (
              <button
                key={key}
                type="button"
                onClick={() => setFilter(key)}
                className={`rounded-full px-3 py-1.5 text-[12px] font-medium ${
                  filter === key ? 'power-safe' : 'watch-complication'
                }`}
              >
                {key === 'all' ? 'Todos' : PLATFORM_LABELS[key]}
              </button>
            ))}
          </div>

          {filtered.length === 0 ? (
            <EmptyState title="Nenhum item neste filtro." />
          ) : (
            <section>
              <h2 className="font-display mb-4 text-lg font-semibold">Recomendados para você</h2>
              <div className="grid grid-cols-3 gap-3 sm:grid-cols-4 md:grid-cols-5">
                {filtered.map((app) => (
                  <AppTile key={app.id} app={app} />
                ))}
              </div>
            </section>
          )}
        </>
      )}
    </div>
  )
}

function FeaturedHero({ app }: { app: MarketplaceApp }) {
  const version = latestVersion(app)
  return (
    <Link
      to={`/app/${app.slug}`}
      className="watch-complication-lift watch-complication relative flex flex-col gap-4 overflow-hidden rounded-[22px] p-6 md:flex-row md:items-center md:gap-8"
    >
      <AppIcon app={app} className="size-20 rounded-[22px] md:size-24" />
      <div className="min-w-0 flex-1">
        <p className="hud-label text-muted-foreground/70">Destaque</p>
        <h1 className="font-display mt-1 text-2xl font-semibold tracking-tight">{app.name}</h1>
        {app.description && <p className="mt-2 line-clamp-2 text-sm text-muted-foreground">{app.description}</p>}
        {version && (
          <p className="mt-2 text-xs text-muted-foreground">
            v{version.version}
            {version.channel !== 'stable' ? ` · ${version.channel}` : ''}
          </p>
        )}
      </div>
      <span className="btn-glow inline-flex items-center justify-center rounded-full px-5 py-2.5 text-sm font-medium">
        Ver detalhes
      </span>
    </Link>
  )
}

function AppTile({ app }: { app: MarketplaceApp }) {
  const version = latestVersion(app)
  return (
    <Link to={`/app/${app.slug}`} className="flex flex-col items-center gap-2 rounded-[18px] px-2 py-3 text-center hover:bg-white/6">
      <AppIcon app={app} className="size-16 rounded-[18px]" />
      <span className="font-display w-full truncate text-[13px] font-semibold">{app.name}</span>
      <span className="text-[11px] text-muted-foreground">{version ? `v${version.version}` : '—'}</span>
    </Link>
  )
}

function AppIcon({ app, className }: { app: MarketplaceApp; className?: string }) {
  if (app.icon_url) {
    return <img src={app.icon_url} alt="" className={`object-cover ${className ?? ''}`} />
  }
  return (
    <span className={`icon-well-lg flex items-center justify-center text-foreground ${className ?? ''}`}>
      <Package className="size-7" />
    </span>
  )
}

export function PlayStoreDetail() {
  const { slug } = useParams()
  const { data: apps, loading, error } = usePollingData(() => api.listMarketplaceApps(), 30_000)
  const app = apps?.find((item) => item.slug === slug)

  if (loading || !apps) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-8">
        <Skeleton className="h-40 w-full rounded-[22px]" />
      </div>
    )
  }

  if (error) {
    return <p className="px-4 py-8 text-sm text-destructive">{error}</p>
  }

  if (!app) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-8">
        <EmptyState title="App não encontrado." />
        <div className="mt-4 text-center">
          <Link to="/" className="text-sm underline underline-offset-4">
            Voltar à loja
          </Link>
        </div>
      </div>
    )
  }

  const version = latestVersion(app)

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 px-4 py-6 md:px-8">
      <Link to="/" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ArrowLeft className="size-4" />
        Loja
      </Link>

      <div className="flex flex-col gap-5 sm:flex-row sm:items-start">
        <AppIcon app={app} className="size-24 shrink-0 rounded-[22px]" />
        <div className="min-w-0 flex-1">
          <h1 className="font-display text-2xl font-semibold tracking-tight">{app.name}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{app.description || 'App da intranet ihuull.'}</p>
          <div className="mt-3 flex flex-wrap gap-2">
            {appPlatforms(app).map((p) => (
              <Badge key={p} variant="outline">
                {PLATFORM_LABELS[p]}
              </Badge>
            ))}
            {version && <Badge variant="secondary">v{version.version}</Badge>}
          </div>
        </div>
      </div>

      {version?.changelog && (
        <section className="watch-complication rounded-[18px] p-5">
          <h2 className="hud-label text-muted-foreground/70">Novidades</h2>
          <p className="mt-2 text-sm whitespace-pre-wrap">{version.changelog}</p>
        </section>
      )}

      <section className="flex flex-col gap-3">
        <h2 className="font-display text-lg font-semibold">Instalar</h2>
        {!version || version.assets.length === 0 ? (
          <p className="text-sm text-muted-foreground">Nenhum instalador nesta versão.</p>
        ) : (
          version.assets.map((asset) => <InstallRow key={asset.id} asset={asset} />)
        )}
      </section>
    </div>
  )
}

function InstallRow({ asset }: { asset: MarketplaceAsset }) {
  const [busy, setBusy] = useState(false)
  const Icon = PLATFORM_ICONS[asset.platform]

  async function install() {
    setBusy(true)
    try {
      await api.downloadMarketplaceAsset(asset.id, asset.filename)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao baixar')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="watch-complication flex flex-wrap items-center justify-between gap-3 rounded-[16px] px-4 py-3">
      <div className="flex min-w-0 items-center gap-3">
        <Icon className="size-5 shrink-0 text-muted-foreground" />
        <div className="min-w-0">
          <p className="truncate text-sm font-medium">
            {PLATFORM_LABELS[asset.platform]} · {asset.arch}
          </p>
          <p className="text-xs text-muted-foreground">
            {formatBytes(asset.size_bytes)} · sha256:{asset.sha256.slice(0, 12)}…
          </p>
        </div>
      </div>
      <Button type="button" size="lg" className="rounded-full" onClick={install} disabled={busy}>
        <Download className="size-4" />
        {busy ? 'Baixando…' : 'Instalar'}
      </Button>
    </div>
  )
}
