import { useCallback, useState, type FormEvent } from 'react'
import { Link, useParams } from 'react-router-dom'
import { Box, Copy, Download, Gem, Package } from 'lucide-react'
import { toast } from 'sonner'
import { api, ApiError, type ForgePackage, type ForgePackageKind } from '@/lib/api'
import { formatBytes, formatRelativeTime } from '@/lib/format'
import { usePollingData } from '@/hooks/use-polling-data'
import { xgitPath } from '@/lib/xgit'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'

const GIT = 'https://xgit.corp.ihuull.com'

type RegistryCard = {
  id: string
  title: string
  blurb: string
  ready: boolean
  hint?: string
}

function registryCards(org: string, slug: string): RegistryCard[] {
  const repo = org && slug ? `${org}/${slug}` : '<org>/<slug>'
  const base = `${GIT}/api/packages/${repo}`
  return [
    {
      id: 'maven',
      title: 'Apache Maven',
      blurb: 'Registry Maven 2 (mvn deploy, SNAPSHOT). Auth = JWE.',
      ready: false,
      hint: `mvn deploy -DaltDeploymentRepository=xgit::default::${base}/maven`,
    },
    {
      id: 'nuget',
      title: 'NuGet',
      blurb: 'Feed NuGet na malha (dotnet nuget push). Auth = JWE.',
      ready: false,
      hint: `dotnet nuget push *.nupkg --source ${base}/nuget/index.json`,
    },
    {
      id: 'rubygems',
      title: 'RubyGems',
      blurb: 'gem push contra xgit.corp. Auth = JWE.',
      ready: false,
      hint: `gem push --host ${base}/rubygems`,
    },
    {
      id: 'npm',
      title: 'npm',
      blurb: 'Registry npm scoped (@ihuull). Auth = Bearer JWE.',
      ready: true,
      hint: `npm publish --registry ${base}/npm/`,
    },
    {
      id: 'pypi',
      title: 'PyPI',
      blurb: 'Simple API (PEP 503 / 691). Auth = Basic user + JWE.',
      ready: true,
      hint: `twine upload --repository-url ${base}/pypi -u <user> -p <JWE> dist/*`,
    },
    {
      id: 'generic',
      title: 'Generic',
      blurb: 'Tarball ou binário via multipart na aba. Sem hostname novo.',
      ready: true,
    },
    {
      id: 'containers',
      title: 'Containers',
      blurb: 'Imagens Docker não misturam neste host — Fase 60 (Harbor / registry:2 no wg0).',
      ready: false,
    },
  ]
}

export function XgitRepoPackagesPage() {
  return <XgitPackagesPage />
}

export function XgitPackagesPage() {
  const { org = '', slug = '' } = useParams()
  const fetchHome = useCallback(() => api.listXgitPackages(), [])
  const fetchRepo = useCallback(() => api.listProjectPackages(`${org}/${slug}`), [org, slug])
  const { data, loading, error, reload } = usePollingData(slug ? fetchRepo : fetchHome, 20_000)
  const items = data?.items ?? []
  const canPublish = slug ? Boolean(data && 'can_publish' in data && data.can_publish) : false

  if (loading && !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-48 w-full" />
  }

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h2 className="font-display mb-1 text-base font-semibold">
          {items.length === 0 && slug ? 'Get started with Packages' : 'Packages'}
        </h2>
        <p className="text-sm text-muted-foreground">
          Registry na malha em <code className="text-xs">xgit.corp</code>. O artefacto vive ao lado do
          código em <code className="text-xs">{slug ? `${org}/${slug}` : '<org>/<slug>'}</code>. Auth =
          JWE (o PAT). Sem npmjs / Maven Central / nuget.org.
        </p>
      </div>
      {items.length === 0 ? <RegistryGrid org={org} slug={slug} /> : null}
      {canPublish ? <PublishForm org={org} slug={slug} onPublished={() => void reload()} /> : null}
      {items.length === 0 ? (
        slug ? null : (
          <p className="text-sm text-muted-foreground">
            Nenhum package visível. Os exemplos <code className="text-xs">xcorp/hello-*</code> publicam no
            boot do servidor.
          </p>
        )
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

function RegistryGrid({ org, slug }: { org: string; slug: string }) {
  const cards = registryCards(org, slug)
  return (
    <div className="flex flex-col gap-3">
      <p className="hud-label text-muted-foreground/70">Choose a registry</p>
      <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {cards.map((card) => (
          <li key={card.id} className="watch-complication flex h-full flex-col gap-3 rounded-[18px] p-4">
            <div className="flex items-start gap-3">
              <span className="icon-well flex size-9 shrink-0 items-center justify-center">
                {card.id === 'containers' ? <Box className="size-4" /> : card.id === 'rubygems' ? <Gem className="size-4" /> : <Package className="size-4" />}
              </span>
              <div className="min-w-0">
                <h3 className="font-medium leading-tight">{card.title}</h3>
                <p className="mt-1 text-sm text-muted-foreground">{card.blurb}</p>
              </div>
            </div>
            <div className="mt-auto flex flex-wrap items-center gap-2">
              {card.ready ? (
                <span className="rounded-full bg-muted px-2 py-0.5 text-xs">pronto</span>
              ) : (
                <span className="rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground">
                  {card.id === 'containers' ? 'Fase 60' : 'Fase 59'}
                </span>
              )}
              {card.hint && slug ? (
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  onClick={() => {
                    void navigator.clipboard.writeText(card.hint ?? '')
                    toast.success('Comando copiado')
                  }}
                >
                  <Copy className="mr-1.5 size-3.5" />
                  Copiar
                </Button>
              ) : null}
            </div>
          </li>
        ))}
      </ul>
      {slug ? (
        <p className="text-xs text-muted-foreground">
          Publish com Actions:{' '}
          <Link to={xgitPath(`${org}/${slug}/actions/new?category=publish`)} className="text-primary hover:underline">
            New workflow → Publish a package
          </Link>
          . O script não grava o JWE.
        </p>
      ) : null}
    </div>
  )
}

function PublishForm({ org, slug, onPublished }: { org: string; slug: string; onPublished: () => void }) {
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
      await api.uploadProjectPackage(`${org}/${slug}`, { name, version, kind, file })
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

  const npmRegistry = `${GIT}/api/packages/${org}/${slug}/npm/`
  const pypiSimple = `${GIT}/api/packages/${org}/${slug}/pypi/simple/`
  const npmHint = `npm publish --registry ${npmRegistry}`
  const pipHint = `pip install <pkg> --index-url https://<user>:<JWE>@xgit.corp.ihuull.com/api/packages/${org}/${slug}/pypi/simple/`
  const twineHint = `twine upload --repository-url ${GIT}/api/packages/${org}/${slug}/pypi -u <user> -p <JWE> dist/*`
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
