import { useCallback, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { Copy, Plus, Ticket, Trash2 } from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { api, ApiError, type InviteResponse, type User } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { formatDateTime } from '@/lib/format'
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
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'

export function UsersPage() {
  const fetchUsers = useCallback(() => api.listUsers(), [])
  const { data: users, loading, reload } = usePollingData(fetchUsers, 30_000)

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">Usuários</h1>
          <p className="text-muted-foreground">Contas com acesso à VPN e ao painel administrativo.</p>
        </div>
        <CreateUserDialog onCreated={reload} />
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Todos os usuários</CardTitle>
        </CardHeader>
        <CardContent>
          {loading || !users ? (
            <Skeleton className="h-32 w-full" />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Usuário</TableHead>
                  <TableHead>Criado em</TableHead>
                  <TableHead className="text-right">Ações</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {users.map((user) => (
                  <UserRow key={user.id} user={user} onChanged={reload} />
                ))}
                {users.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={3} className="text-center text-muted-foreground">
                      Nenhum usuário cadastrado ainda.
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

function UserRow({ user, onChanged }: { user: User; onChanged: () => void }) {
  const [deleting, setDeleting] = useState(false)

  async function handleDelete() {
    setDeleting(true)
    try {
      await api.deleteUser(user.id)
      toast.success(`Usuário "${user.username}" removido`)
      onChanged()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao remover usuário')
    } finally {
      setDeleting(false)
    }
  }

  return (
    <TableRow>
      <TableCell className="font-medium">{user.username}</TableCell>
      <TableCell className="text-muted-foreground">{formatDateTime(user.created_at)}</TableCell>
      <TableCell className="flex justify-end gap-2">
        <InviteDialog user={user} />
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button variant="ghost" size="icon" disabled={deleting}>
              <Trash2 className="size-4 text-destructive" />
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Remover usuário "{user.username}"?</AlertDialogTitle>
              <AlertDialogDescription>
                Isso remove o usuário e revoga todos os dispositivos associados a ele na interface WireGuard.
                Essa ação não pode ser desfeita.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancelar</AlertDialogCancel>
              <AlertDialogAction onClick={handleDelete}>Remover</AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </TableCell>
    </TableRow>
  )
}

function CreateUserDialog({ onCreated }: { onCreated: () => void }) {
  const [open, setOpen] = useState(false)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      await api.createUser(username, password)
      toast.success(`Usuário "${username}" criado`)
      setUsername('')
      setPassword('')
      setOpen(false)
      onCreated()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Falha ao criar usuário')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button>
          <Plus className="size-4" />
          Novo usuário
        </Button>
      </DialogTrigger>
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Novo usuário</DialogTitle>
            <DialogDescription>
              Cria uma conta com acesso à VPN. Depois de criada, gere um convite para registrar o primeiro
              dispositivo.
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-4 py-4">
            <div className="flex flex-col gap-2">
              <Label htmlFor="new-username">Usuário</Label>
              <Input id="new-username" required value={username} onChange={(e) => setUsername(e.target.value)} />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="new-password">Senha</Label>
              <Input
                id="new-password"
                type="password"
                required
                minLength={8}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </div>
            {error && <p className="text-sm text-destructive">{error}</p>}
          </div>
          <DialogFooter>
            <Button type="submit" disabled={submitting}>
              {submitting ? 'Criando…' : 'Criar usuário'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function InviteDialog({ user }: { user: User }) {
  const [open, setOpen] = useState(false)
  const [invite, setInvite] = useState<InviteResponse | null>(null)
  const [generating, setGenerating] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleOpenChange(nextOpen: boolean) {
    setOpen(nextOpen)
    if (nextOpen && !invite) {
      setGenerating(true)
      setError(null)
      try {
        setInvite(await api.createInvite(user.id))
      } catch (err) {
        setError(err instanceof ApiError ? err.message : 'Falha ao gerar convite')
      } finally {
        setGenerating(false)
      }
    }
    if (!nextOpen) {
      // Convite é de uso único e curta duração — não faz sentido mantê-lo
      // em memória depois que o diálogo fecha (ver frontend-react.mdc).
      setInvite(null)
    }
  }

  function copyToken() {
    if (!invite) return
    navigator.clipboard.writeText(invite.token)
    toast.success('Código copiado')
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button variant="ghost" size="icon" title="Gerar convite">
          <Ticket className="size-4" />
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Convite para "{user.username}"</DialogTitle>
          <DialogDescription>
            Use no cliente desktop para registrar um novo dispositivo. Válido por tempo limitado e uso único.
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col items-center gap-4 py-4">
          {generating && <Skeleton className="h-48 w-48" />}
          {error && <p className="text-sm text-destructive">{error}</p>}
          {invite && (
            <>
              <div className="rounded-md border bg-white p-3">
                <QRCodeSVG value={JSON.stringify({ invite_token: invite.token })} size={192} />
              </div>
              <div className="flex items-center gap-2">
                <code className="rounded bg-muted px-3 py-1.5 text-sm font-medium">{invite.token}</code>
                <Button variant="outline" size="icon" onClick={copyToken}>
                  <Copy className="size-4" />
                </Button>
              </div>
              <p className="text-xs text-muted-foreground">Expira em {formatDateTime(invite.expires_at)}</p>
            </>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
