import { useEffect, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { KeyRound, Lock } from 'lucide-react'
import { api, ApiError } from '@/lib/api'
import { useAuth } from '@/lib/auth-context'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { ProgressBar } from '@/components/ui/progress-bar'

export function AccountPage() {
  const { user, isLoadingUser } = useAuth()

  return (
    <div className="flex flex-col gap-6">
      {isLoadingUser || !user ? (
        <Skeleton className="h-48 w-full" />
      ) : (
        <>
          <ChangePasswordCard />
          <ManualSSHKeyCard />
        </>
      )}
    </div>
  )
}

function ChangePasswordCard() {
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    setError(null)
    if (newPassword.length < 8) {
      setError('A nova senha deve ter ao menos 8 caracteres')
      return
    }
    if (newPassword !== confirmPassword) {
      setError('A confirmação não confere com a nova senha')
      return
    }
    if (newPassword === currentPassword) {
      setError('A nova senha deve ser diferente da atual')
      return
    }
    setSubmitting(true)
    try {
      await api.changeMyPassword(currentPassword, newPassword)
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')
      toast.success('Senha atualizada')
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : 'Falha ao trocar senha'
      setError(msg)
      toast.error(msg)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-base">
          <Lock className="size-4" />
          Senha do painel
        </CardTitle>
        <CardDescription>
          Usada em /my/login e /admin/login. Não é senha Samba — o acesso a arquivos autentica pela VPN.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className="flex max-w-md flex-col gap-3">
          <div className="flex flex-col gap-2">
            <Label htmlFor="current-password">Senha atual</Label>
            <Input
              id="current-password"
              type="password"
              autoComplete="current-password"
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
              disabled={submitting}
              required
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="new-password">Nova senha</Label>
            <Input
              id="new-password"
              type="password"
              autoComplete="new-password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              disabled={submitting}
              minLength={8}
              required
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="confirm-password">Confirmar nova senha</Label>
            <Input
              id="confirm-password"
              type="password"
              autoComplete="new-password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              disabled={submitting}
              minLength={8}
              required
            />
          </div>
          {submitting && <ProgressBar label="Atualizando senha…" />}
          {error && <p className="text-sm text-destructive">{error}</p>}
          <div>
            <Button type="submit" disabled={submitting}>
              {submitting ? 'Salvando…' : 'Trocar senha'}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  )
}

function ManualSSHKeyCard() {
  const { user, isLoadingUser, refreshUser } = useAuth()
  const [sshKey, setSshKey] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    if (user) setSshKey(user.ssh_public_key ?? '')
  }, [user])

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      await api.updateMySSHPublicKey(sshKey)
      await refreshUser()
      toast.success(
        user?.sftp_enabled
          ? 'Chave SSH atualizada e aplicada no SFTP'
          : 'Chave salva — passa a valer quando o admin ligar o SFTP',
      )
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : 'Falha ao salvar chave'
      setError(msg)
      toast.error(msg)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-base">
          <KeyRound className="size-4" />
          Chave SSH manual (SFTP)
        </CardTitle>
        <CardDescription>
          Escape hatch para celular ou máquina sem o cliente XVPN. As chaves dos seus dispositivos VPN entram sozinhas
          quando você abre o app conectado. Esta caixa é só a chave extra.
        </CardDescription>
      </CardHeader>
      <CardContent>
        {isLoadingUser || !user ? (
          <Skeleton className="h-28 w-full" />
        ) : (
          <form onSubmit={handleSubmit} className="flex flex-col gap-3">
            {!user.sftp_enabled && (
              <p className="text-xs text-muted-foreground">
                Seu SFTP ainda não está ligado — a chave fica guardada e o admin ativa o acesso.
              </p>
            )}
            <div className="flex flex-col gap-2">
              <Label htmlFor="account-ssh-key">Chave pública</Label>
              <Textarea
                id="account-ssh-key"
                className="min-h-24 font-mono text-sm"
                placeholder="ssh-ed25519 AAAAC3NzaC1lZDI1NTE5... user@host"
                value={sshKey}
                onChange={(e) => setSshKey(e.target.value)}
                spellCheck={false}
                disabled={submitting}
              />
            </div>
            {submitting && <ProgressBar label="Salvando chave…" />}
            {error && <p className="text-sm text-destructive">{error}</p>}
            <div>
              <Button type="submit" disabled={submitting}>
                {submitting ? 'Salvando…' : 'Salvar chave'}
              </Button>
            </div>
          </form>
        )}
      </CardContent>
    </Card>
  )
}
