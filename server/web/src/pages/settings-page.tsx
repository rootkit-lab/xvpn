import { useCallback } from 'react'
import { api } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'

export function SettingsPage() {
  const fetchConfig = useCallback(() => api.getConfig(), [])
  const { data: config, loading, error } = usePollingData(fetchConfig, 60_000)

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="text-2xl font-semibold">Configurações</h1>
        <p className="text-muted-foreground">Parâmetros de rede atuais do servidor (somente leitura).</p>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Rede WireGuard</CardTitle>
          <CardDescription>
            Edição via painel ainda não é suportada — altere via variáveis de ambiente do servidor (ver{' '}
            <code>server/README.md</code>) e reinicie o serviço.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {loading || !config ? (
            <Skeleton className="h-40 w-full" />
          ) : (
            <dl className="grid gap-4 sm:grid-cols-2">
              <SettingItem label="Interface" value={config.wireguard_interface} />
              <SettingItem label="Endereço da interface" value={config.wireguard_address} />
              <SettingItem label="Sub-rede de peers" value={config.wireguard_allowed_subnet} />
              <SettingItem label="Porta de escuta (UDP)" value={String(config.wireguard_listen_port)} />
              <SettingItem label="Endpoint público" value={config.wireguard_endpoint} />
              <SettingItem label="Chave pública do servidor" value={config.server_public_key} mono />
              <SettingItem label="Validade do convite" value={`${config.invite_token_ttl_minutes} min`} />
              <SettingItem label="Validade da sessão do painel" value={`${config.jwt_token_ttl_minutes} min`} />
            </dl>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function SettingItem({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div>
      <dt className="text-sm text-muted-foreground">{label}</dt>
      <dd className={mono ? 'break-all font-mono text-sm' : 'font-medium'}>{value}</dd>
    </div>
  )
}
