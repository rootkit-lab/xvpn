import { lazy, Suspense } from 'react'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { AuthProvider } from '@/lib/auth-context'
import { VIEWER_UP_ROLES } from '@/lib/roles'
import { ProtectedRoute } from '@/components/layout/protected-route'
import { AppShell } from '@/components/layout/app-shell'
import { PageFallback } from '@/components/layout/page-fallback'
import { LandingPage } from '@/pages/landing-page'
import { LoginPage } from '@/pages/login-page'

// Só a landing pública e o login entram no bundle inicial — todas as
// telas autenticadas são code-split (React.lazy) porque um visitante
// anônimo (o caso mais comum em "/") nunca precisa baixar o JS do
// dashboard/usuários/etc. Ver PageFallback pelo estado de carregamento.
const DashboardPage = lazy(() => import('@/pages/dashboard-page').then((m) => ({ default: m.DashboardPage })))
const UsersPage = lazy(() => import('@/pages/users-page').then((m) => ({ default: m.UsersPage })))
const DevicesPage = lazy(() => import('@/pages/devices-page').then((m) => ({ default: m.DevicesPage })))
const SharesPage = lazy(() => import('@/pages/shares-page').then((m) => ({ default: m.SharesPage })))
const WaitlistPage = lazy(() => import('@/pages/waitlist-page').then((m) => ({ default: m.WaitlistPage })))
const DownloadPage = lazy(() => import('@/pages/download-page').then((m) => ({ default: m.DownloadPage })))
const SettingsPage = lazy(() => import('@/pages/settings-page').then((m) => ({ default: m.SettingsPage })))
const AuditPage = lazy(() => import('@/pages/audit-page').then((m) => ({ default: m.AuditPage })))
const PortalPage = lazy(() => import('@/pages/portal-page').then((m) => ({ default: m.PortalPage })))

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Suspense fallback={<PageFallback />}>
          <Routes>
            {/* "/" é a landing pública do produto (ver ROADMAP.md): explica o
                que é o XVPN e recebe cadastros na lista de espera, sem exigir
                login. Quem já está autenticado é mandado direto pro
                dashboard/portal, conforme o papel. */}
            <Route path="/" element={<LandingPage />} />
            <Route path="/login" element={<LoginPage />} />
            {/* Qualquer papel autenticado (inclusive member) — telas comuns a
                todos, sem controles administrativos. */}
            <Route element={<ProtectedRoute />}>
              <Route element={<AppShell />}>
                <Route path="portal" element={<PortalPage />} />
                <Route path="download" element={<DownloadPage />} />
                {/* super_admin/admin/viewer: telas do painel administrativo
                    (leitura garantida a viewer; escrita segue escondida/
                    bloqueada por papel dentro de cada página — ver
                    PLAN.md §6.7). member nunca entra aqui. */}
                <Route element={<ProtectedRoute allowedRoles={VIEWER_UP_ROLES} />}>
                  <Route path="dashboard" element={<DashboardPage />} />
                  <Route path="users" element={<UsersPage />} />
                  <Route path="devices" element={<DevicesPage />} />
                  <Route path="shares" element={<SharesPage />} />
                  <Route path="waitlist" element={<WaitlistPage />} />
                  <Route path="settings" element={<SettingsPage />} />
                  <Route path="audit" element={<AuditPage />} />
                </Route>
              </Route>
            </Route>
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </Suspense>
      </AuthProvider>
    </BrowserRouter>
  )
}
