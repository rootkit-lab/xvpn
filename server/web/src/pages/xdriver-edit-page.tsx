import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { toast } from 'sonner'
import { ArrowLeft, Save } from 'lucide-react'
import { api, ApiError, type DriverRoot } from '@/lib/api'
import { driverFileKind } from '@/lib/driver-kind'
import { Button } from '@/components/ui/button'
import { useDocumentTitleOverride } from '@/components/layout/document-title'

const MAX_EDIT = 2 * 1024 * 1024

function asRoot(v: string | null): DriverRoot {
  return v === 'home' ? 'home' : 'shared'
}

export function XDriverEditPage() {
  const [params] = useSearchParams()
  const root = asRoot(params.get('root'))
  const path = params.get('path') ?? ''
  const name = path.split('/').filter(Boolean).pop() ?? 'arquivo'
  const [text, setText] = useState('')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useDocumentTitleOverride(`Editar ${name}`)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)
    ;(async () => {
      try {
        if (driverFileKind(name, false) !== 'text') {
          throw new Error('Este arquivo não é editável no Drive.')
        }
        const blob = await api.fetchDriverBlob(root, path)
        if (blob.size > MAX_EDIT) {
          throw new Error('Arquivo maior que 2 MiB — baixe para editar fora.')
        }
        const next = await blob.text()
        if (!cancelled) setText(next)
      } catch (err) {
        if (!cancelled) setError(err instanceof ApiError ? err.message : err instanceof Error ? err.message : 'Falha ao abrir')
      } finally {
        if (!cancelled) setLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [name, path, root])

  async function save() {
    setSaving(true)
    try {
      await api.writeDriver(root, path, text)
      toast.success('Salvo')
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao salvar')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="mx-auto flex w-full max-w-4xl flex-col gap-4 px-4 py-6 md:px-8">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <Link to="/" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
          <ArrowLeft className="size-4" />
          Drive
        </Link>
        <Button size="sm" className="rounded-full" disabled={saving || loading || Boolean(error)} onClick={() => void save()}>
          <Save className="size-4" />
          {saving ? 'Salvando…' : 'Salvar'}
        </Button>
      </div>
      <div>
        <h1 className="font-display text-xl font-semibold tracking-tight">{name}</h1>
        <p className="mt-1 text-sm text-muted-foreground">Edição no navegador. Máximo 2 MiB.</p>
      </div>
      {error && <p className="text-sm text-destructive">{error}</p>}
      {loading ? (
        <div className="watch-complication h-80 animate-pulse rounded-[18px]" />
      ) : (
        !error && (
          <textarea
            className="field-glass min-h-[28rem] w-full resize-y rounded-[16px] p-4 font-mono text-sm leading-relaxed"
            value={text}
            onChange={(e) => setText(e.target.value)}
            spellCheck={false}
          />
        )
      )}
    </div>
  )
}
