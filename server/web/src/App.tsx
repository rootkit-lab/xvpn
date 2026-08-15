import { lazy, Suspense } from 'react'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { AuthProvider } from '@/lib/auth-context'
import { VIEWER_UP_ROLES } from '@/lib/roles'
import { ProtectedRoute } from '@/components/layout/protected-route'
import { AdminShell } from '@/components/layout/admin-shell'
import { UserShell } from '@/components/layout/user-shell'
import { PageFallback } from '@/components/layout/page-fallback'
import { LandingPage } from '@/pages/landing-page'
import { LoginPage } from '@/pages/login-page'

const DashboardPage = lazy(() => import('@/pages/dashboard-page').then((m) => ({ default: m.DashboardPage })))
const UsersPage = lazy(() => import('@/pages/users-page').then((m) => ({ default: m.UsersPage })))
const DevicesPage = lazy(() => import('@/pages/devices-page').then((m) => ({ default: m.DevicesPage })))
const SharesPage = lazy(() => import('@/pages/shares-page').then((m) => ({ default: m.SharesPage })))
const WaitlistPage = lazy(() => import('@/pages/waitlist-page').then((m) => ({ default: m.WaitlistPage })))
const DownloadPage = lazy(() => import('@/pages/download-page').then((m) => ({ default: m.DownloadPage })))
const MarketplacePage = lazy(() => import('@/pages/marketplace-page').then((m) => ({ default: m.MarketplacePage })))
const SettingsPage = lazy(() => import('@/pages/settings-page').then((m) => ({ default: m.SettingsPage })))
const AuditPage = lazy(() => import('@/pages/audit-page').then((m) => ({ default: m.AuditPage })))
const PortalPage = lazy(() => import('@/pages/portal-page').then((m) => ({ default: m.PortalPage })))

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Suspense fallback={<PageFallback />}>
          <Routes>
            <Route path="/" element={<LandingPage />} />

            {/* Logins com framing distinto (mesmo endpoint de auth). */}
            <Route path="/app/login" element={<LoginPage variant="user" />} />
            <Route path="/admin/login" element={<LoginPage variant="admin" />} />
            <Route path="/login" element={<Navigate to="/app/login" replace />} />

            {/* Painel do usuário — qualquer papel autenticado. */}
            <Route element={<ProtectedRoute />}>
              <Route path="/app" element={<UserShell />}>
                <Route index element={<PortalPage />} />
                <Route path="download" element={<DownloadPage />} />
                <Route path="marketplace" element={<MarketplacePage variant="consume" />} />
              </Route>
            </Route>

            {/* Administração do sistema — viewer+. */}
            <Route element={<ProtectedRoute allowedRoles={VIEWER_UP_ROLES} />}>
              <Route path="/admin" element={<AdminShell />}>
                <Route index element={<DashboardPage />} />
                <Route path="users" element={<UsersPage />} />
                <Route path="devices" element={<DevicesPage />} />
                <Route path="shares" element={<SharesPage />} />
                <Route path="waitlist" element={<WaitlistPage />} />
                <Route path="download" element={<DownloadPage />} />
                <Route path="marketplace" element={<MarketplacePage variant="manage" />} />
                <Route path="settings" element={<SettingsPage />} />
                <Route path="audit" element={<AuditPage />} />
              </Route>
            </Route>

            {/* Aliases legados (bookmarks / docs antigos). */}
            <Route path="/portal" element={<Navigate to="/app" replace />} />
            <Route path="/dashboard" element={<Navigate to="/admin" replace />} />
            <Route path="/users" element={<Navigate to="/admin/users" replace />} />
            <Route path="/devices" element={<Navigate to="/admin/devices" replace />} />
            <Route path="/shares" element={<Navigate to="/admin/shares" replace />} />
            <Route path="/waitlist" element={<Navigate to="/admin/waitlist" replace />} />
            <Route path="/settings" element={<Navigate to="/admin/settings" replace />} />
            <Route path="/audit" element={<Navigate to="/admin/audit" replace />} />
            <Route path="/download" element={<Navigate to="/app/download" replace />} />
            <Route path="/marketplace" element={<Navigate to="/app/marketplace" replace />} />

            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </Suspense>
      </AuthProvider>
    </BrowserRouter>
  )
}
