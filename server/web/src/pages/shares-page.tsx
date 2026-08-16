import { useCallback } from 'react'
import { FolderOpen, Globe, ShieldCheck, User } from 'lucide-react'
import { api } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'

function serverAddress(wireguardAddress: string): string {
  return wireguardAddress.split('/')[0] ?? wireguardAddress
}

export function SharesPage() {
  const fetchConfig = useCallback(() => api.getConfig(), [])
  const { data: config, loading, error } = usePollingData(fetchConfig, 60_000)
  const host = config ? serverAddress(config.wireguard_address) : null

  return (
    <div className="flex flex-col gap-6">
      {error && <p className="text-sm text-destructive">{error}</p>}

      <Card>
        <CardHeader className="flex-row items-start gap-3 space-y-0">
          <ShieldCheck className="mt-0.5 size-5 shrink-0 text-muted-foreground" />
          <div>
            <CardTitle className="text-base">Só acessível com a VPN conectada</CardTitle>
            <CardDescription>
              xdriver (Samba + Drive web nativo) escuta exclusivamente na interface WireGuard — nunca no IP público. A autenticação do
              Samba é a própria VPN (<code className="font-mono text-xs">guest ok</code> +{' '}
              <code className="font-mono text-xs">force user</code>); o cliente desktop abre os shares sem pedir senha.
            </CardDescription>
          </div>
        </CardHeader>
      </Card>

      {loading || !config || !host ? (
        <Skeleton className="h-40 w-full" />
      ) : (
        <div className="grid gap-4 sm:grid-cols-2">
          <Card>
            <CardHeader>
              <User className="mb-1 size-6 text-muted-foreground" />
              <CardTitle className="text-base">Meus arquivos (pessoal)</CardTitle>
              <CardDescription>
                Share <code className="font-mono">home-&lt;usuário&gt;</code> criado quando o toggle Samba está ligado no
                painel (Fase 13/14). No cliente: botão &quot;Meus arquivos&quot;.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-1 text-sm">
              <p>
                Windows: <code className="font-mono">{String.raw`\\${host}\home-<usuario>`}</code>
              </p>
              <p>
                Linux/macOS: <code className="font-mono">{`smb://${host}/home-<usuario>`}</code>
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <FolderOpen className="mb-1 size-6 text-muted-foreground" />
              <CardTitle className="text-base">Compartilhado</CardTitle>
              <CardDescription>
                Share <code className="font-mono">[shared]</code> em <code className="font-mono">/srv/xvpn/shared</code>,
                também guest (qualquer peer da VPN). No cliente: botão &quot;Compartilhado&quot;.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-1 text-sm">
              <p>
                Windows: <code className="font-mono">{String.raw`\\${host}\shared`}</code>
              </p>
              <p>
                Linux/macOS: <code className="font-mono">{`smb://${host}/shared`}</code>
              </p>
            </CardContent>
          </Card>

          <Card className="sm:col-span-2">
            <CardHeader>
              <Globe className="mb-1 size-6 text-muted-foreground" />
              <CardTitle className="text-base">XDriver (web)</CardTitle>
              <CardDescription>Drive nativo em xdriver.corp — só na VPN. Sem FileBrowser.</CardDescription>
            </CardHeader>
            <CardContent className="text-sm">
              <a
                href="https://xdriver.corp.ihuull.com"
                target="_blank"
                rel="noreferrer"
                className="font-mono text-primary underline underline-offset-2"
              >
                https://xdriver.corp.ihuull.com
              </a>
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  )
}
