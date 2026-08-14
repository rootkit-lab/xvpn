import { useCallback, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import {
  Copy,
  Download,
  Monitor,
  Package,
  Pencil,
  Plus,
  Smartphone,
  Terminal,
  Trash2,
  UploadCloud,
  Users as UsersIcon,
} from 'lucide-react'
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
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
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

// Espelha store.Platform (server/internal/store/models.go) — ícone e rótulo
// de cada plataforma de asset suportada (ver PLAN.md §6.8).
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

export function MarketplacePage() {
  const { user: caller } = useAuth()
  const isAdmin = isAdminRole(caller?.role)
  const fetchApps = useCallback(() => api.listMarketplaceApps(), [])
  const { data: apps, loading, error, reload } = usePollingData(fetchApps, 30_000)

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold">Marketplace</h1>
          <p className="text-muted-foreground">
            Catálogo interno de programas para Linux, Windows e Android — separado do cliente XVPN (ver Downloads).
          </p>
        </div>
        {isAdmin && <CreateAppDialog onCreated={reload} />}
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {loading || !apps ? (
        <Skeleton className="h-32 w-full" />
      ) : apps.length === 0 ? (
        <Card>
          <CardContent className="py-10 text-center text-muted-foreground">
            {isAdmin
              ? 'Nenhum app publicado ainda. Use "Novo app" para começar.'
              : 'Nenhum app disponível para o seu usuário no momento.'}
          </CardContent>
        </Card>
      ) : (
        <div className="flex flex-col gap-4">
          {apps.map((app) => (
            <AppCard key={app.id} app={app} isAdmin={isAdmin} onChanged={reload} />
          ))}
        </div>
      )}
    </div>
  )
}

function AppCard({ app, isAdmin, onChanged }: { app: MarketplaceApp; isAdmin: boolean; onChanged: () => void }) {
  const [deleting, setDeleting] = useState(false)

  async function handleDelete() {
    setDeleting(true)
    try {
      await api.deleteMarketplaceApp(app.id)
      toast.success(`App "${app.name}" removido`)
      onChanged()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao remover app')
    } finally {
      setDeleting(false)
    }
  }

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
            </div>
            {app.description && <CardDescription>{app.description}</CardDescription>}
          </div>
        </div>
        {isAdmin && (
          <div className="flex shrink-0 gap-1">
            <ManageAccessDialog app={app} onChanged={onChanged} />
            <CreateVersionDialog appId={app.id} onCreated={onChanged} />
            <EditAppDialog app={app} onChanged={onChanged} />
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button variant="ghost" size="icon" disabled={deleting} title="Remover app">
                  <Trash2 className="size-4 text-destructive" />
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>Remover app "{app.name}"?</AlertDialogTitle>
                  <AlertDialogDescription>
                    Remove todas as versões, arquivos enviados e a lista de acesso deste app. Essa ação não pode ser
                    desfeita.
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>Cancelar</AlertDialogCancel>
                  <AlertDialogAction onClick={handleDelete}>Remover</AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </div>
        )}
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        {app.versions.length === 0 && <p className="text-sm text-muted-foreground">Nenhuma versão publicada ainda.</p>}
        {app.versions.map((version) => (
          <VersionBlock key={version.id} version={version} isAdmin={isAdmin} onChanged={onChanged} />
        ))}
      </CardContent>
    </Card>
  )
}

function VersionBlock({
  version,
  isAdmin,
  onChanged,
}: {
  version: MarketplaceVersion
  isAdmin: boolean
  onChanged: () => void
}) {
  const [deleting, setDeleting] = useState(false)
  const channelLabel = CHANNEL_LABELS[version.channel as MarketplaceChannel] ?? version.channel

  async function handleDelete() {
    setDeleting(true)
    try {
      await api.deleteMarketplaceVersion(version.id)
      toast.success(`Versão ${version.version} removida`)
      onChanged()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao remover versão')
    } finally {
      setDeleting(false)
    }
  }

  return (
    <div className="rounded-lg border border-white/5 bg-background/40 p-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <span className="font-medium">v{version.version}</span>
          <Badge variant="outline">{channelLabel}</Badge>
          <span className="text-xs text-muted-foreground">{formatDateTime(version.created_at)}</span>
        </div>
        {isAdmin && (
          <div className="flex gap-1">
            <UploadAssetDialog versionId={version.id} onUploaded={onChanged} />
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button variant="ghost" size="icon-sm" disabled={deleting} title="Remover versão">
                  <Trash2 className="size-4 text-destructive" />
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>Remover versão {version.version}?</AlertDialogTitle>
                  <AlertDialogDescription>
                    Remove os arquivos enviados para esta versão. Essa ação não pode ser desfeita.
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>Cancelar</AlertDialogCancel>
                  <AlertDialogAction onClick={handleDelete}>Remover</AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </div>
        )}
      </div>
      {version.changelog && <p className="mt-2 text-sm text-muted-foreground">{version.changelog}</p>}
      <div className="mt-3 flex flex-col gap-2">
        {version.assets.length === 0 && <p className="text-xs text-muted-foreground">Nenhum arquivo enviado ainda.</p>}
        {version.assets.map((asset) => (
          <AssetRow key={asset.id} asset={asset} isAdmin={isAdmin} onChanged={onChanged} />
        ))}
      </div>
    </div>
  )
}

// AssetRow sempre mostra o botão de baixar: se o asset apareceu na resposta
// de GET /marketplace/apps é porque o app é global, o papel é admin+, ou o
// servidor já confirmou AppAccess pra este usuário — as mesmas três
// condições checadas de novo em handleDownloadMarketplaceAsset (ver
// marketplace_handler.go), então nunca há um asset visível aqui que o
// download vá rejeitar com 403.
function AssetRow({ asset, isAdmin, onChanged }: { asset: MarketplaceAsset; isAdmin: boolean; onChanged: () => void }) {
  const [downloading, setDownloading] = useState(false)
  const [deleting, setDeleting] = useState(false)
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

  async function handleDelete() {
    setDeleting(true)
    try {
      await api.deleteMarketplaceAsset(asset.id)
      toast.success(`Arquivo "${asset.filename}" removido`)
      onChanged()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao remover arquivo')
    } finally {
      setDeleting(false)
    }
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
      <div className="flex shrink-0 gap-1">
        <Button variant="ghost" size="icon-sm" onClick={handleDownload} disabled={downloading} title="Baixar">
          <Download className="size-4" />
        </Button>
        {isAdmin && (
          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button variant="ghost" size="icon-sm" disabled={deleting} title="Remover arquivo">
                <Trash2 className="size-4 text-destructive" />
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>Remover arquivo "{asset.filename}"?</AlertDialogTitle>
                <AlertDialogDescription>Essa ação não pode ser desfeita.</AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>Cancelar</AlertDialogCancel>
                <AlertDialogAction onClick={handleDelete}>Remover</AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        )}
      </div>
    </div>
  )
}

function VisibilitySelect({
  value,
  onChange,
}: {
  value: MarketplaceVisibility
  onChange: (v: MarketplaceVisibility) => void
}) {
  return (
    <Select value={value} onValueChange={(v) => onChange(v as MarketplaceVisibility)}>
      <SelectTrigger className="w-full">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {(Object.keys(VISIBILITY_LABELS) as MarketplaceVisibility[]).map((v) => (
          <SelectItem key={v} value={v}>
            {VISIBILITY_LABELS[v]}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

function PlatformSelect({ value, onChange }: { value: MarketplacePlatform; onChange: (p: MarketplacePlatform) => void }) {
  return (
    <Select value={value} onValueChange={(v) => onChange(v as MarketplacePlatform)}>
      <SelectTrigger className="w-full">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {(Object.keys(PLATFORM_LABELS) as MarketplacePlatform[]).map((p) => (
          <SelectItem key={p} value={p}>
            {PLATFORM_LABELS[p]}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

function ChannelSelect({ value, onChange }: { value: MarketplaceChannel; onChange: (c: MarketplaceChannel) => void }) {
  return (
    <Select value={value} onValueChange={(v) => onChange(v as MarketplaceChannel)}>
      <SelectTrigger className="w-full">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {(Object.keys(CHANNEL_LABELS) as MarketplaceChannel[]).map((c) => (
          <SelectItem key={c} value={c}>
            {CHANNEL_LABELS[c]}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

function CreateAppDialog({ onCreated }: { onCreated: () => void }) {
  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [iconUrl, setIconUrl] = useState('')
  const [visibility, setVisibility] = useState<MarketplaceVisibility>('global')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  function handleOpenChange(next: boolean) {
    setOpen(next)
    if (next) {
      setName('')
      setDescription('')
      setIconUrl('')
      setVisibility('global')
      setError(null)
    }
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      await api.createMarketplaceApp({ name, description, icon_url: iconUrl, visibility })
      toast.success(`App "${name}" criado`)
      setOpen(false)
      onCreated()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Falha ao criar app')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button>
          <Plus className="size-4" />
          Novo app
        </Button>
      </DialogTrigger>
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Novo app</DialogTitle>
            <DialogDescription>
              Cria a entrada no catálogo. Depois, publique uma versão e envie os arquivos de instalação.
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-4 py-4">
            <div className="flex flex-col gap-2">
              <Label htmlFor="new-app-name">Nome</Label>
              <Input id="new-app-name" required value={name} onChange={(e) => setName(e.target.value)} />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="new-app-description">Descrição</Label>
              <Textarea
                id="new-app-description"
                rows={2}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="new-app-icon">URL do ícone (opcional)</Label>
              <Input
                id="new-app-icon"
                type="url"
                placeholder="https://…"
                value={iconUrl}
                onChange={(e) => setIconUrl(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label>Visibilidade</Label>
              <VisibilitySelect value={visibility} onChange={setVisibility} />
            </div>
            {error && <p className="text-sm text-destructive">{error}</p>}
          </div>
          <DialogFooter>
            <Button type="submit" disabled={submitting}>
              {submitting ? 'Criando…' : 'Criar app'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function EditAppDialog({ app, onChanged }: { app: MarketplaceApp; onChanged: () => void }) {
  const [open, setOpen] = useState(false)
  const [name, setName] = useState(app.name)
  const [description, setDescription] = useState(app.description)
  const [iconUrl, setIconUrl] = useState(app.icon_url ?? '')
  const [visibility, setVisibility] = useState<MarketplaceVisibility>(app.visibility)
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  function handleOpenChange(next: boolean) {
    setOpen(next)
    if (next) {
      setName(app.name)
      setDescription(app.description)
      setIconUrl(app.icon_url ?? '')
      setVisibility(app.visibility)
      setError(null)
    }
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      const changes: {
        name?: string
        description?: string
        icon_url?: string
        visibility?: MarketplaceVisibility
      } = {}
      if (name !== app.name) changes.name = name
      if (description !== app.description) changes.description = description
      if (iconUrl !== (app.icon_url ?? '')) changes.icon_url = iconUrl
      if (visibility !== app.visibility) changes.visibility = visibility
      await api.updateMarketplaceApp(app.id, changes)
      toast.success(`App "${name}" atualizado`)
      setOpen(false)
      onChanged()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Falha ao atualizar app')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button variant="ghost" size="icon" title="Editar app">
          <Pencil className="size-4" />
        </Button>
      </DialogTrigger>
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Editar "{app.name}"</DialogTitle>
            <DialogDescription>Altera metadados do app — não mexe em versões ou arquivos.</DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-4 py-4">
            <div className="flex flex-col gap-2">
              <Label htmlFor={`edit-app-name-${app.id}`}>Nome</Label>
              <Input id={`edit-app-name-${app.id}`} required value={name} onChange={(e) => setName(e.target.value)} />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor={`edit-app-description-${app.id}`}>Descrição</Label>
              <Textarea
                id={`edit-app-description-${app.id}`}
                rows={2}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor={`edit-app-icon-${app.id}`}>URL do ícone (opcional)</Label>
              <Input
                id={`edit-app-icon-${app.id}`}
                type="url"
                placeholder="https://…"
                value={iconUrl}
                onChange={(e) => setIconUrl(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label>Visibilidade</Label>
              <VisibilitySelect value={visibility} onChange={setVisibility} />
            </div>
            {error && <p className="text-sm text-destructive">{error}</p>}
          </div>
          <DialogFooter>
            <Button type="submit" disabled={submitting}>
              {submitting ? 'Salvando…' : 'Salvar'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// ManageAccessDialog busca a lista de usuários toda vez que abre (em vez de
// cachear) — a tela só existe pra admin, então o custo é uma chamada leve a
// mais por abertura, e evita mostrar a ACL de outro app defasada se um
// usuário foi criado/removido enquanto o diálogo estava fechado.
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
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
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
                ? 'Este app é global — todo mundo já enxerga e baixa. A lista abaixo só passa a valer se você trocar a visibilidade para restrita.'
                : 'Só os usuários marcados abaixo enxergam e baixam este app (além de admin/super_admin, que sempre têm acesso).'}
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

function CreateVersionDialog({ appId, onCreated }: { appId: number; onCreated: () => void }) {
  const [open, setOpen] = useState(false)
  const [version, setVersion] = useState('')
  const [channel, setChannel] = useState<MarketplaceChannel>('stable')
  const [changelog, setChangelog] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  function handleOpenChange(next: boolean) {
    setOpen(next)
    if (next) {
      setVersion('')
      setChannel('stable')
      setChangelog('')
      setError(null)
    }
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      await api.createMarketplaceVersion(appId, { version, channel, changelog })
      toast.success(`Versão ${version} publicada`)
      setOpen(false)
      onCreated()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Falha ao publicar versão')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button variant="ghost" size="icon" title="Nova versão">
          <Plus className="size-4" />
        </Button>
      </DialogTrigger>
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Nova versão</DialogTitle>
            <DialogDescription>Depois de publicar, envie os arquivos de cada plataforma.</DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-4 py-4">
            <div className="flex flex-col gap-2">
              <Label htmlFor="new-version-number">Versão</Label>
              <Input
                id="new-version-number"
                required
                placeholder="1.0.0"
                value={version}
                onChange={(e) => setVersion(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label>Canal</Label>
              <ChannelSelect value={channel} onChange={setChannel} />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="new-version-changelog">Changelog (opcional)</Label>
              <Textarea
                id="new-version-changelog"
                rows={3}
                value={changelog}
                onChange={(e) => setChangelog(e.target.value)}
              />
            </div>
            {error && <p className="text-sm text-destructive">{error}</p>}
          </div>
          <DialogFooter>
            <Button type="submit" disabled={submitting}>
              {submitting ? 'Publicando…' : 'Publicar versão'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function UploadAssetDialog({ versionId, onUploaded }: { versionId: number; onUploaded: () => void }) {
  const [open, setOpen] = useState(false)
  const [platform, setPlatform] = useState<MarketplacePlatform>('linux')
  const [arch, setArch] = useState('')
  const [file, setFile] = useState<File | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  function handleOpenChange(next: boolean) {
    setOpen(next)
    if (next) {
      setPlatform('linux')
      setArch('')
      setFile(null)
      setError(null)
    }
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    if (!file) {
      setError('Selecione um arquivo')
      return
    }
    setError(null)
    setSubmitting(true)
    try {
      await api.uploadMarketplaceAsset(versionId, file, platform, arch)
      toast.success(`Arquivo "${file.name}" enviado`)
      setOpen(false)
      onUploaded()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Falha ao enviar arquivo')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button variant="ghost" size="icon-sm" title="Enviar arquivo">
          <UploadCloud className="size-4" />
        </Button>
      </DialogTrigger>
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Enviar arquivo</DialogTitle>
            <DialogDescription>
              O SHA-256 e o tamanho são calculados pelo servidor a partir do arquivo enviado, nunca informados pelo
              navegador.
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-4 py-4">
            <div className="flex flex-col gap-2">
              <Label>Plataforma</Label>
              <PlatformSelect value={platform} onChange={setPlatform} />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="upload-arch">Arquitetura (opcional)</Label>
              <Input id="upload-arch" placeholder="amd64" value={arch} onChange={(e) => setArch(e.target.value)} />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="upload-file">Arquivo</Label>
              <Input id="upload-file" type="file" required onChange={(e) => setFile(e.target.files?.[0] ?? null)} />
            </div>
            {error && <p className="text-sm text-destructive">{error}</p>}
          </div>
          <DialogFooter>
            <Button type="submit" disabled={submitting}>
              {submitting ? 'Enviando…' : 'Enviar'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
