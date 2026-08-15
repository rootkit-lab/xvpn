import { useCallback, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { Copy, Download, ExternalLink, Monitor, Package, Smartphone, Terminal, Users as UsersIcon } from 'lucide-react'
import {
  api,
  ApiError,
  type MarketplaceApp,
  type MarketplaceAsset,
  type MarketplaceChannel,
  type MarketplacePlatform,
  type MarketplaceVersion,
  type MarketplaceVisibility,
  type User,
} from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { formatBytes, formatDateTime } from '@/lib/format'
import { useAuth } from '@/lib/auth-context'
import { isAdminRole, ROLE_BADGE_VARIANT, ROLE_LABELS } from '@/lib/roles'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'

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

const VISIBILITY_LABELS: Record<MarketplaceVisibility, string> = {
  global: 'Global (todos)',
  restricted: 'Restrito (ACL)',
}

const CHANNEL_LABELS: Record<MarketplaceChannel, string> = {
  stable: 'Estável',
  beta: 'Beta',
}

const GITHUB_APPS_BASE = 'https://github.com/rootkit-lab/xvpn/tree/main/'

export function MarketplacePage({ variant = 'consume' }: { variant?: 'consume' | 'manage' }) {
  const { user: caller } = useAuth()
  const isManage = variant === 'manage' && isAdminRole(caller?.role)
  const fetchApps = useCallback(() => api.listMarketplaceApps(), [])
  const { data: apps, loading, error, reload } = usePollingData(fetchApps, 30_000)

  return (
    <div className="flex flex-col gap-6">
      <div>
        <p className="hud-label mb-1 text-muted-foreground/70">
          {isManage ? '// administração · marketplace' : '// meu espaço · apps'}
        </p>
        <h1 className="text-2xl font-semibold">{isManage ? 'Marketplace' : 'Apps'}</h1>
        <p className="text-muted-foreground">
          {isManage
            ? 'Catálogo espelhado de apps/*/marketplace.yaml — publicação só via CI. Aqui você controla ACL de apps restritos e baixa os arquivos.'
            : 'Programas liberados para a sua conta. Baixe o instalador da sua plataforma e confira o SHA-256 antes de instalar.'}
        </p>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {loading || !apps ? (
        <Skeleton className="h-32 w-full" />
      ) : apps.length === 0 ? (
        isManage ? (
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Catálogo ainda vazio</CardTitle>
              <CardDescription>
                O painel só espelha o que o CI publicou. Nada aparece aqui até existir uma release GitHub
                compatível com o manifesto e um sync bem-sucedido.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3 text-sm text-muted-foreground">
              <ol className="list-decimal space-y-2 pl-5">
                <li>
                  Release com tag <code className="font-mono text-xs">xvpn-client-v*</code> e assets
                  nomeados como no <code className="font-mono text-xs">apps/xvpn-client/marketplace.yaml</code>.
                </li>
                <li>
                  Workflow <code className="font-mono text-xs">marketplace-sync</code> no GitHub Actions.
                </li>
              </ol>
              <p>
                Fonte:{' '}
                <a
                  href={`${GITHUB_APPS_BASE}xvpn-client/marketplace.yaml`}
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex items-center gap-1 text-foreground underline-offset-4 hover:underline"
                >
                  apps/xvpn-client/marketplace.yaml
                  <ExternalLink className="size-3" />
                </a>
              </p>
            </CardContent>
          </Card>
        ) : (
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Nenhum app disponível</CardTitle>
              <CardDescription>
                Quando um administrador liberar programas para você, eles aparecem aqui para download.
              </CardDescription>
            </CardHeader>
          </Card>
        )
      ) : (
        <div className="flex flex-col gap-4">
          {apps.map((app) => (
            <AppCard key={app.id} app={app} isAdmin={isManage} onChanged={reload} />
          ))}
        </div>
      )}
    </div>
  )
}

function AppCard({ app, isAdmin, onChanged }: { app: MarketplaceApp; isAdmin: boolean; onChanged: () => void }) {
  return (
    <Card>
      <CardHeader className="flex flex-row flex-wrap items-start justify-between gap-4">
        <div className="flex min-w-0 items-start gap-3">
          {app.icon_url ? (
            <img src={app.icon_url} alt="" className="size-10 shrink-0 rounded-full object-cover" />
          ) : (
            <div className="flex size-10 shrink-0 items-center justify-center rounded-full bg-primary/15 text-primary shadow-[0_0_20px_-6px_var(--color-glow)]">
              <Package className="size-5" />
            </div>
          )}
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <CardTitle className="text-base">{app.name}</CardTitle>
              <Badge variant={app.visibility === 'global' ? 'outline' : 'secondary'}>
                {VISIBILITY_LABELS[app.visibility]}
              </Badge>
              {app.source_path && (
                <a
                  href={`${GITHUB_APPS_BASE}${app.source_path}`}
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex items-center gap-1 rounded-md border border-border px-2 py-0.5 font-mono text-xs text-muted-foreground hover:text-foreground"
                  title="Origem no repositório"
                >
                  {app.source_path}
                  <ExternalLink className="size-3" />
                </a>
              )}
            </div>
            {app.description && <CardDescription>{app.description}</CardDescription>}
          </div>
        </div>
        {isAdmin && <ManageAccessDialog app={app} onChanged={onChanged} />}
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        {app.versions.length === 0 && <p className="text-sm text-muted-foreground">Nenhuma versão sincronizada ainda.</p>}
        {app.versions.map((version) => (
          <VersionBlock key={version.id} version={version} />
        ))}
      </CardContent>
    </Card>
  )
}

function VersionBlock({ version }: { version: MarketplaceVersion }) {
  const channelLabel = CHANNEL_LABELS[version.channel as MarketplaceChannel] ?? version.channel

  return (
    <div className="rounded-lg border border-white/5 bg-background/40 p-4">
      <div className="flex flex-wrap items-center gap-2">
        <span className="font-medium">v{version.version}</span>
        <Badge variant="outline">{channelLabel}</Badge>
        <span className="text-xs text-muted-foreground">{formatDateTime(version.created_at)}</span>
      </div>
      {version.changelog && <p className="mt-2 text-sm text-muted-foreground">{version.changelog}</p>}
      <div className="mt-3 flex flex-col gap-2">
        {version.assets.length === 0 && <p className="text-xs text-muted-foreground">Nenhum arquivo nesta versão.</p>}
        {version.assets.map((asset) => (
          <AssetRow key={asset.id} asset={asset} />
        ))}
      </div>
    </div>
  )
}

function AssetRow({ asset }: { asset: MarketplaceAsset }) {
  const [downloading, setDownloading] = useState(false)
  const Icon = PLATFORM_ICONS[asset.platform]

  async function handleDownload() {
    setDownloading(true)
    try {
      await api.downloadMarketplaceAsset(asset.id, asset.filename)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao baixar arquivo')
    } finally {
      setDownloading(false)
    }
  }

  function copyHash() {
    navigator.clipboard.writeText(asset.sha256)
    toast.success('SHA-256 copiado')
  }

  return (
    <div className="flex flex-wrap items-center justify-between gap-2 rounded-md border border-white/5 bg-card/40 px-3 py-2">
      <div className="flex min-w-0 items-center gap-2">
        <Icon className="size-4 shrink-0 text-muted-foreground" />
        <div className="min-w-0">
          <p className="truncate text-sm font-medium" title={asset.filename}>
            {PLATFORM_LABELS[asset.platform]} · {asset.arch} — {asset.filename}
          </p>
          <div className="flex flex-wrap items-center gap-x-2 text-xs text-muted-foreground">
            <span>{formatBytes(asset.size_bytes)}</span>
            <span>·</span>
            <span>
              {asset.download_count} download{asset.download_count === 1 ? '' : 's'}
            </span>
            <span>·</span>
            <span className="inline-flex items-center gap-1">
              <span className="font-mono" title={asset.sha256}>
                sha256:{asset.sha256.slice(0, 12)}…
              </span>
              <Button type="button" variant="ghost" size="icon-xs" onClick={copyHash} title="Copiar hash completo">
                <Copy className="size-3" />
              </Button>
            </span>
          </div>
        </div>
      </div>
      <Button variant="ghost" size="icon-sm" onClick={handleDownload} disabled={downloading} title="Baixar">
        <Download className="size-4" />
      </Button>
    </div>
  )
}

function ManageAccessDialog({ app, onChanged }: { app: MarketplaceApp; onChanged: () => void }) {
  const [open, setOpen] = useState(false)
  const [users, setUsers] = useState<User[] | null>(null)
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const [error, setError] = useState<string | null>(null)
  const [loadingUsers, setLoadingUsers] = useState(false)
  const [submitting, setSubmitting] = useState(false)

  function handleOpenChange(next: boolean) {
    setOpen(next)
    if (next) {
      setError(null)
      setSelected(new Set(app.access_user_ids ?? []))
      setLoadingUsers(true)
      api
        .listUsers()
        .then(setUsers)
        .catch((err) => setError(err instanceof ApiError ? err.message : 'Falha ao carregar usuários'))
        .finally(() => setLoadingUsers(false))
    } else {
      setUsers(null)
    }
  }

  function toggle(id: number) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      await api.setMarketplaceAppAccess(app.id, Array.from(selected))
      toast.success(`Acesso de "${app.name}" atualizado`)
      setOpen(false)
      onChanged()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Falha ao atualizar acesso')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button variant="ghost" size="icon" title="Gerenciar acesso">
          <UsersIcon className="size-4" />
        </Button>
      </DialogTrigger>
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Acesso a "{app.name}"</DialogTitle>
            <DialogDescription>
              {app.visibility === 'global'
                ? 'Este app é global — todo mundo já enxerga e baixa. A lista abaixo só passa a valer se o manifesto trocar a visibilidade para restrita.'
                : 'Só os usuários marcados abaixo enxergam e baixam este app (além de admin/super_admin).'}
            </DialogDescription>
          </DialogHeader>
          <div className="flex max-h-72 flex-col gap-1 overflow-y-auto py-4">
            {loadingUsers || !users ? (
              <Skeleton className="h-32 w-full" />
            ) : (
              <>
                {users.map((u) => (
                  <div key={u.id} className="flex items-center gap-3 rounded-md px-2 py-1.5 hover:bg-accent">
                    <Checkbox
                      id={`access-user-${u.id}`}
                      checked={selected.has(u.id)}
                      onCheckedChange={() => toggle(u.id)}
                    />
                    <Label htmlFor={`access-user-${u.id}`} className="flex-1 cursor-pointer font-normal">
                      {u.username}
                    </Label>
                    <Badge variant={ROLE_BADGE_VARIANT[u.role]}>{ROLE_LABELS[u.role]}</Badge>
                  </div>
                ))}
                {users.length === 0 && <p className="text-sm text-muted-foreground">Nenhum usuário cadastrado.</p>}
              </>
            )}
          </div>
          {error && <p className="text-sm text-destructive">{error}</p>}
          <DialogFooter>
            <Button type="submit" disabled={submitting || loadingUsers}>
              {submitting ? 'Salvando…' : 'Salvar acesso'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
