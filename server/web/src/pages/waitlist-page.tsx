import { useCallback, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { UserPlus, X } from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { api, ApiError, type ProvisionWaitlistResponse, type WaitlistEntry } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { formatDateTime } from '@/lib/format'
import { useAuth } from '@/lib/auth-context'
import { isAdminRole, type Role } from '@/lib/roles'
import { CopyField } from '@/components/copy-field'
import { RoleSelect } from '@/components/role-select'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'

// suggestUsername vira o ponto de partida editável do campo "Usuário" no
// diálogo de provisionamento — só remove acentos/espaços do nome
// cadastrado na waitlist, unicidade de fato é validada pelo backend.
function suggestUsername(name: string): string {
  return name
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '.')
    .replace(/^\.+|\.+$/g, '')
}

export function WaitlistPage() {
  const { user: caller } = useAuth()
  const fetchWaitlist = useCallback(() => api.listWaitlist(), [])
  const { data: entries, loading, error, reload } = usePollingData(fetchWaitlist, 15_000)
  const canReview = isAdminRole(caller?.role)

  const pending = entries?.filter((e) => e.status === 'pending') ?? []
  const reviewed = entries?.filter((e) => e.status !== 'pending') ?? []

  return (
    <div className="flex flex-col gap-6">
      {error && <p className="text-sm text-destructive">{error}</p>}

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Pendentes ({pending.length})</CardTitle>
        </CardHeader>
        <CardContent>
          {loading || !entries ? (
            <Skeleton className="h-32 w-full" />
          ) : (
            <WaitlistTable entries={pending} onChanged={reload} showActions={canReview} />
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
  const [rejecting, setRejecting] = useState(false)

  async function handleReject() {
    setRejecting(true)
    try {
      await api.rejectWaitlist(entry.id)
      toast.success(`"${entry.name}" rejeitado`)
      onChanged()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao rejeitar cadastro')
    } finally {
      setRejecting(false)
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
          <ProvisionDialog entry={entry} onChanged={onChanged} />
          <Button variant="ghost" size="icon" disabled={rejecting} title="Rejeitar" onClick={handleReject}>
            <X className="size-4 text-destructive" />
          </Button>
        </TableCell>
      )}
    </TableRow>
  )
}

function ProvisionDialog({ entry, onChanged }: { entry: WaitlistEntry; onChanged: () => void }) {
  const { user: caller } = useAuth()
  const [open, setOpen] = useState(false)
  const [username, setUsername] = useState(() => suggestUsername(entry.name))
  const [role, setRole] = useState<Role>('member')
  const [result, setResult] = useState<ProvisionWaitlistResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  function handleOpenChange(next: boolean) {
    setOpen(next)
    if (next) {
      setUsername(suggestUsername(entry.name))
      setRole('member')
      setResult(null)
      setError(null)
    } else {
      // Senha e convite gerados são de uso único — não ficam em memória
      // depois que o diálogo fecha (mesmo padrão de InviteDialog na tela
      // Usuários, ver frontend-react.mdc).
      setResult(null)
    }
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      const resp = await api.provisionWaitlist(entry.id, username, role)
      setResult(resp)
      onChanged()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Falha ao provisionar usuário')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button variant="ghost" size="icon" title="Aprovar e provisionar">
          <UserPlus className="size-4 text-primary" />
        </Button>
      </DialogTrigger>
      <DialogContent>
        {result ? (
          <>
            <DialogHeader>
              <DialogTitle>Usuário "{result.user.username}" provisionado</DialogTitle>
              <DialogDescription>Copie a senha e o convite agora — não serão exibidos de novo.</DialogDescription>
            </DialogHeader>
            <div className="flex flex-col gap-4 py-4">
              <CopyField label="Senha" value={result.password} />
              <div className="flex flex-col items-center gap-3">
                <div className="rounded-md border bg-white p-3">
                  <QRCodeSVG value={JSON.stringify({ invite_token: result.invite.token })} size={176} />
                </div>
                <CopyField label="Código de convite" value={result.invite.token} />
                <p className="text-xs text-muted-foreground">
                  Convite expira em {formatDateTime(result.invite.expires_at)}
                </p>
              </div>
            </div>
            <DialogFooter>
              <Button onClick={() => setOpen(false)}>Fechar</Button>
            </DialogFooter>
          </>
        ) : (
          <form onSubmit={handleSubmit}>
            <DialogHeader>
              <DialogTitle>Aprovar e provisionar "{entry.name}"</DialogTitle>
              <DialogDescription>
                Cria o usuário e o convite num só passo, e marca este cadastro como aprovado.
              </DialogDescription>
            </DialogHeader>
            <div className="flex flex-col gap-4 py-4">
              <div className="flex flex-col gap-2">
                <Label htmlFor={`provision-username-${entry.id}`}>Usuário</Label>
                <Input
                  id={`provision-username-${entry.id}`}
                  required
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label>Papel</Label>
                <RoleSelect value={role} onChange={setRole} caller={caller?.role} />
              </div>
              {error && <p className="text-sm text-destructive">{error}</p>}
            </div>
            <DialogFooter>
              <Button type="submit" disabled={submitting}>
                {submitting ? 'Provisionando…' : 'Provisionar'}
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  )
}
