import { useCallback, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { Copy, FolderKey, KeyRound, Pencil, Plus, Ticket, Trash2 } from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { api, ApiError, type DeviceSSHKey, type InviteResponse, type User } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { formatDateTime } from '@/lib/format'
import { useAuth } from '@/lib/auth-context'
import { canManageRole, isAdminRole, ROLE_BADGE_VARIANT, ROLE_LABELS, type Role } from '@/lib/roles'
import { CopyField } from '@/components/copy-field'
import { RoleSelect } from '@/components/role-select'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
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
import { ProgressBar } from '@/components/ui/progress-bar'

export function UsersPage() {
  const { user: caller } = useAuth()
  const fetchUsers = useCallback(() => api.listUsers(), [])
  const { data: users, loading, reload } = usePollingData(fetchUsers, 30_000)
  const canCreate = isAdminRole(caller?.role)

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">Usuários</h1>
          <p className="text-muted-foreground">Contas com acesso à VPN e ao painel administrativo.</p>
        </div>
        {canCreate && <CreateUserDialog onCreated={reload} />}
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
                  <TableHead>Papel</TableHead>
                  <TableHead>Criado em</TableHead>
                  <TableHead className="text-right">Ações</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {users.map((u) => (
                  <UserRow key={u.id} user={u} caller={caller} onChanged={reload} />
                ))}
                {users.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={4} className="text-center text-muted-foreground">
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

function UserRow({ user, caller, onChanged }: { user: User; caller: User | null; onChanged: () => void }) {
  const [deleting, setDeleting] = useState(false)
  const canInvite = isAdminRole(caller?.role)
  const canManage = isAdminRole(caller?.role) && canManageRole(caller?.role, user.role)
  const isSelf = caller?.id === user.id

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
      <TableCell>
        <Badge variant={ROLE_BADGE_VARIANT[user.role]}>{ROLE_LABELS[user.role]}</Badge>
      </TableCell>
      <TableCell className="text-muted-foreground">{formatDateTime(user.created_at)}</TableCell>
      <TableCell className="flex justify-end gap-2">
        {canInvite && <InviteDialog user={user} />}
        {canManage && <FileAccessDialog user={user} onChanged={onChanged} />}
        {canManage && <EditUserDialog user={user} caller={caller} isSelf={isSelf} onChanged={onChanged} />}
        {canManage && <ResetPasswordDialog user={user} />}
        {canManage && (
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
        )}
      </TableCell>
    </TableRow>
  )
}

function CreateUserDialog({ onCreated }: { onCreated: () => void }) {
  const { user: caller } = useAuth()
  const [open, setOpen] = useState(false)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<Role>('member')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      await api.createUser(username, password, role)
      toast.success(`Usuário "${username}" criado`)
      setUsername('')
      setPassword('')
      setRole('member')
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
            <div className="flex flex-col gap-2">
              <Label>Papel</Label>
              <RoleSelect value={role} onChange={setRole} caller={caller?.role} />
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

function EditUserDialog({
  user,
  caller,
  isSelf,
  onChanged,
}: {
  user: User
  caller: User | null
  isSelf: boolean
  onChanged: () => void
}) {
  const [open, setOpen] = useState(false)
  const [username, setUsername] = useState(user.username)
  const [role, setRole] = useState<Role>(user.role)
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  function handleOpenChange(next: boolean) {
    setOpen(next)
    if (next) {
      setUsername(user.username)
      setRole(user.role)
      setError(null)
    }
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      const changes: { username?: string; role?: Role } = {}
      if (username !== user.username) changes.username = username
      if (!isSelf && role !== user.role) changes.role = role
      await api.updateUser(user.id, changes)
      toast.success(`Usuário "${username}" atualizado`)
      setOpen(false)
      onChanged()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Falha ao atualizar usuário')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button variant="ghost" size="icon" title="Editar usuário">
          <Pencil className="size-4" />
        </Button>
      </DialogTrigger>
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Editar "{user.username}"</DialogTitle>
            <DialogDescription>Altere o nome de usuário e/ou o papel no painel.</DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-4 py-4">
            <div className="flex flex-col gap-2">
              <Label htmlFor={`edit-username-${user.id}`}>Usuário</Label>
              <Input
                id={`edit-username-${user.id}`}
                required
                value={username}
                onChange={(e) => setUsername(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label>Papel</Label>
              {isSelf ? (
                <p className="text-sm text-muted-foreground">
                  Não é possível alterar o próprio papel — peça a outro administrador.
                </p>
              ) : (
                <RoleSelect value={role} onChange={setRole} caller={caller?.role} />
              )}
            </div>
            {error && <p className="text-sm text-destructive">{error}</p>}
          </div>
          <DialogFooter>
            <Button type="submit" disabled={submitting}>
              {submitting ? 'Salvando…' : 'Salvar'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function ResetPasswordDialog({ user }: { user: User }) {
  const [open, setOpen] = useState(false)
  const [password, setPassword] = useState('')
  const [generated, setGenerated] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  function handleOpenChange(next: boolean) {
    setOpen(next)
    if (!next) {
      setPassword('')
      setGenerated(null)
      setError(null)
    }
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      const resp = await api.resetPassword(user.id, password || undefined)
      if (resp.password) {
        setGenerated(resp.password)
      } else {
        toast.success(`Senha de "${user.username}" redefinida`)
        setOpen(false)
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Falha ao redefinir senha')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button variant="ghost" size="icon" title="Redefinir senha">
          <KeyRound className="size-4" />
        </Button>
      </DialogTrigger>
      <DialogContent>
        {generated ? (
          <>
            <DialogHeader>
              <DialogTitle>Senha redefinida</DialogTitle>
              <DialogDescription>Copie agora — essa senha não será exibida de novo.</DialogDescription>
            </DialogHeader>
            <div className="py-4">
              <CopyField label="Nova senha" value={generated} />
            </div>
            <DialogFooter>
              <Button onClick={() => setOpen(false)}>Fechar</Button>
            </DialogFooter>
          </>
        ) : (
          <form onSubmit={handleSubmit}>
            <DialogHeader>
              <DialogTitle>Redefinir senha de "{user.username}"</DialogTitle>
              <DialogDescription>
                Deixe em branco para gerar uma senha aleatória, ou defina uma manualmente (mín. 8 caracteres).
              </DialogDescription>
            </DialogHeader>
            <div className="flex flex-col gap-4 py-4">
              <div className="flex flex-col gap-2">
                <Label htmlFor={`reset-password-${user.id}`}>Nova senha (opcional)</Label>
                <Input
                  id={`reset-password-${user.id}`}
                  type="password"
                  minLength={8}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="Gerar automaticamente"
                />
              </div>
              {error && <p className="text-sm text-destructive">{error}</p>}
            </div>
            <DialogFooter>
              <Button type="submit" disabled={submitting}>
                {submitting ? 'Redefinindo…' : 'Redefinir senha'}
              </Button>
            </DialogFooter>
          </form>
        )}
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

// FileAccessDialog (Fase 13, PLAN.md §6.9): controla o acesso a
// arquivos do servidor por usuário — SFTP (chrooted, chave pública)
// e Samba (share [home-<user>] acessível só via wg0). O back-end
// provisiona a conta Unix via binário privilegiado e mantém o DB
// consistente com o sistema (ver api.handleSetFileAccess).
function FileAccessDialog({ user, onChanged }: { user: User; onChanged: () => void }) {
  const [open, setOpen] = useState(false)
  const [sftp, setSftp] = useState(false)
  const [samba, setSamba] = useState(false)
  const [sshKey, setSshKey] = useState('')
  const [deviceKeys, setDeviceKeys] = useState<DeviceSSHKey[]>([])
  const [keysLoading, setKeysLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  function handleOpenChange(next: boolean) {
    setOpen(next)
    if (next) {
      setSftp(Boolean(user.sftp_enabled))
      setSamba(Boolean(user.samba_enabled))
      setSshKey(user.ssh_public_key ?? '')
      setDeviceKeys([])
      setError(null)
      setKeysLoading(true)
      void api.listUserSSHKeys(user.id).then(
        (res) => setDeviceKeys(res.device_keys ?? []),
        (err) => {
          setDeviceKeys([])
          if (err instanceof ApiError && err.status === 403) {
            setError(err.message)
          }
        },
      ).finally(() => setKeysLoading(false))
    }
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      await api.setFileAccess(user.id, {
        sftp_enabled: sftp,
        samba_enabled: samba,
        ssh_public_key: sshKey,
      })
      toast.success(
        sftp || samba
          ? `Acesso de "${user.username}" atualizado — shares/SFTP já valem na VPN`
          : `Acesso a arquivos de "${user.username}" desligado`,
      )
      setOpen(false)
      onChanged()
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : 'Falha ao atualizar acesso a arquivos'
      setError(msg)
      toast.error(msg)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button variant="ghost" size="icon" title="Acesso a arquivos (SFTP/Samba)">
          <FolderKey className="size-4" />
        </Button>
      </DialogTrigger>
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Acesso a arquivos — "{user.username}"</DialogTitle>
            <DialogDescription>
              Habilita SFTP (chrooted, autenticação por chave pública) e/ou Samba (share acessível só via VPN).
              A conta Unix é criada automaticamente no servidor. A chave SSH chega sozinha quando a pessoa abre o XVPN.
            </DialogDescription>
          </DialogHeader>
          <fieldset disabled={submitting} className="flex flex-col gap-4 py-4 disabled:opacity-70">
            <label className="flex items-center gap-3 text-sm">
              <input
                type="checkbox"
                className="size-4"
                checked={sftp}
                onChange={(e) => setSftp(e.target.checked)}
              />
              <span>
                <strong>SFTP</strong> — acesso via <code>sftp://vpn.officeempresa.com</code> (chrooted em <code>/home/{user.username}/files</code>)
              </span>
            </label>
            <label className="flex items-center gap-3 text-sm">
              <input
                type="checkbox"
                className="size-4"
                checked={samba}
                onChange={(e) => setSamba(e.target.checked)}
              />
              <span>
                <strong>Samba</strong> — share <code>[home-{user.username}]</code> acessível só pela VPN (guest + force user)
              </span>
            </label>
            <div className="flex flex-col gap-2">
              <Label>Chaves dos dispositivos (automáticas)</Label>
              {keysLoading ? (
                <ProgressBar label="Carregando chaves registradas…" />
              ) : deviceKeys.length === 0 ? (
                <p className="text-xs text-muted-foreground">
                  Nenhuma chave registrada ainda. Elas aparecem aqui quando a pessoa abrir o XVPN conectado à VPN —
                  não é preciso colar nada para ligar o SFTP.
                </p>
              ) : (
                <ul className="space-y-1.5 rounded-md border border-border p-2 font-mono text-xs">
                  {deviceKeys.map((k) => (
                    <li key={k.device_id} className="flex flex-col gap-0.5">
                      <span className="text-foreground">{k.device_name}</span>
                      <span className="text-muted-foreground">{k.fingerprint}</span>
                    </li>
                  ))}
                </ul>
              )}
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor={`ssh-key-${user.id}`}>Chave pública extra (opcional)</Label>
              <textarea
                id={`ssh-key-${user.id}`}
                className="min-h-24 w-full rounded-md border border-input bg-background px-3 py-2 text-sm font-mono"
                placeholder="ssh-ed25519 AAAAC3NzaC1lZDI1NTE5... user@host"
                value={sshKey}
                onChange={(e) => setSshKey(e.target.value)}
                spellCheck={false}
              />
              <p className="text-xs text-muted-foreground">
                Escape hatch para celular ou máquina sem XVPN. Pode incluir múltiplas chaves, uma por linha.
              </p>
            </div>
            {submitting && (
              <ProgressBar label="Aplicando no servidor (cria conta Unix / atualiza Samba e SFTP)…" />
            )}
            {error && <p className="text-sm text-destructive">{error}</p>}
          </fieldset>
          <DialogFooter>
            <Button type="submit" disabled={submitting}>
              {submitting ? 'Aplicando…' : 'Aplicar'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
