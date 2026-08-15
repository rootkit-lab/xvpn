import { useCallback, useEffect, useState } from 'react'
import { motion } from 'framer-motion'
import { Download, Loader2, RefreshCw } from 'lucide-react'

import type { DiagnosticsReport } from '../../bindings/github.com/rootkit-lab/xvpn/client'
import { VPNService } from '../../bindings/github.com/rootkit-lab/xvpn/client'
import { WatchIconButton, WatchPageHeader, WatchShell } from '@/components/watch-chrome'

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
    <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} transition={{ duration: 0.2 }} className="h-full">
      <WatchShell scroll className="gap-4">
        <WatchPageHeader
          title="Diagnóstico"
          onBack={onBack}
          trailing={
            <>
              <WatchIconButton onClick={run} label="Atualizar" disabled={loading} filled>
                {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" strokeWidth={2} />}
              </WatchIconButton>
              <WatchIconButton onClick={exportReport} label="Exportar relatório" disabled={!report} filled>
                <Download className="h-4 w-4" strokeWidth={2} />
              </WatchIconButton>
            </>
          }
        />

        {error && <p className="relative z-10 font-display text-[13px] text-destructive">{error}</p>}

        {report && (
          <div className="relative z-10 flex flex-col gap-2.5">
            <div className="watch-complication grid grid-cols-2 gap-3 rounded-[18px] px-3.5 py-3">
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
            </div>

            <div className="watch-complication grid grid-cols-2 gap-3 rounded-[18px] px-3.5 py-3">
              <Info label="IP atribuído" value={report.assignedIP || '—'} />
              <Info label="Endpoint do servidor" value={report.serverEndpoint || '—'} />
              <Info label="Painel" value={report.serverBaseURL || '—'} />
              <Info
                label="Último handshake"
                value={report.lastHandshakeAgoSeconds != null ? `${report.lastHandshakeAgoSeconds}s atrás` : '—'}
              />
            </div>
          </div>
        )}

        <div className="relative z-10 flex min-h-[140px] flex-1 flex-col gap-1.5 overflow-hidden">
          <p className="font-display text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground/75">
            Logs recentes
          </p>
          <div className="watch-complication flex-1 overflow-y-auto rounded-[18px] px-3 py-2.5 font-mono text-[11px] leading-relaxed text-muted-foreground">
            {logs.length === 0 ? (
              <p>Sem logs disponíveis.</p>
            ) : (
              logs.map((line, i) => <div key={i}>{line}</div>)
            )}
          </div>
        </div>
      </WatchShell>
    </motion.div>
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
  const tone = ok
    ? 'text-primary'
    : neutralWhenFalse
      ? 'text-muted-foreground'
      : 'text-destructive'
  const text = ok ? 'OK' : neutralWhenFalse ? '—' : 'Falhou'

  return (
    <div className="flex flex-col gap-0.5">
      <span className="font-display text-[10px] uppercase tracking-[0.12em] text-muted-foreground/75">{label}</span>
      <span className={`font-display text-[13px] font-semibold ${tone}`}>{text}</span>
      {detail && <span className="font-display text-[11px] text-muted-foreground">{detail}</span>}
    </div>
  )
}

function Info({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="font-display text-[10px] uppercase tracking-[0.12em] text-muted-foreground/75">{label}</span>
      <span className="font-display text-[13px] font-semibold tracking-tight">{value}</span>
    </div>
  )
}
