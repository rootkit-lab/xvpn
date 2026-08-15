import { Link } from 'react-router-dom'
import { FolderOpen, KeyRound, Laptop, Pencil, Shield } from 'lucide-react'
import { useCallback } from 'react'
import { api, type Device } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { formatDateTime } from '@/lib/format'
import { useAuth } from '@/lib/auth-context'
import { ROLE_BADGE_VARIANT, ROLE_CAPABILITIES, ROLE_DESCRIPTIONS, ROLE_LABELS } from '@/lib/roles'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'

const HANDSHAKE_RECENT_THRESHOLD_MS = 3 * 60 * 1000

function isOnline(device: Device): boolean {
  if (!device.last_handshake) return false
  return Date.now() - new Date(device.last_handshake).getTime() < HANDSHAKE_RECENT_THRESHOLD_MS
}

export function ProfilePage() {
  const { user, isLoadingUser } = useAuth()
  const fetchDevices = useCallback(() => api.listMyDevices(), [])
  const { data: devices, loading: loadingDevices } = usePollingData(fetchDevices, 30_000)

  if (isLoadingUser || !user) {
    return <Skeleton className="h-64 w-full" />
  }

  const onlineCount = devices?.filter(isOnline).length ?? 0
  const myCaps = ROLE_CAPABILITIES.filter((c) => c.roles.includes(user.role))
  const quota = user.disk_quota_mb && user.disk_quota_mb > 0 ? `${user.disk_quota_mb} MB` : 'sem cota definida'
  const hasSSH = Boolean(user.ssh_public_key?.trim())

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <p className="mb-1 text-[11px] font-medium uppercase tracking-[0.14em] text-muted-foreground/70">Meu espaço</p>
          <h1 className="text-2xl font-semibold tracking-tight">Perfil</h1>
          <p className="text-muted-foreground">Como a conta aparece no XVPN — só leitura. Para senha e chave SSH, use Editar conta.</p>
        </div>
        <Button asChild>
          <Link to="/app/account">
            <Pencil className="size-4" />
            Editar minha conta
          </Link>
        </Button>
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Identidade</CardTitle>
            <CardDescription>Nome de usuário e papel no painel.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            <ProfileRow label="Usuário" value={user.username} />
            <div className="flex items-center justify-between gap-3">
              <span className="text-muted-foreground">Papel</span>
              <Badge variant={ROLE_BADGE_VARIANT[user.role]}>{ROLE_LABELS[user.role]}</Badge>
            </div>
            <p className="text-muted-foreground">{ROLE_DESCRIPTIONS[user.role]}</p>
            <ProfileRow label="Conta criada" value={formatDateTime(user.created_at)} />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">Acesso a arquivos</CardTitle>
            <CardDescription>Ligado pelo administrador. Os caminhos estão em Arquivos.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            <div className="flex items-center justify-between gap-3">
              <span className="text-muted-foreground">SFTP</span>
              <Badge variant={user.sftp_enabled ? 'default' : 'outline'}>
                {user.sftp_enabled ? 'ligado' : 'desligado'}
              </Badge>
            </div>
            <div className="flex items-center justify-between gap-3">
              <span className="text-muted-foreground">Samba</span>
              <Badge variant={user.samba_enabled ? 'default' : 'outline'}>
                {user.samba_enabled ? 'ligado' : 'desligado'}
              </Badge>
            </div>
            <ProfileRow label="Cota de disco" value={quota} />
            <ProfileRow label="Chave SSH manual" value={hasSSH ? 'cadastrada' : 'não cadastrada'} />
            <Button variant="outline" size="sm" asChild>
              <Link to="/app/files">
                <FolderOpen className="size-4" />
                Ver caminhos
              </Link>
            </Button>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <Laptop className="size-4" />
            Dispositivos
          </CardTitle>
          <CardDescription>Resumo dos peers WireGuard desta conta.</CardDescription>
        </CardHeader>
        <CardContent>
          {loadingDevices || !devices ? (
            <Skeleton className="h-16 w-full" />
          ) : (
            <div className="flex flex-wrap items-center gap-6 text-sm">
              <p>
                <span className="font-semibold">{devices.length}</span>
                <span className="text-muted-foreground"> registrados</span>
              </p>
              <p>
                <span className="font-semibold">{onlineCount}</span>
                <span className="text-muted-foreground"> online agora</span>
              </p>
              <Button variant="outline" size="sm" asChild>
                <Link to="/app">Gerenciar dispositivos</Link>
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <Shield className="size-4" />
            O que este papel pode
          </CardTitle>
          <CardDescription>Permissões do seu papel atual — um administrador altera isso na tela de usuários.</CardDescription>
        </CardHeader>
        <CardContent>
          <ul className="grid gap-2 text-sm sm:grid-cols-2">
            {myCaps.map((cap) => (
              <li key={cap.id} className="flex items-start gap-2 text-muted-foreground">
                <KeyRound className="mt-0.5 size-3.5 shrink-0 text-primary" />
                {cap.label}
              </li>
            ))}
          </ul>
        </CardContent>
      </Card>
    </div>
  )
}

function ProfileRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3">
      <span className="text-muted-foreground">{label}</span>
      <span className="truncate font-medium">{value}</span>
    </div>
  )
}
