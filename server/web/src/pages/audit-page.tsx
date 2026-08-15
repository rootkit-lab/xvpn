import { useCallback } from 'react'
import { api } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { formatDateTime } from '@/lib/format'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Skeleton } from '@/components/ui/skeleton'

export function AuditPage() {
  const fetchAudit = useCallback(() => api.listAudit(), [])
  const { data: logs, loading, error } = usePollingData(fetchAudit, 15_000)

  return (
    <div className="flex flex-col gap-6">
      {error && <p className="text-sm text-destructive">{error}</p>}

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Log de ações</CardTitle>
        </CardHeader>
        <CardContent>
          {loading || !logs ? (
            <Skeleton className="h-64 w-full" />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Quando</TableHead>
                  <TableHead>Ator</TableHead>
                  <TableHead>Ação</TableHead>
                  <TableHead>Detalhe</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {logs.map((log) => (
                  <TableRow key={log.id}>
                    <TableCell className="whitespace-nowrap text-muted-foreground">
                      {formatDateTime(log.created_at)}
                    </TableCell>
                    <TableCell className="font-medium">{log.actor}</TableCell>
                    <TableCell>
                      <code className="text-sm">{log.action}</code>
                    </TableCell>
                    <TableCell className="text-muted-foreground">{log.detail || '—'}</TableCell>
                  </TableRow>
                ))}
                {logs.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={4} className="text-center text-muted-foreground">
                      Nenhuma ação registrada ainda.
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
