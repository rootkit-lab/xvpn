import { lazy, Suspense, useCallback, useEffect, useState, type ReactNode } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import { AlertTriangle, Loader2 } from 'lucide-react'

import { VPNService } from '../bindings/github.com/rootkit-lab/xvpn/client'
import type { StatusView } from '../bindings/github.com/rootkit-lab/xvpn/client'
import { EnrollmentPage } from './pages/enrollment-page'
import { MainPage } from './pages/main-page'

// Preferências e diagnóstico são visitados bem menos que a tela principal
// — code-split via React.lazy pra não pesar o primeiro paint da janela
// (que já é sensível: é a primeira coisa que aparece depois de clicar no
// ícone da bandeja).
const SettingsPage = lazy(() => import('./pages/settings-page').then((m) => ({ default: m.SettingsPage })))
const DiagnosticsPage = lazy(() => import('./pages/diagnostics-page').then((m) => ({ default: m.DiagnosticsPage })))
const AppsPage = lazy(() => import('./pages/apps-page').then((m) => ({ default: m.AppsPage })))

// Intervalo de polling do status — rápido o suficiente para a UI parecer
// "ao vivo" (handshake, tráfego) sem sobrecarregar o helper com chamadas
// IPC constantes.
const POLL_INTERVAL_MS = 2000

// Navegação simples por estado local, sem react-router: o app inteiro só
// tem estas telas, e todas dependem do mesmo status polled aqui — não
// compensa a dependência extra (ver ROADMAP.md Fase 6).
type View = 'main' | 'settings' | 'diagnostics' | 'apps'

function App() {
  const [status, setStatus] = useState<StatusView | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [view, setView] = useState<View>('main')

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

  let content: ReactNode
  let key: string = view

  if (!status) {
    key = 'loading'
    content = (
      <CenteredMessage>
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </CenteredMessage>
    )
  } else if (!status.helperReachable) {
    key = 'unreachable'
    content = (
      <CenteredMessage title="Serviço indisponível">
        <AlertTriangle className="mb-2 h-8 w-8 text-destructive" />
        <p>
          O serviço <code className="rounded bg-secondary px-1 py-0.5">xvpn-client-helper</code>{' '}
          não está acessível.
        </p>
        <p className="mt-1">Verifique se ele está instalado e rodando e tente novamente.</p>
      </CenteredMessage>
    )
  } else if (!status.enrolled) {
    key = 'enrollment'
    content = <EnrollmentPage onEnrolled={refresh} />
  } else if (view === 'settings') {
    content = <SettingsPage onBack={() => setView('main')} />
  } else if (view === 'diagnostics') {
    content = <DiagnosticsPage onBack={() => setView('main')} />
  } else if (view === 'apps') {
    content = <AppsPage status={status} onBack={() => setView('main')} />
  } else {
    content = (
      <MainPage
        status={status}
        onChange={refresh}
        error={error}
        onOpenSettings={() => setView('settings')}
        onOpenDiagnostics={() => setView('diagnostics')}
        onOpenApps={() => setView('apps')}
      />
    )
  }

  return (
    <Suspense fallback={<CenteredMessage><Loader2 className="h-6 w-6 animate-spin text-muted-foreground" /></CenteredMessage>}>
      <AnimatePresence mode="wait">
        <motion.div
          key={key}
          className="h-full"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.15 }}
        >
          {content}
        </motion.div>
      </AnimatePresence>
    </Suspense>
  )
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
