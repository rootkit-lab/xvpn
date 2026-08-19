import { useCallback, useState, type FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import { QRCodeSVG } from 'qrcode.react'
import { api, ApiError, type Device, type DeviceSSHKey, type InviteResponse, type User } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { formatDateTime } from '@/lib/format'
import { useAuth } from '@/lib/auth-context'
import {
  canManageRole,
  canWriteAdminProduct,
  isAdminRole,
  PRODUCT_LABELS,
  ROLE_BADGE_VARIANT,
  ROLE_LABELS,
  type Product,
  type Role,
} from '@/lib/roles'
import { ProductScopeFields } from '@/components/product-scope-fields'
import { CopyField } from '@/components/copy-field'
import { RoleSelect } from '@/components/role-select'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Skeleton } from '@/components/ui/skeleton'
import { ProgressBar } from '@/components/ui/progress-bar'
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

export function UserDetailPage() {
  const { id } = useParams()
  const userId = Number(id)
  const fetchUser = useCallback(() => api.getUser(userId), [userId])
  const { data: user, loading, error, reload } = usePollingData(fetchUser, 30_000)

  if (!Number.isFinite(userId) || userId < 1) {
    return <p className="text-sm text-destructive">ID inválido.</p>
  }
  if (loading || !user) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-64 w-full" />
  }

  return <UserFicha user={user} onChanged={reload} />
}

function UserFicha({ user, onChanged }: { user: User; onChanged: () => void }) {
  const { user: caller } = useAuth()
  const navigate = useNavigate()
  const canManage = isAdminRole(caller?.role) && canManageRole(caller?.role, user.role)
  const canManageFiles = canManage && canWriteAdminProduct(caller?.role, caller?.products, 'xdriver')
  const isSelf = caller?.id === user.id

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="text-sm text-muted-foreground">
            <button type="button" className="underline-offset-4 hover:underline" onClick={() => navigate('/admin/users')}>
              Usuários
            </button>
            <span className="mx-2">/</span>
            {user.username}
          </p>
          <div className="mt-1 flex items-center gap-2">
            <h2 className="text-xl font-semibold">{user.username}</h2>
            <Badge variant={ROLE_BADGE_VARIANT[user.role]}>{ROLE_LABELS[user.role]}</Badge>
          </div>
        </div>
      </div>

      <Tabs defaultValue="geral">
        <TabsList>
          <TabsTrigger value="geral">Geral</TabsTrigger>
          <TabsTrigger value="arquivos">Arquivos</TabsTrigger>
          <TabsTrigger value="dispositivos">Dispositivos</TabsTrigger>
          <TabsTrigger value="convites">Convites</TabsTrigger>
          <TabsTrigger value="seguranca">Segurança</TabsTrigger>
        </TabsList>
        <TabsContent value="geral">
          <GeralTab user={user} caller={caller} isSelf={isSelf} canManage={canManage} onChanged={onChanged} />
        </TabsContent>
        <TabsContent value="arquivos">
          <ArquivosTab user={user} canManage={canManageFiles} onChanged={onChanged} />
        </TabsContent>
        <TabsContent value="dispositivos">
          <DevicesTab userId={user.id} />
        </TabsContent>
        <TabsContent value="convites">
          <ConvitesTab user={user} canInvite={isAdminRole(caller?.role)} />
        </TabsContent>
        <TabsContent value="seguranca">
          <SegurancaTab user={user} canManage={canManage} onChanged={onChanged} />
        </TabsContent>
      </Tabs>
    </div>
  )
}

function GeralTab({
  user,
  caller,
  isSelf,
  canManage,
  onChanged,
}: {
  user: User
  caller: User | null
  isSelf: boolean
  canManage: boolean
  onChanged: () => void
}) {
  const [username, setUsername] = useState(user.username)
  const [role, setRole] = useState<Role>(user.role)
  const [products, setProducts] = useState<Product[]>(user.products ?? [])
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const showProducts = role === 'admin'
  const sameProducts =
    (user.products ?? []).length === products.length && (user.products ?? []).every((p) => products.includes(p))

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      const changes: { username?: string; role?: Role; products?: Product[] } = {}
      if (username !== user.username) changes.username = username
      if (!isSelf && role !== user.role) changes.role = role
      if (!isSelf && showProducts && !sameProducts) changes.products = products
      await api.updateUser(user.id, changes)
      toast.success('Usuário atualizado')
      onChanged()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Falha ao atualizar')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Conta</CardTitle>
        <CardDescription>Criado em {formatDateTime(user.created_at)}</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className="flex max-w-md flex-col gap-4">
          <div className="flex flex-col gap-2">
            <Label htmlFor="edit-username">Usuário</Label>
            <Input
              id="edit-username"
              required
              value={username}
              disabled={!canManage}
              onChange={(e) => setUsername(e.target.value)}
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label>Papel</Label>
            {isSelf || !canManage ? (
              <p className="text-sm text-muted-foreground">
                {ROLE_LABELS[user.role]}
                {isSelf ? ' — não é possível alterar o próprio papel.' : ''}
              </p>
            ) : (
              <RoleSelect value={role} onChange={setRole} caller={caller?.role} />
            )}
          </div>
          {showProducts && (
            <ProductScopeFields
              value={products}
              onChange={setProducts}
              disabled={!canManage || isSelf}
              hint={
                isSelf
                  ? 'Ninguém altera o próprio escopo.'
                  : caller?.role === 'admin' && (caller.products?.length ?? 0) > 0
                    ? 'Você só pode conceder produtos do seu próprio escopo.'
                    : undefined
              }
            />
          )}
          {user.role === 'admin' && (user.products?.length ?? 0) > 0 && !canManage && (
            <p className="text-xs text-muted-foreground">
              Escopo: {user.products?.map((p) => PRODUCT_LABELS[p]).join(', ')}
            </p>
          )}
          {error && <p className="text-sm text-destructive">{error}</p>}
          {canManage && (
            <Button type="submit" disabled={submitting}>
              {submitting ? 'Salvando…' : 'Salvar'}
            </Button>
          )}
        </form>
      </CardContent>
    </Card>
  )
}

function ArquivosTab({ user, canManage, onChanged }: { user: User; canManage: boolean; onChanged: () => void }) {
  const [sftp, setSftp] = useState(Boolean(user.sftp_enabled))
  const [samba, setSamba] = useState(Boolean(user.samba_enabled))
  const [sshKey, setSshKey] = useState(user.ssh_public_key ?? '')
  const [quotaMB, setQuotaMB] = useState(String(user.disk_quota_mb ?? 0))
  const fetchKeys = useCallback(async () => {
    try {
      return await api.listUserSSHKeys(user.id)
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) return { device_keys: [] as DeviceSSHKey[] }
      throw err
    }
  }, [user.id])
  const { data: keysData, loading: keysLoading } = usePollingData(fetchKeys, 60_000)
  const deviceKeys: DeviceSSHKey[] = keysData?.device_keys ?? []
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    const quota = Number(quotaMB)
    if (!Number.isFinite(quota) || quota < 0 || !Number.isInteger(quota)) {
      setError('Quota inválida (inteiro ≥ 0; 0 = sem limite)')
      return
    }
    setError(null)
    setSubmitting(true)
    try {
      await api.setFileAccess(user.id, {
        sftp_enabled: sftp,
        samba_enabled: samba,
        ssh_public_key: sshKey,
        disk_quota_mb: quota,
      })
      toast.success('Acesso a arquivos atualizado')
      onChanged()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Falha ao atualizar acesso')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>SFTP e Samba</CardTitle>
        <CardDescription>Só respondem em 10.66.66.1, dentro da VPN.</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className="flex max-w-xl flex-col gap-4">
          <label className="flex items-center gap-3 text-sm">
            <input type="checkbox" className="size-4" checked={sftp} disabled={!canManage} onChange={(e) => setSftp(e.target.checked)} />
            SFTP (chroot em /home/{user.username}/files)
          </label>
          <label className="flex items-center gap-3 text-sm">
            <input type="checkbox" className="size-4" checked={samba} disabled={!canManage} onChange={(e) => setSamba(e.target.checked)} />
            Samba [home-{user.username}]
          </label>
          <div className="flex flex-col gap-2">
            <Label htmlFor="quota">Quota (MiB)</Label>
            <Input id="quota" type="number" min={0} value={quotaMB} disabled={!canManage} onChange={(e) => setQuotaMB(e.target.value)} />
          </div>
          <div>
            <Label>Chaves dos dispositivos</Label>
            {keysLoading ? (
              <ProgressBar label="Carregando…" />
            ) : deviceKeys.length === 0 ? (
              <p className="text-xs text-muted-foreground">Nenhuma chave automática ainda.</p>
            ) : (
              <ul className="mt-2 space-y-1 font-mono text-xs">
                {deviceKeys.map((k) => (
                  <li key={k.device_id}>
                    {k.device_name} — {k.fingerprint}
                  </li>
                ))}
              </ul>
            )}
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="ssh-extra">Chave pública extra</Label>
            <textarea
              id="ssh-extra"
              className="min-h-24 w-full rounded-md border border-input bg-background px-3 py-2 font-mono text-sm"
              value={sshKey}
              disabled={!canManage}
              onChange={(e) => setSshKey(e.target.value)}
            />
          </div>
          {error && <p className="text-sm text-destructive">{error}</p>}
          {canManage && (
            <Button type="submit" disabled={submitting}>
              {submitting ? 'Aplicando…' : 'Aplicar'}
            </Button>
          )}
        </form>
      </CardContent>
    </Card>
  )
}

function DevicesTab({ userId }: { userId: number }) {
  const fetchDevices = useCallback(() => api.listDevices({ per_page: 100, q: '' }), [])
  const { data, loading } = usePollingData(fetchDevices, 15_000)
  const mine = (data?.items ?? []).filter((d: Device) => d.user_id === userId)

  if (loading || !data) return <Skeleton className="h-32 w-full" />
  if (mine.length === 0) return <p className="text-sm text-muted-foreground">Nenhum dispositivo deste usuário.</p>
  return (
    <ul className="space-y-2">
      {mine.map((d) => (
        <li key={d.id} className="rounded-md border border-white/8 px-3 py-2 text-sm">
          <span className="font-medium">{d.name}</span>
          <span className="ml-2 text-muted-foreground">{d.allowed_ip}</span>
        </li>
      ))}
    </ul>
  )
}

function ConvitesTab({ user, canInvite }: { user: User; canInvite: boolean }) {
  const [invite, setInvite] = useState<InviteResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [generating, setGenerating] = useState(false)

  async function generate() {
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

  return (
    <Card>
      <CardHeader>
        <CardTitle>Convite de dispositivo</CardTitle>
        <CardDescription>Uso único e curta duração. Copie agora.</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col items-start gap-4">
        {canInvite && (
          <Button onClick={generate} disabled={generating}>
            {generating ? 'Gerando…' : 'Gerar convite'}
          </Button>
        )}
        {error && <p className="text-sm text-destructive">{error}</p>}
        {invite && (
          <>
            <div className="rounded-md border bg-white p-3">
              <QRCodeSVG value={JSON.stringify({ invite_token: invite.token })} size={176} />
            </div>
            <CopyField label="Código" value={invite.token} />
            <p className="text-xs text-muted-foreground">Expira em {formatDateTime(invite.expires_at)}</p>
          </>
        )}
      </CardContent>
    </Card>
  )
}

function SegurancaTab({ user, canManage, onChanged }: { user: User; canManage: boolean; onChanged: () => void }) {
  const navigate = useNavigate()
  const [password, setPassword] = useState('')
  const [generated, setGenerated] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [deleting, setDeleting] = useState(false)

  async function handleReset(event: FormEvent) {
    event.preventDefault()
    setSubmitting(true)
    setError(null)
    try {
      const resp = await api.resetPassword(user.id, password || undefined)
      if (resp.password) setGenerated(resp.password)
      else toast.success('Senha redefinida')
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Falha ao redefinir senha')
    } finally {
      setSubmitting(false)
    }
  }

  async function handleDelete() {
    setDeleting(true)
    try {
      await api.deleteUser(user.id)
      toast.success(`Usuário "${user.username}" removido`)
      navigate('/admin/users')
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao remover')
      onChanged()
    } finally {
      setDeleting(false)
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <Card>
        <CardHeader>
          <CardTitle>Redefinir senha</CardTitle>
        </CardHeader>
        <CardContent>
          {generated ? (
            <CopyField label="Nova senha" value={generated} />
          ) : (
            <form onSubmit={handleReset} className="flex max-w-md flex-col gap-3">
              <Input
                type="password"
                minLength={8}
                value={password}
                disabled={!canManage}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="Em branco = gerar automaticamente"
              />
              {error && <p className="text-sm text-destructive">{error}</p>}
              {canManage && (
                <Button type="submit" disabled={submitting}>
                  {submitting ? 'Redefinindo…' : 'Redefinir'}
                </Button>
              )}
            </form>
          )}
        </CardContent>
      </Card>
      {canManage && (
        <Card className="border-destructive/40">
          <CardHeader>
            <CardTitle>Excluir conta</CardTitle>
            <CardDescription>Revoga dispositivos WireGuard. Irreversível.</CardDescription>
          </CardHeader>
          <CardContent>
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button variant="destructive" disabled={deleting}>
                  Remover usuário
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>Remover "{user.username}"?</AlertDialogTitle>
                  <AlertDialogDescription>Essa ação não pode ser desfeita.</AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>Cancelar</AlertDialogCancel>
                  <AlertDialogAction onClick={handleDelete}>Remover</AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </CardContent>
        </Card>
      )}
    </div>
  )
}
