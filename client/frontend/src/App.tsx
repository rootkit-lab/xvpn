import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { AlertTriangle, Loader2 } from 'lucide-react'

import { VPNService } from '../bindings/github.com/rootkit-lab/xvpn/client'
import type { StatusView } from '../bindings/github.com/rootkit-lab/xvpn/client'
import { EnrollmentPage } from './pages/enrollment-page'
import { MainPage } from './pages/main-page'

// Intervalo de polling do status — rápido o suficiente para a UI parecer
// "ao vivo" (handshake, tráfego) sem sobrecarregar o helper com chamadas
// IPC constantes.
const POLL_INTERVAL_MS = 2000

function App() {
  const [status, setStatus] = useState<StatusView | null>(null)
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    try {
      const result = await VPNService.Status()
      setStatus(result)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }, [])

  useEffect(() => {
    refresh()
    const interval = setInterval(refresh, POLL_INTERVAL_MS)
    return () => clearInterval(interval)
  }, [refresh])

  if (!status) {
    return (
      <CenteredMessage>
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </CenteredMessage>
    )
  }

  if (!status.helperReachable) {
    return (
      <CenteredMessage title="Serviço indisponível">
        <AlertTriangle className="mb-2 h-8 w-8 text-destructive" />
        <p>
          O serviço <code className="rounded bg-secondary px-1 py-0.5">xvpn-client-helper</code>{' '}
          não está acessível.
        </p>
        <p className="mt-1">
          Verifique se ele está instalado e rodando e tente novamente.
        </p>
      </CenteredMessage>
    )
  }

  if (!status.enrolled) {
    return <EnrollmentPage onEnrolled={refresh} />
  }

  return <MainPage status={status} onChange={refresh} error={error} />
}

function CenteredMessage({ title, children }: { title?: string; children: ReactNode }) {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-2 p-6 text-center text-sm text-muted-foreground">
      {title && <h2 className="text-base font-semibold text-foreground">{title}</h2>}
      {children}
    </div>
  )
}

export default App
