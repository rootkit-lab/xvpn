import { useCallback } from 'react'
import { FolderOpen, Globe, ShieldCheck } from 'lucide-react'
import { api } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'

// A interface do servidor é sempre um endereço com máscara (ex.:
// "10.66.66.1/24") — só o IP interessa aqui para montar os caminhos de
// acesso.
function serverAddress(wireguardAddress: string): string {
  return wireguardAddress.split('/')[0] ?? wireguardAddress
}

export function SharesPage() {
  const fetchConfig = useCallback(() => api.getConfig(), [])
  const { data: config, loading, error } = usePollingData(fetchConfig, 60_000)
  const host = config ? serverAddress(config.wireguard_address) : null

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="text-2xl font-semibold">Compartilhamentos</h1>
        <p className="text-muted-foreground">Diretórios do VPS compartilhados na rede privada.</p>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      <Card>
        <CardHeader className="flex-row items-start gap-3 space-y-0">
          <ShieldCheck className="mt-0.5 size-5 shrink-0 text-muted-foreground" />
          <div>
            <CardTitle className="text-base">Só acessível com a VPN conectada</CardTitle>
            <CardDescription>
              Samba e FileBrowser escutam exclusivamente na interface WireGuard do servidor — nunca no IP público.
              Conecte-se pelo cliente desktop antes de tentar acessar os endereços abaixo.
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
              <FolderOpen className="mb-1 size-6 text-muted-foreground" />
              <CardTitle className="text-base">Unidade de rede (Samba)</CardTitle>
              <CardDescription>Windows, Linux (GVFS) e macOS conseguem montar via SMB3.</CardDescription>
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

          <Card>
            <CardHeader>
              <Globe className="mb-1 size-6 text-muted-foreground" />
              <CardTitle className="text-base">Interface web (FileBrowser)</CardTitle>
              <CardDescription>Upload/download rápido pelo navegador, sem montar nada.</CardDescription>
            </CardHeader>
            <CardContent className="text-sm">
              <a
                href={`http://${host}:8081`}
                target="_blank"
                rel="noreferrer"
                className="font-mono text-primary underline underline-offset-2"
              >
                {`http://${host}:8081`}
              </a>
            </CardContent>
          </Card>
        </div>
      )}

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Acesso de usuários</CardTitle>
          <CardDescription>
            Por segurança, contas Samba não são criadas automaticamente a partir dos usuários deste painel — peça ao
            administrador do servidor para criar/remover seu acesso.
          </CardDescription>
        </CardHeader>
      </Card>
    </div>
  )
}
