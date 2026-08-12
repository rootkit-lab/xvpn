import { useCallback, type ComponentType } from 'react'
import { Activity, ArrowDownUp, Laptop, Wifi } from 'lucide-react'
import { api } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { formatBytes, formatDuration } from '@/lib/format'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'

export function DashboardPage() {
  const fetchDashboard = useCallback(async () => {
    const [status, devices] = await Promise.all([api.status(), api.listDevices()])
    const totalTraffic = devices.reduce((sum, d) => sum + d.receive_bytes + d.transmit_bytes, 0)
    return { status, devices, totalTraffic }
  }, [])

  const { data, error, loading } = usePollingData(fetchDashboard, 10_000)

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="text-2xl font-semibold">Dashboard</h1>
        <p className="text-muted-foreground">Visão geral da VPN em tempo real.</p>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <MetricCard
          icon={Wifi}
          label="Peers conectados"
          value={loading || !data ? undefined : `${data.status.connected_peers} / ${data.status.total_peers}`}
        />
        <MetricCard
          icon={Laptop}
          label="Dispositivos cadastrados"
          value={loading || !data ? undefined : String(data.devices.length)}
        />
        <MetricCard
          icon={ArrowDownUp}
          label="Tráfego total (acumulado)"
          value={loading || !data ? undefined : formatBytes(data.totalTraffic)}
        />
        <MetricCard
          icon={Activity}
          label="Uptime do servidor"
          value={loading || !data ? undefined : formatDuration(data.status.uptime_seconds)}
        />
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Status</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground">
          {loading || !data ? (
            <Skeleton className="h-4 w-64" />
          ) : (
            <p>
              API v{data.status.api_version} — {data.status.connected_peers} de {data.status.total_peers} peers com
              handshake nos últimos 3 minutos.
            </p>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function MetricCard({
  icon: Icon,
  label,
  value,
}: {
  icon: ComponentType<{ className?: string }>
  label: string
  value?: string
}) {
  return (
    <Card>
      <CardContent className="flex items-center gap-4 pt-6">
        <div className="rounded-full bg-primary/10 p-3">
          <Icon className="size-5 text-primary" />
        </div>
        <div>
          <p className="text-sm text-muted-foreground">{label}</p>
          {value === undefined ? <Skeleton className="mt-1 h-6 w-16" /> : <p className="text-xl font-semibold">{value}</p>}
        </div>
      </CardContent>
    </Card>
  )
}
