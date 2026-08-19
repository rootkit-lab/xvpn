import { Link, useSearchParams } from 'react-router-dom'
import { ArrowLeft, Download } from 'lucide-react'
import { api, ApiError, type DriverRoot, driverFileURL } from '@/lib/api'
import { driverFileKind } from '@/lib/driver-kind'
import { Button } from '@/components/ui/button'
import { useDocumentTitleOverride } from '@/components/layout/document-title'
import { toast } from 'sonner'

function asRoot(v: string | null): DriverRoot {
  return v === 'home' ? 'home' : 'shared'
}

export function XDriverViewPage() {
  const [params] = useSearchParams()
  const root = asRoot(params.get('root'))
  const path = params.get('path') ?? ''
  const name = path.split('/').filter(Boolean).pop() ?? 'arquivo'
  const kind = driverFileKind(name, false)
  const src = driverFileURL(root, path, true)

  useDocumentTitleOverride(name)

  async function download() {
    try {
      await api.downloadDriver(root, path, name)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha no download')
    }
  }

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-4 px-4 py-6 md:px-8">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <Link to="/" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
          <ArrowLeft className="size-4" />
          Drive
        </Link>
        <Button size="sm" variant="outline" className="rounded-full" onClick={() => void download()}>
          <Download className="size-4" />
          Baixar
        </Button>
      </div>
      <h1 className="font-display text-xl font-semibold tracking-tight">{name}</h1>

      <div className="watch-complication flex min-h-[20rem] items-center justify-center overflow-hidden rounded-[22px] p-4">
        {kind === 'image' && <img src={src} alt={name} className="max-h-[70svh] max-w-full rounded-[12px] object-contain" />}
        {kind === 'video' && (
          <video src={src} controls className="max-h-[70svh] w-full rounded-[12px]" preload="metadata">
            Seu navegador não reproduz este vídeo.
          </video>
        )}
        {kind === 'audio' && (
          <audio src={src} controls className="w-full">
            Seu navegador não reproduz este áudio.
          </audio>
        )}
        {kind === 'pdf' && <iframe title={name} src={src} className="h-[70svh] w-full rounded-[12px] bg-background" />}
        {kind !== 'image' && kind !== 'video' && kind !== 'audio' && kind !== 'pdf' && (
          <p className="text-sm text-muted-foreground">Este tipo não tem visualização. Use Baixar.</p>
        )}
      </div>
    </div>
  )
}
