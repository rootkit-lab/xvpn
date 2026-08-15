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
import { FilterBar } from '@/components/filter-bar'
import { DataTable, type DataTableColumn } from '@/components/data-table'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'

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
  const canReview = isAdminRole(caller?.role)
  const [page, setPage] = useState(1)
  const [q, setQ] = useState('')
  const [status, setStatus] = useState<'all' | 'pending' | 'approved' | 'rejected'>('pending')

  const fetchWaitlist = useCallback(
    () => api.listWaitlist({ page, per_page: 25, q, status: status === 'all' ? undefined : status }),
    [page, q, status],
  )
  const { data, loading, error, reload } = usePollingData(fetchWaitlist, 15_000)

  const columns: DataTableColumn<WaitlistEntry>[] = [
    { key: 'name', header: 'Nome', cell: (e) => <span className="font-medium">{e.name}</span> },
    { key: 'email', header: 'E-mail', cell: (e) => <span className="text-muted-foreground">{e.email}</span> },
    {
      key: 'msg',
      header: 'Mensagem',
      cell: (e) => (
        <span className="max-w-xs truncate text-muted-foreground" title={e.message}>
          {e.message || '—'}
        </span>
      ),
    },
    {
      key: 'when',
      header: 'Cadastrado em',
      cell: (e) => <span className="whitespace-nowrap text-muted-foreground">{formatDateTime(e.created_at)}</span>,
    },
    {
      key: 'status',
      header: 'Status',
      cell: (e) => (
        <Badge variant={e.status === 'approved' ? 'default' : e.status === 'rejected' ? 'destructive' : 'secondary'}>
          {e.status === 'pending' ? 'Pendente' : e.status === 'approved' ? 'Aprovado' : 'Rejeitado'}
        </Badge>
      ),
    },
    {
      key: 'actions',
      header: '',
      className: 'text-right',
      cell: (e) =>
        canReview && e.status === 'pending' ? (
          <span className="flex justify-end gap-2" onClick={(ev) => ev.stopPropagation()}>
            <ProvisionDialog entry={e} onChanged={reload} />
            <RejectButton entry={e} onChanged={reload} />
          </span>
        ) : null,
    },
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
        placeholder="Buscar nome ou e-mail"
      >
        {(['all', 'pending', 'approved', 'rejected'] as const).map((s) => (
          <Button
            key={s}
            type="button"
            size="sm"
            variant={status === s ? 'default' : 'outline'}
            onClick={() => {
              setStatus(s)
              setPage(1)
            }}
          >
            {s === 'all' ? 'Todos' : s === 'pending' ? 'Pendentes' : s === 'approved' ? 'Aprovados' : 'Rejeitados'}
          </Button>
        ))}
      </FilterBar>
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Lista de espera</CardTitle>
        </CardHeader>
        <CardContent>
          <DataTable
            columns={columns}
            rows={data?.items ?? []}
            rowKey={(e) => e.id}
            loading={loading || !data}
            emptyTitle="Nenhum cadastro neste filtro."
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

function RejectButton({ entry, onChanged }: { entry: WaitlistEntry; onChanged: () => void }) {
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
    <Button variant="ghost" size="icon" disabled={rejecting} title="Rejeitar" onClick={handleReject}>
      <X className="size-4 text-destructive" />
    </Button>
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
