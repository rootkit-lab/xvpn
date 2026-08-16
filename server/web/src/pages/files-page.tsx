import { ExternalLink, FolderOpen, Globe, ShieldCheck, Terminal, User } from 'lucide-react'
import { CopyField } from '@/components/copy-field'
import { useAuth } from '@/lib/auth-context'
import { FILEBROWSER_URL, personalShareName, sambaUnc, sambaUri, VPN_FILE_HOST } from '@/lib/vpn-files'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'

export function FilesPage() {
  const { user, isLoadingUser } = useAuth()

  if (isLoadingUser || !user) {
    return <Skeleton className="h-64 w-full" />
  }

  const personal = personalShareName(user.username)
  const sftpCmd = `sftp ${user.username}@${VPN_FILE_HOST}`

  return (
    <div className="flex flex-col gap-6">
      <Card>
        <CardHeader className="flex-row items-start gap-3 space-y-0">
          <ShieldCheck className="mt-0.5 size-5 shrink-0 text-muted-foreground" />
          <div>
            <CardTitle className="text-base">Só com o túnel ligado</CardTitle>
            <CardDescription>
              Os serviços escutam em <code className="font-mono text-xs">{VPN_FILE_HOST}</code> (interface WireGuard),
              nunca no IP público. A autenticação do Samba é a própria VPN — não há senha Samba separada.
            </CardDescription>
          </div>
        </CardHeader>
        <CardContent className="flex flex-wrap gap-2">
          <Badge variant={user.samba_enabled ? 'default' : 'outline'}>
            Samba {user.samba_enabled ? 'ligado' : 'desligado'}
          </Badge>
          <Badge variant={user.sftp_enabled ? 'default' : 'outline'}>
            SFTP {user.sftp_enabled ? 'ligado' : 'desligado'}
          </Badge>
        </CardContent>
      </Card>

      <div className="grid gap-4 sm:grid-cols-2">
        <Card>
          <CardHeader>
            <User className="mb-1 size-6 text-muted-foreground" />
            <CardTitle className="text-base">Meus arquivos</CardTitle>
            <CardDescription>
              Share pessoal <code className="font-mono">{personal}</code>
              {!user.samba_enabled && ' — peça a um admin para ligar o Samba nesta conta.'}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <CopyField label="Windows" value={sambaUnc(personal)} />
            <CopyField label="Linux / macOS" value={sambaUri(personal)} />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <FolderOpen className="mb-1 size-6 text-muted-foreground" />
            <CardTitle className="text-base">Compartilhado</CardTitle>
            <CardDescription>
              Share <code className="font-mono">shared</code> — qualquer peer da VPN, com o túnel ativo.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <CopyField label="Windows" value={sambaUnc('shared')} />
            <CopyField label="Linux / macOS" value={sambaUri('shared')} />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <Globe className="mb-1 size-6 text-muted-foreground" />
            <CardTitle className="text-base">XDriver</CardTitle>
            <CardDescription>Drive na intranet — só com o túnel ligado.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <CopyField label="URL" value={FILEBROWSER_URL} />
            <Button variant="outline" size="sm" asChild>
              <a href={FILEBROWSER_URL} target="_blank" rel="noreferrer">
                <ExternalLink className="size-4" />
                Abrir XDriver
              </a>
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <Terminal className="mb-1 size-6 text-muted-foreground" />
            <CardTitle className="text-base">SFTP</CardTitle>
            <CardDescription>
              {user.sftp_enabled
                ? 'Use a chave SSH da sua conta (Editar conta) ou a chave auto-registrada pelo cliente.'
                : 'Desligado nesta conta — um admin liga o SFTP na tela de usuários.'}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <CopyField label="Comando" value={sftpCmd} />
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
