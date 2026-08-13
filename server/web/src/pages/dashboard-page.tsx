import { useCallback, type ComponentType } from 'react'
import { motion } from 'framer-motion'
import { Activity, ArrowDownUp, Laptop, Wifi } from 'lucide-react'
import { api } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { formatBytes, formatDuration } from '@/lib/format'
import { RadialGauge } from '@/components/radial-gauge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'

export function DashboardPage() {
  const fetchDashboard = useCallback(async () => {
    const [status, devices] = await Promise.all([api.status(), api.listDevices()])
    const totalTraffic = status.receive_bytes_total + status.transmit_bytes_total
    return { status, devices, totalTraffic }
  }, [])

  const { data, error, loading } = usePollingData(fetchDashboard, 10_000)
  const connectedRatio =
    !loading && data && data.status.total_peers > 0 ? (data.status.connected_peers / data.status.total_peers) * 100 : 0

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="text-2xl font-semibold">Dashboard</h1>
        <p className="text-muted-foreground">Visão geral da VPN em tempo real.</p>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <Card className="border-white/5 bg-card/60 sm:col-span-2 lg:col-span-1">
          <CardContent className="flex items-center gap-4 pt-6">
            <RadialGauge value={connectedRatio} size={72} strokeWidth={7}>
              <Wifi className="size-5 text-primary" />
            </RadialGauge>
            <div>
              <p className="text-sm text-muted-foreground">Peers conectados</p>
              {loading || !data ? (
                <Skeleton className="mt-1 h-6 w-16" />
              ) : (
                <p className="text-xl font-semibold">
                  {data.status.connected_peers} / {data.status.total_peers}
                </p>
              )}
            </div>
          </CardContent>
        </Card>
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

      <Card className="border-white/5 bg-card/60">
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
    <Card className="border-white/5 bg-card/60 transition-shadow hover:shadow-[0_0_24px_-10px_var(--color-glow)]">
      <CardContent className="flex items-center gap-4 pt-6">
        <div className="rounded-full bg-primary/15 p-3">
          <Icon className="size-5 text-primary" />
        </div>
        <div>
          <p className="text-sm text-muted-foreground">{label}</p>
          {value === undefined ? (
            <Skeleton className="mt-1 h-6 w-16" />
          ) : (
            <motion.p
              key={value}
              initial={{ opacity: 0, y: 4 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.25 }}
              className="text-xl font-semibold"
            >
              {value}
            </motion.p>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
