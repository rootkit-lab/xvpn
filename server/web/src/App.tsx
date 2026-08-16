import { lazy, Suspense, useEffect } from 'react'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { AuthProvider } from '@/lib/auth-context'
import { VIEWER_UP_ROLES } from '@/lib/roles'
import { MARKETPLACE_ORIGIN, PANEL_ORIGIN, XDRIVER_CORP_ORIGIN, productKind } from '@/lib/product-host'
import { ProtectedRoute } from '@/components/layout/protected-route'
import { AdminShell } from '@/components/layout/admin-shell'
import { UserShell } from '@/components/layout/user-shell'
import { SocialShell } from '@/components/layout/social-shell'
import { ChatHost } from '@/components/layout/chat-host'
import { PageFallback } from '@/components/layout/page-fallback'
import { LandingPage } from '@/pages/landing-page'
import { LoginPage } from '@/pages/login-page'
import { SSOLoginRedirect } from '@/pages/sso-login-redirect'
import { HostRedirect } from '@/pages/host-redirect'

const DashboardPage = lazy(() => import('@/pages/dashboard-page').then((m) => ({ default: m.DashboardPage })))
const UsersPage = lazy(() => import('@/pages/users-page').then((m) => ({ default: m.UsersPage })))
const UserDetailPage = lazy(() => import('@/pages/user-detail-page').then((m) => ({ default: m.UserDetailPage })))
const UserCreatePage = lazy(() => import('@/pages/user-create-page').then((m) => ({ default: m.UserCreatePage })))
const DevicesPage = lazy(() => import('@/pages/devices-page').then((m) => ({ default: m.DevicesPage })))
const SharesPage = lazy(() => import('@/pages/shares-page').then((m) => ({ default: m.SharesPage })))
const WaitlistPage = lazy(() => import('@/pages/waitlist-page').then((m) => ({ default: m.WaitlistPage })))
const MarketplacePage = lazy(() => import('@/pages/marketplace-page').then((m) => ({ default: m.MarketplacePage })))
const SettingsPage = lazy(() => import('@/pages/settings-page').then((m) => ({ default: m.SettingsPage })))
const DNSPage = lazy(() => import('@/pages/dns-page').then((m) => ({ default: m.DNSPage })))
const AuditPage = lazy(() => import('@/pages/audit-page').then((m) => ({ default: m.AuditPage })))
const PortalPage = lazy(() => import('@/pages/portal-page').then((m) => ({ default: m.PortalPage })))
const ProfilePage = lazy(() => import('@/pages/profile-page').then((m) => ({ default: m.ProfilePage })))
const AccountPage = lazy(() => import('@/pages/account-page').then((m) => ({ default: m.AccountPage })))
const RbacPage = lazy(() => import('@/pages/rbac-page').then((m) => ({ default: m.RbacPage })))
const XGroupAdminPage = lazy(() => import('@/pages/xgroup-admin-page').then((m) => ({ default: m.XGroupAdminPage })))
const SocialFeedPage = lazy(() => import('@/pages/social-feed-page').then((m) => ({ default: m.SocialFeedPage })))
const SocialDirectoryPage = lazy(() =>
  import('@/pages/social-directory-page').then((m) => ({ default: m.SocialDirectoryPage })),
)
const SocialProfilePage = lazy(() =>
  import('@/pages/social-profile-page').then((m) => ({ default: m.SocialProfilePage })),
)
const SocialMessagesPage = lazy(() =>
  import('@/pages/social-messages-page').then((m) => ({ default: m.SocialMessagesPage })),
)
const SocialGroupsPage = lazy(() =>
  import('@/pages/social-groups-page').then((m) => ({ default: m.SocialGroupsPage })),
)
const PlayStoreLayout = lazy(() => import('@/pages/play-store-page').then((m) => ({ default: m.PlayStoreLayout })))
const PlayStoreHome = lazy(() => import('@/pages/play-store-page').then((m) => ({ default: m.PlayStoreHome })))
const PlayStoreDetail = lazy(() => import('@/pages/play-store-page').then((m) => ({ default: m.PlayStoreDetail })))
const ProductSettingsPage = lazy(() =>
  import('@/pages/product-settings-page').then((m) => ({ default: m.ProductSettingsPage })),
)
const XDriverLayout = lazy(() => import('@/pages/xdriver-app-page').then((m) => ({ default: m.XDriverLayout })))
const XDriverAppPage = lazy(() => import('@/pages/xdriver-app-page').then((m) => ({ default: m.XDriverAppPage })))
const XvpnProductPortal = lazy(() =>
  import('@/pages/xvpn-portal-page').then((m) => ({ default: m.XvpnProductPortal })),
)
const XGroupPublicLanding = lazy(() =>
  import('@/pages/xgroup-landing-page').then((m) => ({ default: m.XGroupPublicLanding })),
)

function XAuthApp() {
  return (
    <Routes>
      <Route path="/" element={<LoginPage variant="sso" />} />
      <Route path="/login" element={<LoginPage variant="sso" />} />
      <Route path="*" element={<XAuthLeave />} />
    </Routes>
  )
}

/** /admin no xauth não existe — mandar ao painel, nunca de volta ao /login (loop). */
function XAuthLeave() {
  useEffect(() => {
    const path = window.location.pathname
    window.location.replace(path.startsWith('/admin') ? `${PANEL_ORIGIN}/admin` : `${PANEL_ORIGIN}/`)
  }, [])
  return <PageFallback />
}

function MarketplaceApp() {
  return (
    <Routes>
      <Route path="/login" element={<SSOLoginRedirect />} />
      <Route element={<ProtectedRoute />}>
        <Route element={<PlayStoreLayout />}>
          <Route index element={<PlayStoreHome />} />
          <Route path="app/:slug" element={<PlayStoreDetail />} />
          <Route path="settings" element={<ProductSettingsPage product="marketplace" />} />
        </Route>
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

function XDriverPublicApp() {
  return null
}

function XDriverCorpApp() {
  return (
    <Routes>
      <Route path="/login" element={<SSOLoginRedirect />} />
      <Route element={<ProtectedRoute />}>
        <Route element={<XDriverLayout />}>
          <Route index element={<XDriverAppPage />} />
          <Route path="settings" element={<ProductSettingsPage product="xdriver" />} />
        </Route>
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

function XGroupPublicApp() {
  return (
    <Routes>
      <Route path="/" element={<XGroupPublicLanding />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

function PanelApp({ home }: { home: 'landing' | 'portal' }) {
  return (
    <ChatHost>
      <Routes>
        <Route path="/" element={home === 'portal' ? <XvpnProductPortal /> : <LandingPage />} />

        <Route path="/my/login" element={<SSOLoginRedirect />} />
        <Route path="/admin/login" element={<SSOLoginRedirect />} />

        <Route element={<ProtectedRoute />}>
          <Route path="/my" element={<UserShell />}>
            <Route index element={home === 'portal' ? <Navigate to="/" replace /> : <PortalPage />} />
            <Route path="devices" element={<PortalPage />} />
            <Route path="files" element={<HostRedirect to={XDRIVER_CORP_ORIGIN} />} />
            <Route path="download" element={<HostRedirect to={MARKETPLACE_ORIGIN} />} />
            <Route path="marketplace" element={<HostRedirect to={MARKETPLACE_ORIGIN} />} />
            <Route path="profile" element={<ProfilePage />} />
            <Route path="account" element={<AccountPage />} />
          </Route>
          <Route path="/social" element={<SocialShell />}>
            <Route index element={<SocialFeedPage />} />
            <Route path="explore" element={<SocialDirectoryPage />} />
            <Route path="u/:username" element={<SocialProfilePage />} />
            <Route path="messages" element={<SocialMessagesPage />} />
            <Route path="groups" element={<SocialGroupsPage />} />
          </Route>
          <Route path="/xgroup" element={<SocialShell />}>
            <Route index element={<SocialFeedPage />} />
            <Route path="explore" element={<SocialDirectoryPage />} />
            <Route path="u/:username" element={<SocialProfilePage />} />
            <Route path="messages" element={<SocialMessagesPage />} />
            <Route path="groups" element={<SocialGroupsPage />} />
          </Route>
          <Route path="/xchat/messages" element={<Navigate to="/social/messages" replace />} />
        </Route>

        <Route element={<ProtectedRoute allowedRoles={VIEWER_UP_ROLES} />}>
          <Route path="/admin" element={<AdminShell />}>
            <Route index element={<DashboardPage />} />
            <Route path="users" element={<UsersPage />} />
            <Route path="users/new" element={<UserCreatePage />} />
            <Route path="users/:id" element={<UserDetailPage />} />
            <Route path="rbac" element={<RbacPage />} />
            <Route path="devices" element={<DevicesPage />} />
            <Route path="shares" element={<SharesPage />} />
            <Route path="waitlist" element={<WaitlistPage />} />
            <Route path="download" element={<HostRedirect to={MARKETPLACE_ORIGIN} />} />
            <Route path="marketplace" element={<MarketplacePage variant="manage" />} />
            <Route path="xgroup" element={<XGroupAdminPage />} />
            <Route path="settings" element={<SettingsPage />} />
            <Route path="dns" element={<DNSPage />} />
            <Route path="audit" element={<AuditPage />} />
          </Route>
        </Route>

        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </ChatHost>
  )
}

export default function App() {
  const kind = productKind()
  return (
    <BrowserRouter>
      <AuthProvider>
        <Suspense fallback={<PageFallback />}>
          {kind === 'xauth' ? (
            <XAuthApp />
          ) : kind === 'marketplace' ? (
            <MarketplaceApp />
          ) : kind === 'xdriver-corp' ? (
            <XDriverCorpApp />
          ) : kind === 'xdriver' ? (
            <XDriverPublicApp />
          ) : kind === 'xgroup' ? (
            <XGroupPublicApp />
          ) : (
            <PanelApp home={kind === 'xvpn' ? 'portal' : 'landing'} />
          )}
        </Suspense>
      </AuthProvider>
    </BrowserRouter>
  )
}
