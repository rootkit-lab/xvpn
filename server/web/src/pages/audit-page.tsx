import { useCallback, useState } from 'react'
import { api, type AuditLog } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { formatDateTime } from '@/lib/format'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { FilterBar } from '@/components/filter-bar'
import { DataTable, type DataTableColumn } from '@/components/data-table'

export function AuditPage() {
  const [page, setPage] = useState(1)
  const [q, setQ] = useState('')
  const fetchAudit = useCallback(() => api.listAudit({ page, per_page: 25, q }), [page, q])
  const { data, loading, error } = usePollingData(fetchAudit, 15_000)

  const columns: DataTableColumn<AuditLog>[] = [
    {
      key: 'when',
      header: 'Quando',
      cell: (log) => <span className="whitespace-nowrap text-muted-foreground">{formatDateTime(log.created_at)}</span>,
    },
    { key: 'actor', header: 'Ator', cell: (log) => <span className="font-medium">{log.actor}</span> },
    {
      key: 'action',
      header: 'Ação',
      cell: (log) => <code className="text-sm">{log.action}</code>,
    },
    { key: 'detail', header: 'Detalhe', cell: (log) => <span className="text-muted-foreground">{log.detail || '—'}</span> },
  ]

  return (
    <div className="flex flex-col gap-6">
      {error && <p className="text-sm text-destructive">{error}</p>}
      <FilterBar
        q={q}
        onQChange={(next) => {
          setQ(next)
          setPage(1)
        }}
        placeholder="Buscar ator ou ação"
      />
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Log de ações</CardTitle>
        </CardHeader>
        <CardContent>
          <DataTable
            columns={columns}
            rows={data?.items ?? []}
            rowKey={(log) => log.id}
            loading={loading || !data}
            emptyTitle="Nenhuma ação registrada ainda."
            page={data?.page ?? page}
            perPage={data?.per_page ?? 25}
            total={data?.total ?? 0}
            onPageChange={setPage}
          />
        </CardContent>
      </Card>
    </div>
  )
}
