import { useCallback, useState } from 'react'
import { toast } from 'sonner'
import { Check, X } from 'lucide-react'
import { api, ApiError, type WaitlistEntry } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { formatDateTime } from '@/lib/format'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Skeleton } from '@/components/ui/skeleton'

export function WaitlistPage() {
  const fetchWaitlist = useCallback(() => api.listWaitlist(), [])
  const { data: entries, loading, error, reload } = usePollingData(fetchWaitlist, 15_000)

  const pending = entries?.filter((e) => e.status === 'pending') ?? []
  const reviewed = entries?.filter((e) => e.status !== 'pending') ?? []

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="text-2xl font-semibold">Lista de espera</h1>
        <p className="text-muted-foreground">
          Cadastros recebidos pela landing pública em "/". Aprovar aqui só marca o interesse como liberado —
          crie o usuário e o convite normalmente na tela Usuários.
        </p>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Pendentes ({pending.length})</CardTitle>
        </CardHeader>
        <CardContent>
          {loading || !entries ? (
            <Skeleton className="h-32 w-full" />
          ) : (
            <WaitlistTable entries={pending} onChanged={reload} showActions />
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Já avaliados</CardTitle>
        </CardHeader>
        <CardContent>
          {loading || !entries ? (
            <Skeleton className="h-32 w-full" />
          ) : (
            <WaitlistTable entries={reviewed} onChanged={reload} showActions={false} />
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function WaitlistTable({
  entries,
  onChanged,
  showActions,
}: {
  entries: WaitlistEntry[]
  onChanged: () => void
  showActions: boolean
}) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Nome</TableHead>
          <TableHead>E-mail</TableHead>
          <TableHead>Mensagem</TableHead>
          <TableHead>Cadastrado em</TableHead>
          <TableHead>Status</TableHead>
          {showActions && <TableHead className="text-right">Ações</TableHead>}
        </TableRow>
      </TableHeader>
      <TableBody>
        {entries.map((entry) => (
          <WaitlistRow key={entry.id} entry={entry} onChanged={onChanged} showActions={showActions} />
        ))}
        {entries.length === 0 && (
          <TableRow>
            <TableCell colSpan={showActions ? 6 : 5} className="text-center text-muted-foreground">
              Nenhum cadastro aqui.
            </TableCell>
          </TableRow>
        )}
      </TableBody>
    </Table>
  )
}

function WaitlistRow({
  entry,
  onChanged,
  showActions,
}: {
  entry: WaitlistEntry
  onChanged: () => void
  showActions: boolean
}) {
  const [working, setWorking] = useState(false)

  async function handleApprove() {
    setWorking(true)
    try {
      await api.approveWaitlist(entry.id)
      toast.success(`"${entry.name}" aprovado — crie o usuário na tela Usuários`)
      onChanged()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao aprovar cadastro')
    } finally {
      setWorking(false)
    }
  }

  async function handleReject() {
    setWorking(true)
    try {
      await api.rejectWaitlist(entry.id)
      toast.success(`"${entry.name}" rejeitado`)
      onChanged()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao rejeitar cadastro')
    } finally {
      setWorking(false)
    }
  }

  return (
    <TableRow>
      <TableCell className="font-medium">{entry.name}</TableCell>
      <TableCell className="text-muted-foreground">{entry.email}</TableCell>
      <TableCell className="max-w-xs truncate text-muted-foreground" title={entry.message}>
        {entry.message || '—'}
      </TableCell>
      <TableCell className="whitespace-nowrap text-muted-foreground">{formatDateTime(entry.created_at)}</TableCell>
      <TableCell>
        <Badge variant={entry.status === 'approved' ? 'default' : entry.status === 'rejected' ? 'destructive' : 'secondary'}>
          {entry.status === 'pending' ? 'Pendente' : entry.status === 'approved' ? 'Aprovado' : 'Rejeitado'}
        </Badge>
      </TableCell>
      {showActions && (
        <TableCell className="flex justify-end gap-2">
          <Button variant="ghost" size="icon" disabled={working} title="Aprovar" onClick={handleApprove}>
            <Check className="size-4 text-primary" />
          </Button>
          <Button variant="ghost" size="icon" disabled={working} title="Rejeitar" onClick={handleReject}>
            <X className="size-4 text-destructive" />
          </Button>
        </TableCell>
      )}
    </TableRow>
  )
}
