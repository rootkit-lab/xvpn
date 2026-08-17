import { useCallback, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import { api, ApiError, type CiJobStatus } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'

const STATUS_LABEL: Record<CiJobStatus, string> = {
  pending: 'Na fila',
  running: 'Rodando',
  success: 'OK',
  failed: 'Falhou',
  canceled: 'Cancelado',
}

export function CiJobStatusBadge({ status }: { status: CiJobStatus }) {
  const variant = status === 'success' ? 'secondary' : status === 'failed' ? 'destructive' : 'outline'
  return <Badge variant={variant}>{STATUS_LABEL[status]}</Badge>
}

export function CiJobPage() {
  const { slug = '', n = '' } = useParams()
  const number = Number(n)
  const fetchJob = useCallback(() => api.getCiJob(slug, number), [slug, number])
  const fetchLog = useCallback(() => api.getCiJobLog(slug, number).catch(() => ''), [slug, number])
  const { data, loading, error, reload } = usePollingData(fetchJob, 8_000)
  const { data: log } = usePollingData(fetchLog, 8_000)
  const [busy, setBusy] = useState(false)

  if (!Number.isFinite(number) || number < 1) {
    return <p className="text-sm text-destructive">Job inválido.</p>
  }
  if (loading || !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-48 w-full" />
  }

  async function cancel() {
    setBusy(true)
    try {
      await api.cancelCiJob(slug, number)
      toast.success('Job cancelado')
      reload()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao cancelar')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <p className="text-sm text-muted-foreground">
        <Link to="/admin/projects" className="hover:underline">
          Projetos
        </Link>
        <span className="px-1.5">/</span>
        <Link to={`/admin/projects/${slug}`} className="hover:underline">
          {slug}
        </Link>
        <span className="px-1.5">/</span>
        <span className="text-foreground">#{data.number}</span>
      </p>

      <Card>
        <CardHeader>
          <div className="flex flex-wrap items-center gap-2">
            <CardTitle className="text-base">Job #{data.number}</CardTitle>
            <CiJobStatusBadge status={data.status} />
          </div>
          <CardDescription>
            {data.trigger} · <code className="font-mono text-xs">{data.ref}</code>
            {data.runner ? ` · ${data.runner}` : ' · aguardando runner'}
            {data.merge_request_number ? ` · MR !${data.merge_request_number}` : null}
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <p className="font-mono text-xs break-all text-muted-foreground">{data.sha}</p>
          {data.error ? <p className="text-sm text-destructive">{data.error}</p> : null}
          <div className="flex flex-wrap gap-2">
            {data.has_artifact ? (
              <Button type="button" variant="outline" onClick={() => void api.downloadCiArtifact(slug, number)}>
                Baixar artifact
              </Button>
            ) : null}
            {data.status === 'pending' || data.status === 'running' ? (
              <Button type="button" variant="outline" disabled={busy} onClick={() => void cancel()}>
                Cancelar
              </Button>
            ) : null}
          </div>
          <pre className="watch-complication max-h-96 overflow-auto rounded-[18px] p-4 font-mono text-xs leading-relaxed">
            {log || (data.status === 'pending' ? 'Aguardando um peer role=runner na malha…' : 'Sem log ainda.')}
          </pre>
        </CardContent>
      </Card>
    </div>
  )
}
