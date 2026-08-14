import { useCallback, useEffect, useMemo, useState, type FormEvent } from 'react'
import { motion } from 'framer-motion'
import { ArrowLeft, CheckCircle2, Download, FolderOpen, Loader2, LogOut, Package } from 'lucide-react'

import type {
  MarketplaceApp,
  MarketplaceAsset,
  MarketplaceSession,
  StatusView,
} from '../../bindings/github.com/rootkit-lab/xvpn/client'
import { VPNService } from '../../bindings/github.com/rootkit-lab/xvpn/client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent } from '@/components/ui/card'
import { formatBytes } from '@/lib/format'

interface AppsPageProps {
  status: StatusView
  onBack: () => void
}

interface CatalogEntry {
  app: MarketplaceApp
  versionLabel: string
  changelog: string
  assets: MarketplaceAsset[]
}

type DownloadStatus = 'idle' | 'downloading' | 'done' | 'error'

// AppsPage é o catálogo do marketplace dentro do cliente desktop (Fase
// 12, ROADMAP.md) — separado da conta de dispositivo (enrollment): aqui
// o usuário autentica com usuário/senha do painel (JWT) só para
// listar/baixar programas de terceiros, sessão mantida em memória pelo
// processo Go (ver internal/marketplaceclient) e nunca persistida em
// disco. Fechar o app ou o token expirar (padrão 12h) volta a pedir
// login — não há "lembrar-me".
export function AppsPage({ status, onBack }: AppsPageProps) {
  const [session, setSession] = useState<MarketplaceSession | null>(null)
  const [platform, setPlatform] = useState<string | null>(null)
  const [apps, setApps] = useState<MarketplaceApp[] | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [downloadStatus, setDownloadStatus] = useState<Record<number, DownloadStatus>>({})
  const [downloadedPaths, setDownloadedPaths] = useState<Record<number, string>>({})

  const loadCatalog = useCallback(async () => {
    setError(null)
    try {
      const list = await VPNService.ListMarketplaceApps()
      setApps(list ?? [])
    } catch (err) {
      // Qualquer falha aqui (sessão expirada ou erro de rede) volta pra
      // tela de login — mais simples e robusto do que tentar distinguir
      // os dois casos pelo texto da mensagem (ver comentário em
      // vpnservice.go/ListMarketplaceApps).
      setApps(null)
      setSession({ loggedIn: false, username: '', role: '' })
      setError(err instanceof Error ? err.message : String(err))
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      setLoading(true)
      try {
        const [plat, sess] = await Promise.all([VPNService.Platform(), VPNService.MarketplaceSessionStatus()])
        if (cancelled) return
        setPlatform(plat)
        setSession(sess)
        if (sess.loggedIn) await loadCatalog()
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : String(err))
      } finally {
        if (!cancelled) setLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [loadCatalog])

  async function handleLogin(username: string, password: string) {
    setError(null)
    setLoading(true)
    try {
      const sess = await VPNService.MarketplaceLogin({
        serverBaseURL: status.serverBaseURL,
        username,
        password,
      })
      setSession(sess)
      await loadCatalog()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }

  function handleLogout() {
    VPNService.MarketplaceLogout()
    setSession({ loggedIn: false, username: '', role: '' })
    setApps(null)
    setDownloadStatus({})
    setDownloadedPaths({})
  }

  async function handleDownload(asset: MarketplaceAsset) {
    setError(null)
    setDownloadStatus((prev) => ({ ...prev, [asset.id]: 'downloading' }))
    try {
      const result = await VPNService.DownloadMarketplaceAsset({
        assetId: asset.id,
        filename: asset.filename,
        sha256: asset.sha256,
      })
      setDownloadStatus((prev) => ({ ...prev, [asset.id]: 'done' }))
      setDownloadedPaths((prev) => ({ ...prev, [asset.id]: result.path }))
    } catch (err) {
      setDownloadStatus((prev) => ({ ...prev, [asset.id]: 'error' }))
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  // Pra cada app, usa a versão mais recente (o servidor já devolve
  // ordenado desc) que tenha pelo menos um asset da plataforma deste
  // dispositivo — apps sem nenhum asset compatível (ex.: só Android)
  // somem da lista: o ROADMAP.md Fase 12 pede "lista por plataforma do
  // SO atual", não o catálogo inteiro sem filtro.
  const catalog = useMemo<CatalogEntry[]>(() => {
    if (!apps || !platform) return []
    const entries: CatalogEntry[] = []
    for (const app of apps) {
      const version = (app.versions ?? []).find((v) => (v.assets ?? []).some((a) => a.platform === platform))
      if (!version) continue
      entries.push({
        app,
        versionLabel: version.version,
        changelog: version.changelog,
        assets: (version.assets ?? []).filter((a) => a.platform === platform),
      })
    }
    return entries
  }, [apps, platform])

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: 0.2 }}
      className="flex h-full flex-col gap-4 p-6"
    >
      <header className="flex items-center gap-3">
        <button
          onClick={onBack}
          aria-label="Voltar"
          className="rounded-full p-1.5 text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
        >
          <ArrowLeft className="h-4 w-4" />
        </button>
        <h1 className="flex-1 text-lg font-semibold">Apps</h1>
        {loading && <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />}
        {session?.loggedIn && (
          <button
            onClick={handleLogout}
            aria-label="Sair do marketplace"
            title={`Sair (${session.username})`}
            className="rounded-full p-1.5 text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
          >
            <LogOut className="h-4 w-4" />
          </button>
        )}
      </header>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {!loading && session && !session.loggedIn && (
        <LoginForm serverBaseURL={status.serverBaseURL} onSubmit={handleLogin} />
      )}

      {session?.loggedIn && (
        <div className="flex flex-1 flex-col gap-3 overflow-y-auto">
          {catalog.length === 0 ? (
            <p className="mt-6 text-center text-sm text-muted-foreground">
              Nenhum programa disponível para {platform === 'windows' ? 'Windows' : 'Linux'} ainda.
            </p>
          ) : (
            catalog.map((entry) => (
              <AppEntryCard
                key={entry.app.id}
                entry={entry}
                downloadStatus={downloadStatus}
                downloadedPaths={downloadedPaths}
                onDownload={handleDownload}
              />
            ))
          )}
        </div>
      )}
    </motion.div>
  )
}

function LoginForm({
  serverBaseURL,
  onSubmit,
}: {
  serverBaseURL: string
  onSubmit: (username: string, password: string) => void
}) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')

  function submit(e: FormEvent) {
    e.preventDefault()
    onSubmit(username, password)
  }

  return (
    <Card className="border-white/5 bg-card/70">
      <CardContent className="flex flex-col gap-3 pt-6">
        <p className="text-xs text-muted-foreground">
          Entre com sua conta do painel ({serverBaseURL || 'servidor não configurado'}) para ver os programas
          liberados para você.
        </p>
        <form onSubmit={submit} className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="marketplace-username">Usuário</Label>
            <Input
              id="marketplace-username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="username"
              required
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="marketplace-password">Senha</Label>
            <Input
              id="marketplace-password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
              required
            />
          </div>
          <Button type="submit" className="mt-1 rounded-full">
            Entrar
          </Button>
        </form>
      </CardContent>
    </Card>
  )
}

function AppEntryCard({
  entry,
  downloadStatus,
  downloadedPaths,
  onDownload,
}: {
  entry: CatalogEntry
  downloadStatus: Record<number, DownloadStatus>
  downloadedPaths: Record<number, string>
  onDownload: (asset: MarketplaceAsset) => void
}) {
  return (
    <Card className="border-white/5 bg-card/70">
      <CardContent className="flex flex-col gap-3 p-4">
        <div className="flex items-start gap-3">
          {entry.app.iconURL ? (
            <img src={entry.app.iconURL} alt="" className="h-9 w-9 rounded-lg object-cover" />
          ) : (
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-secondary">
              <Package className="h-4.5 w-4.5 text-muted-foreground" />
            </div>
          )}
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium">{entry.app.name}</p>
            {entry.app.description && (
              <p className="line-clamp-2 text-xs text-muted-foreground">{entry.app.description}</p>
            )}
            <p className="mt-0.5 text-[11px] text-muted-foreground">v{entry.versionLabel}</p>
          </div>
        </div>

        <div className="flex flex-col gap-2">
          {entry.assets.map((asset) => (
            <AssetRow
              key={asset.id}
              asset={asset}
              status={downloadStatus[asset.id] ?? 'idle'}
              path={downloadedPaths[asset.id]}
              onDownload={() => onDownload(asset)}
            />
          ))}
        </div>
      </CardContent>
    </Card>
  )
}

function AssetRow({
  asset,
  status,
  path,
  onDownload,
}: {
  asset: MarketplaceAsset
  status: DownloadStatus
  path?: string
  onDownload: () => void
}) {
  return (
    <div className="flex flex-col gap-1.5 rounded-lg border border-border/60 bg-secondary/40 p-2.5">
      <div className="flex items-center justify-between gap-2">
        <div className="min-w-0">
          <p className="truncate text-xs font-medium">{asset.filename}</p>
          <p className="text-[10px] text-muted-foreground">
            {asset.arch} · {formatBytes(asset.sizeBytes)} ·{' '}
            <span title={`SHA-256: ${asset.sha256}`}>sha256 {asset.sha256.slice(0, 10)}…</span>
          </p>
        </div>
        {status === 'done' ? (
          <span className="flex items-center gap-1 text-[11px] text-primary">
            <CheckCircle2 className="h-3.5 w-3.5" />
            Baixado
          </span>
        ) : (
          <Button
            size="sm"
            variant="secondary"
            disabled={status === 'downloading'}
            onClick={onDownload}
            className="rounded-full"
          >
            {status === 'downloading' ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Download className="h-3.5 w-3.5" />}
            {status === 'downloading' ? 'Baixando…' : status === 'error' ? 'Tentar de novo' : 'Baixar'}
          </Button>
        )}
      </div>
      {status === 'done' && path && (
        <div className="flex gap-2">
          <Button size="sm" variant="outline" className="flex-1 rounded-full text-xs" onClick={() => VPNService.OpenLocalPath(path)}>
            Abrir arquivo
          </Button>
          <Button
            size="sm"
            variant="outline"
            className="flex-1 rounded-full text-xs"
            onClick={() => VPNService.OpenDownloadsFolder()}
          >
            <FolderOpen className="h-3.5 w-3.5" />
            Abrir pasta
          </Button>
        </div>
      )}
    </div>
  )
}
