import { useCallback, useEffect, useState } from 'react'
import { ArrowLeft, Download, Loader2, RefreshCw } from 'lucide-react'

import type { DiagnosticsReport } from '../../bindings/github.com/rootkit-lab/xvpn/client'
import { VPNService } from '../../bindings/github.com/rootkit-lab/xvpn/client'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'

interface DiagnosticsPageProps {
  onBack: () => void
}

export function DiagnosticsPage({ onBack }: DiagnosticsPageProps) {
  const [report, setReport] = useState<DiagnosticsReport | null>(null)
  const [logs, setLogs] = useState<string[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const run = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [reportResult, logsResult] = await Promise.all([
        VPNService.RunDiagnostics(),
        VPNService.GetLogs().catch(() => []),
      ])
      setReport(reportResult)
      setLogs(logsResult ?? [])
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    run()
  }, [run])

  function exportReport() {
    if (!report) return
    const blob = new Blob([JSON.stringify({ report, logs }, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `xvpn-diagnostico-${new Date(report.generatedAt).getTime()}.json`
    link.click()
    URL.revokeObjectURL(url)
  }

  return (
    <div className="flex h-full flex-col gap-4 overflow-y-auto p-6">
      <header className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <button
            onClick={onBack}
            aria-label="Voltar"
            className="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
          >
            <ArrowLeft className="h-4 w-4" />
          </button>
          <h1 className="text-lg font-semibold">Diagnóstico</h1>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={run}
            disabled={loading}
            aria-label="Atualizar"
            className="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground disabled:opacity-50"
          >
            {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
          </button>
          <button
            onClick={exportReport}
            disabled={!report}
            aria-label="Exportar relatório"
            className="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground disabled:opacity-50"
          >
            <Download className="h-4 w-4" />
          </button>
        </div>
      </header>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {report && (
        <>
          <Card>
            <CardContent className="grid grid-cols-2 gap-3 p-4 text-sm">
              <Check label="Serviço local (helper)" ok={report.helperReachable} />
              <Check label="Dispositivo registrado" ok={report.enrolled} />
              <Check label="Túnel conectado" ok={report.connected} />
              <Check label="Kill switch ativo" ok={report.killSwitchActive} neutralWhenFalse />
              <Check
                label="Painel acessível (internet)"
                ok={report.panelReachable}
                detail={report.panelLatencyMs != null ? `${report.panelLatencyMs} ms` : report.panelError}
              />
              <Check
                label="Servidor na VPN acessível"
                ok={report.vpnGatewayReachable}
                detail={report.vpnGatewayLatencyMs != null ? `${report.vpnGatewayLatencyMs} ms` : undefined}
                neutralWhenFalse={!report.connected}
              />
            </CardContent>
          </Card>

          <Card>
            <CardContent className="grid grid-cols-2 gap-3 p-4 text-sm">
              <Info label="IP atribuído" value={report.assignedIP || '—'} />
              <Info label="Endpoint do servidor" value={report.serverEndpoint || '—'} />
              <Info label="Painel" value={report.serverBaseURL || '—'} />
              <Info
                label="Último handshake"
                value={report.lastHandshakeAgoSeconds != null ? `${report.lastHandshakeAgoSeconds}s atrás` : '—'}
              />
            </CardContent>
          </Card>
        </>
      )}

      <div className="flex flex-1 flex-col gap-1 overflow-hidden">
        <p className="text-xs font-medium text-muted-foreground">Logs recentes do serviço</p>
        <div className="flex-1 overflow-y-auto rounded-md border border-border bg-secondary/40 p-2 font-mono text-[11px] leading-relaxed text-muted-foreground">
          {logs.length === 0 ? (
            <p>Sem logs disponíveis.</p>
          ) : (
            logs.map((line, i) => <div key={i}>{line}</div>)
          )}
        </div>
      </div>
    </div>
  )
}

function Check({
  label,
  ok,
  detail,
  neutralWhenFalse,
}: {
  label: string
  ok: boolean
  detail?: string
  neutralWhenFalse?: boolean
}) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-xs text-muted-foreground">{label}</span>
      <Badge variant={ok ? 'default' : neutralWhenFalse ? 'outline' : 'destructive'} className="w-fit">
        {ok ? 'OK' : neutralWhenFalse ? '—' : 'Falhou'}
      </Badge>
      {detail && <span className="text-xs text-muted-foreground">{detail}</span>}
    </div>
  )
}

function Info({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-xs text-muted-foreground">{label}</span>
      <span className="font-medium">{value}</span>
    </div>
  )
}
