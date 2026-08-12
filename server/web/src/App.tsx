import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { AuthProvider } from '@/lib/auth-context'
import { ProtectedRoute } from '@/components/layout/protected-route'
import { AppShell } from '@/components/layout/app-shell'
import { LoginPage } from '@/pages/login-page'
import { DashboardPage } from '@/pages/dashboard-page'
import { UsersPage } from '@/pages/users-page'
import { DevicesPage } from '@/pages/devices-page'
import { SharesPage } from '@/pages/shares-page'
import { SettingsPage } from '@/pages/settings-page'
import { AuditPage } from '@/pages/audit-page'

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route element={<ProtectedRoute />}>
            <Route element={<AppShell />}>
              <Route index element={<DashboardPage />} />
              <Route path="users" element={<UsersPage />} />
              <Route path="devices" element={<DevicesPage />} />
              <Route path="shares" element={<SharesPage />} />
              <Route path="settings" element={<SettingsPage />} />
              <Route path="audit" element={<AuditPage />} />
            </Route>
          </Route>
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  )
}
