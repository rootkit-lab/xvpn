import { ExternalLink, FolderOpen, Globe, ShieldCheck, Terminal, User } from 'lucide-react'
import { CopyField } from '@/components/copy-field'
import { useAuth } from '@/lib/auth-context'
import { XDRIVER_CORP_ORIGIN } from '@/lib/product-host'
import { personalShareName, sambaUnc, sambaUri, VPN_FILE_HOST } from '@/lib/vpn-files'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { StoreShell } from '@/components/layout/store-shell'

export function XDriverLayout() {
  return <StoreShell kind="xdriver" />
}

export function XDriverPortalPage() {
  const { user, isLoadingUser } = useAuth()

  if (isLoadingUser || !user) {
    return (
      <div className="mx-auto max-w-4xl px-4 py-8">
        <Skeleton className="h-64 w-full rounded-[22px]" />
      </div>
    )
  }

  const personal = personalShareName(user.username)
  const sftpCmd = `sftp ${user.username}@${VPN_FILE_HOST}`
  const filesOn = Boolean(user.samba_enabled || user.sftp_enabled)

  return (
    <div className="mx-auto flex w-full max-w-4xl flex-col gap-6 px-4 py-6 md:px-8">
      <div className="watch-complication flex flex-col gap-4 rounded-[22px] p-6 md:flex-row md:items-center">
        <div className="min-w-0 flex-1">
          <p className="hud-label text-muted-foreground/70">Meu Drive</p>
          <h1 className="font-display mt-1 text-2xl font-semibold tracking-tight">XDriver</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Os arquivos moram na VPN. Este site é o portal. O FileBrowser só abre em{' '}
            <code className="font-mono text-xs">{XDRIVER_CORP_ORIGIN.replace('https://', '')}</code> com o túnel
            ligado e o DNS do Chrome <em>seguro</em> desligado (senão o nome <code className="font-mono text-xs">.corp</code> não
            resolve).
          </p>
          <div className="mt-3 flex flex-wrap gap-2">
            <Badge variant={user.samba_enabled ? 'default' : 'outline'}>
              Samba {user.samba_enabled ? 'ligado' : 'desligado'}
            </Badge>
            <Badge variant={user.sftp_enabled ? 'default' : 'outline'}>
              SFTP {user.sftp_enabled ? 'ligado' : 'desligado'}
            </Badge>
          </div>
        </div>
        <Button size="lg" className="rounded-full" asChild disabled={!filesOn}>
          <a href={XDRIVER_CORP_ORIGIN} target="_blank" rel="noreferrer">
            <ExternalLink className="size-4" />
            Abrir XDriver
          </a>
        </Button>
      </div>

      <div className="watch-complication flex items-start gap-3 rounded-[18px] p-4">
        <ShieldCheck className="mt-0.5 size-5 shrink-0 text-muted-foreground" />
        <p className="text-sm text-muted-foreground">
          Samba e FileBrowser escutam só em <code className="font-mono text-xs">{VPN_FILE_HOST}</code> — nunca neste
          hostname público. Sem VPN, o botão acima não alcança os arquivos.
        </p>
      </div>

      <div className="grid gap-3 sm:grid-cols-2">
        <DriveCard
          icon={User}
          title="Meu Drive"
          hint={`Share ${personal}`}
          fields={[
            { label: 'Windows', value: sambaUnc(personal) },
            { label: 'Linux / macOS', value: sambaUri(personal) },
          ]}
        />
        <DriveCard
          icon={FolderOpen}
          title="Compartilhado"
          hint="Share shared — qualquer peer da VPN"
          fields={[
            { label: 'Windows', value: sambaUnc('shared') },
            { label: 'Linux / macOS', value: sambaUri('shared') },
          ]}
        />
        <DriveCard
          icon={Globe}
          title="Na web"
          hint="FileBrowser no túnel"
          action={{ href: XDRIVER_CORP_ORIGIN, label: 'Abrir FileBrowser' }}
        />
        <DriveCard
          icon={Terminal}
          title="SFTP"
          hint={user.sftp_enabled ? 'Chave SSH da conta' : 'Desligado nesta conta'}
          fields={[{ label: 'Comando', value: sftpCmd }]}
        />
      </div>
    </div>
  )
}

function DriveCard({
  icon: Icon,
  title,
  hint,
  fields,
  action,
}: {
  icon: typeof User
  title: string
  hint: string
  fields?: { label: string; value: string }[]
  action?: { href: string; label: string }
}) {
  return (
    <section className="watch-complication flex flex-col gap-3 rounded-[18px] p-5">
      <div className="flex items-start gap-3">
        <span className="icon-well flex size-10 items-center justify-center rounded-[12px]">
          <Icon className="size-4" />
        </span>
        <div>
          <h2 className="font-display text-[15px] font-semibold">{title}</h2>
          <p className="text-xs text-muted-foreground">{hint}</p>
        </div>
      </div>
      {fields?.map((field) => (
        <CopyField key={field.label} label={field.label} value={field.value} />
      ))}
      {action && (
        <Button variant="outline" size="sm" asChild>
          <a href={action.href} target="_blank" rel="noreferrer">
            <ExternalLink className="size-4" />
            {action.label}
          </a>
        </Button>
      )}
    </section>
  )
}
