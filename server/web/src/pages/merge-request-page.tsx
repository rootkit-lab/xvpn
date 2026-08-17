import { useCallback, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import { openChat } from '@chat/messenger/open-chat'
import { api, ApiError, type MergeRequestStatus } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { XGROUP_CORP_ORIGIN } from '@/lib/product-host'
import { xgitPath, xgitReposPath } from '@/lib/xgit'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'

const STATUS_LABEL: Record<MergeRequestStatus, string> = {
  open: 'Aberto',
  merged: 'Mergeado',
  closed: 'Fechado',
}

export function MergeRequestPage() {
  const { slug = '', iid = '' } = useParams()
  const number = Number(iid)
  const fetchMR = useCallback(() => api.getMergeRequest(slug, number), [slug, number])
  const { data, loading, error, reload } = usePollingData(fetchMR, 15_000)
  const [busy, setBusy] = useState(false)

  if (!Number.isFinite(number) || number < 1) {
    return <p className="text-sm text-destructive">MR inválido.</p>
  }
  if (loading || !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-48 w-full" />
  }

  async function act(kind: 'merge' | 'close') {
    setBusy(true)
    try {
      if (kind === 'merge') await api.mergeMergeRequest(slug, number)
      else await api.closeMergeRequest(slug, number)
      toast.success(kind === 'merge' ? 'Merge concluído' : 'MR fechado')
      reload()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha na ação')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <p className="text-sm text-muted-foreground">
        <Link to={xgitReposPath()} className="hover:underline">
          XGIT
        </Link>
        <span className="px-1.5">/</span>
        <Link to={xgitPath(slug)} className="hover:underline">
          {slug}
        </Link>
        <span className="px-1.5">/</span>
        <span className="text-foreground">!{data.number}</span>
      </p>

      <Card>
        <CardHeader>
          <div className="flex flex-wrap items-center gap-2">
            <CardTitle className="text-base">
              !{data.number} {data.title}
            </CardTitle>
            <StatusBadge status={data.status} />
          </div>
          <CardDescription>
            <code className="font-mono text-xs">{data.source_branch}</code>
            {' → '}
            <code className="font-mono text-xs">{data.target_branch}</code>
            {' · '}
            {data.author}
            {data.merged_by ? ` · merge por ${data.merged_by}` : null}
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          {data.description ? <p className="whitespace-pre-wrap text-sm">{data.description}</p> : null}
          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              onClick={() => openChat({ dmId: data.thread_id, title: `!${data.number} ${data.title}` })}
            >
              Abrir no XCHAT
            </Button>
            <a
              href={`${XGROUP_CORP_ORIGIN}/social`}
              className="inline-flex h-9 items-center rounded-md border border-input bg-background px-3 text-sm hover:bg-accent"
              target="_blank"
              rel="noreferrer"
            >
              Comentários no XGROUP
            </a>
            {data.status === 'open' ? (
              <>
                <Button type="button" disabled={busy} onClick={() => void act('merge')}>
                  {busy ? '…' : 'Mergear'}
                </Button>
                <Button type="button" variant="outline" disabled={busy} onClick={() => void act('close')}>
                  Fechar
                </Button>
              </>
            ) : null}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

export function StatusBadge({ status }: { status: MergeRequestStatus }) {
  const variant = status === 'merged' ? 'secondary' : status === 'closed' ? 'outline' : 'default'
  return <Badge variant={variant}>{STATUS_LABEL[status]}</Badge>
}
