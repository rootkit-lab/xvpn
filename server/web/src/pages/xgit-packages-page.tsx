import { useCallback, useState, type FormEvent } from 'react'
import { Link, useParams } from 'react-router-dom'
import { Copy, Download, Package } from 'lucide-react'
import { toast } from 'sonner'
import { api, ApiError, type ForgePackage, type ForgePackageKind } from '@/lib/api'
import { formatBytes, formatRelativeTime } from '@/lib/format'
import { usePollingData } from '@/hooks/use-polling-data'
import { xgitPath } from '@/lib/xgit'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'

export function XgitRepoPackagesPage() {
  return <XgitPackagesPage />
}

export function XgitPackagesPage() {
  const { slug = '' } = useParams()
  const fetchHome = useCallback(() => api.listXgitPackages(), [])
  const fetchRepo = useCallback(() => api.listProjectPackages(slug), [slug])
  const { data, loading, error, reload } = usePollingData(slug ? fetchRepo : fetchHome, 20_000)
  const items = data?.items ?? []
  const canPublish = slug ? Boolean(data && 'can_publish' in data && data.can_publish) : false

  if (loading && !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-48 w-full" />
  }

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h2 className="font-display mb-1 text-base font-semibold">Packages</h2>
        <p className="text-sm text-muted-foreground">
          Registry na malha em <code className="text-xs">xgit.corp</code> — npm, PyPI ou arquivo genérico. Sem
          hostname novo. Container registry entra depois.
        </p>
      </div>
      {canPublish ? <PublishForm slug={slug} onPublished={() => void reload()} /> : null}
      {items.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          {slug ? 'Nenhum package neste repositório.' : 'Nenhum package visível.'}
        </p>
      ) : (
        <ul className="flex flex-col gap-3">
          {items.map((pkg) => (
            <PackageCard key={`${pkg.project_slug}:${pkg.kind}:${pkg.name}`} pkg={pkg} showProject={!slug} />
          ))}
        </ul>
      )}
    </div>
  )
}

function PublishForm({ slug, onPublished }: { slug: string; onPublished: () => void }) {
  const [name, setName] = useState('')
  const [version, setVersion] = useState('0.1.0')
  const [kind, setKind] = useState<ForgePackageKind>('generic')
  const [file, setFile] = useState<File | null>(null)
  const [busy, setBusy] = useState(false)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    if (!file) {
      toast.error('Escolha um arquivo')
      return
    }
    setBusy(true)
    try {
      await api.uploadProjectPackage(slug, { name, version, kind, file })
      toast.success('Package publicado')
      setName('')
      setFile(null)
      onPublished()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao publicar')
    } finally {
      setBusy(false)
    }
  }

  const npmRegistry = `https://xgit.corp.ihuull.com/api/packages/${slug}/npm/`
  const pypiSimple = `https://xgit.corp.ihuull.com/api/packages/${slug}/pypi/simple/`
  const npmHint = `npm publish --registry ${npmRegistry}`
  const pipHint = `pip install <pkg> --index-url https://<user>:<JWE>@xgit.corp.ihuull.com/api/packages/${slug}/pypi/simple/`
  const twineHint = `twine upload --repository-url https://xgit.corp.ihuull.com/api/packages/${slug}/pypi -u <user> -p <JWE> dist/*`
  const cliHint = kind === 'pypi' ? pipHint : npmHint
  const copyText = kind === 'pypi' ? `${twineHint}\n${pipHint}` : `${npmHint}`

  return (
    <form onSubmit={onSubmit} className="watch-complication flex flex-col gap-3 rounded-[18px] p-4">
      <p className="text-sm font-medium">Publicar</p>
      <div className="grid gap-2 sm:grid-cols-3">
        <Input required value={name} onChange={(e) => setName(e.target.value)} placeholder="nome" />
        <Input required value={version} onChange={(e) => setVersion(e.target.value)} placeholder="1.0.0" />
        <Select value={kind} onValueChange={(v) => setKind(v as ForgePackageKind)}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="generic">generic</SelectItem>
            <SelectItem value="npm">npm</SelectItem>
            <SelectItem value="pypi">pypi</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <Input type="file" onChange={(e) => setFile(e.target.files?.[0] ?? null)} />
      <div className="flex flex-wrap items-center gap-2">
        <Button type="submit" size="sm" disabled={busy}>
          Enviar arquivo
        </Button>
        <Button
          type="button"
          size="sm"
          variant="outline"
          onClick={() => {
            void navigator.clipboard.writeText(copyText)
            toast.success(kind === 'pypi' ? 'Comando pip/twine copiado' : 'Comando npm copiado')
          }}
        >
          <Copy className="mr-1.5 size-3.5" />
          {kind === 'pypi' ? 'pip / twine' : 'npm publish'}
        </Button>
      </div>
      <p className="text-xs text-muted-foreground">
        CLI: <code className="break-all">{cliHint}</code>
        {kind === 'pypi' ? (
          <>
            {' '}
            índice <code className="break-all">{pypiSimple}</code>
          </>
        ) : null}
      </p>
    </form>
  )
}

function PackageCard({ pkg, showProject }: { pkg: ForgePackage; showProject: boolean }) {
  return (
    <li className="watch-complication rounded-[18px] p-4">
      <div className="flex flex-wrap items-center gap-2">
        <Package className="size-4 text-muted-foreground" />
        <span className="font-medium">{pkg.name}</span>
        <span className="rounded-full bg-muted px-2 py-0.5 text-xs">{pkg.kind}</span>
        {pkg.latest ? <span className="text-xs text-muted-foreground">{pkg.latest}</span> : null}
        {showProject ? (
          <Link to={xgitPath(`${pkg.project_slug}/packages`)} className="text-sm text-primary hover:underline">
            {pkg.project_slug}
          </Link>
        ) : null}
      </div>
      {pkg.registry_url ? (
        <p className="mt-1 text-xs text-muted-foreground">
          registry <code>{pkg.registry_url}</code>
        </p>
      ) : null}
      <ul className="mt-3 flex flex-col gap-1.5">
        {pkg.versions.map((ver) => (
          <li key={ver.id} className="flex flex-wrap items-center justify-between gap-2 text-sm">
            <span>
              <span className="font-mono text-xs">{ver.version}</span>
              <span className="ml-2 text-muted-foreground">{ver.filename}</span>
              <span className="ml-2 text-xs text-muted-foreground">{formatBytes(ver.size)}</span>
              {ver.published_by ? (
                <span className="ml-2 text-xs text-muted-foreground">{ver.published_by}</span>
              ) : null}
              <span className="ml-2 text-xs text-muted-foreground">{formatRelativeTime(ver.created_at)}</span>
            </span>
            <Button
              type="button"
              size="sm"
              variant="ghost"
              onClick={() => {
                void api.downloadProjectPackage(pkg.project_slug, ver.id, ver.filename).catch((err) => {
                  toast.error(err instanceof ApiError ? err.message : 'Falha no download')
                })
              }}
            >
              <Download className="size-3.5" />
            </Button>
          </li>
        ))}
      </ul>
    </li>
  )
}
