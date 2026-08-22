import { useCallback, useMemo, useState } from 'react'
import { Activity, RefreshCw, Server } from 'lucide-react'
import { toast } from 'sonner'
import { ProductHeader } from '@xvpn/ui/react/product-header'
import { api, type MonitorCheck, type MonitorNode } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { canWriteAdminProduct, isAdminRole } from '@/lib/roles'
import { AccountMenu } from '@/components/layout/account-menu'
import { AppLauncher } from '@/components/layout/app-launcher'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { DataTable, type DataTableColumn } from '@/components/data-table'

const statusClass: Record<string, string> = {
  ok: 'text-emerald-400',
  warn: 'text-amber-400',
  critical: 'text-red-400',
  skipped: 'text-muted-foreground',
}

export function XmonitorPage() {
  const { user } = useAuth()
  const canRefresh = isAdminRole(user?.role) && canWriteAdminProduct(user?.role, user?.products, 'compute')
  const fetchDashboard = useCallback(() => api.getXmonitorDashboard(), [])
  const { data, loading, reload } = usePollingData(fetchDashboard, 30_000)
  const [refreshing, setRefreshing] = useState(false)
  const [checkPage, setCheckPage] = useState(1)
  const [nodePage, setNodePage] = useState(1)

  const checks = data?.checks ?? []
  const nodes = data?.nodes ?? []
  const checkPageSize = Math.max(checks.length, 1)
  const nodePageSize = Math.max(nodes.length, 1)

  const checkColumns: DataTableColumn<MonitorCheck>[] = useMemo(
    () => [
      { key: 'name', header: 'Check', cell: (r) => <span className="font-medium">{r.name}</span> },
      {
        key: 'status',
        header: 'Status',
        cell: (r) => (
          <span className={`uppercase text-xs font-semibold ${statusClass[r.status] ?? ''}`}>{r.status}</span>
        ),
      },
      { key: 'summary', header: 'Resumo', cell: (r) => <span className="text-sm text-muted-foreground">{r.summary}</span> },
      {
        key: 'at',
        header: 'Quando',
        cell: (r) => (
          <span className="hud-mono text-xs text-muted-foreground">
            {r.checked_at ? new Date(r.checked_at).toLocaleString() : '—'}
          </span>
        ),
      },
    ],
    [],
  )

  const nodeColumns: DataTableColumn<MonitorNode>[] = useMemo(
    () => [
      { key: 'host', header: 'Nó', cell: (r) => <span className="font-medium">{r.hostname}</span> },
      { key: 'wg', header: 'WG IP', cell: (r) => <code className="text-xs">{r.wg_ip || '—'}</code> },
      { key: 'role', header: 'Papel', cell: (r) => r.role || '—' },
      { key: 'load', header: 'Load', cell: (r) => (r.reported_at ? r.load1.toFixed(2) : '—') },
      {
        key: 'disk',
        header: 'Disco',
        cell: (r) =>
          r.reported_at ? (
            <span className="text-sm">
              {r.disk_used_pct.toFixed(0)}% · {r.disk_avail_gb.toFixed(0)} GiB livres
            </span>
          ) : (
            '—'
          ),
      },
    ],
    [],
  )

  const onRefresh = async () => {
    if (!canRefresh) return
    setRefreshing(true)
    try {
      await api.refreshXmonitor()
      await reload()
      toast.success('Probes atualizados')
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Falha ao atualizar')
    } finally {
      setRefreshing(false)
    }
  }

  return (
    <div data-product="xmonitor" className="watch-face relative flex min-h-svh flex-col overflow-hidden">
      <div className="watch-vignette pointer-events-none absolute inset-0" aria-hidden="true" />
      <ProductHeader
        product="xmonitor"
        href="/"
        trailing={
          user ? (
            <>
              {canRefresh ? (
                <Button variant="outline" size="sm" disabled={refreshing} onClick={onRefresh}>
                  <RefreshCw className={`mr-2 size-4 ${refreshing ? 'animate-spin' : ''}`} />
                  Atualizar
                </Button>
              ) : null}
              <AppLauncher variant="user" />
              <AccountMenu variant="user" />
            </>
          ) : null
        }
      />

      <main className="relative z-10 mx-auto flex w-full max-w-5xl flex-1 flex-col gap-6 px-6 py-10">
        <div>
          <p className="hud-label text-muted-foreground/70">Operação</p>
          <h1 className="font-display text-3xl font-semibold tracking-tight">XMONITOR</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Saúde da intranet, peers WireGuard e nós da malha — probes do control-plane (sem SSH ao data).
          </p>
        </div>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between gap-4">
            <div>
              <CardTitle className="flex items-center gap-2 text-base">
                <Activity className="size-4" />
                Checks
              </CardTitle>
              <CardDescription>
                {data?.updated_at
                  ? `Última rodada: ${new Date(data.updated_at).toLocaleString()}`
                  : loading
                    ? 'Carregando…'
                    : 'Nenhuma rodada ainda'}
              </CardDescription>
            </div>
          </CardHeader>
          <CardContent>
            <DataTable
              columns={checkColumns}
              rows={checks}
              rowKey={(r) => r.slug}
              loading={loading && !data}
              emptyTitle="Sem probes"
              page={checkPage}
              perPage={checkPageSize}
              total={checks.length}
              onPageChange={setCheckPage}
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base">
              <Server className="size-4" />
              Nós mesh
            </CardTitle>
            <CardDescription>
              Métricas de disco/load chegam via token do agente (`POST /api/xmonitor/report` na VPN).
            </CardDescription>
          </CardHeader>
          <CardContent>
            <DataTable
              columns={nodeColumns}
              rows={nodes}
              rowKey={(r) => r.hostname}
              loading={loading && !data}
              emptyTitle="Nenhum servidor cadastrado"
              page={nodePage}
              perPage={nodePageSize}
              total={nodes.length}
              onPageChange={setNodePage}
            />
          </CardContent>
        </Card>
      </main>
    </div>
  )
}
