import { lazy, Suspense, useEffect } from 'react'
import { BrowserRouter, Navigate, Outlet, Route, Routes, useLocation, useParams } from 'react-router-dom'
import { AuthProvider, useAuth } from '@/lib/auth-context'
import { VIEWER_UP_ROLES } from '@/lib/roles'
import {
  MARKETPLACE_ORIGIN,
  PANEL_ORIGIN,
  XADMIN_CORP_ORIGIN,
  XCHAT_CORP_ORIGIN,
  XDRIVER_CORP_ORIGIN,
  XGIT_CORP_ORIGIN,
  XGROUP_CORP_ORIGIN,
  productKind,
} from '@/lib/product-host'
import { ProtectedRoute } from '@/components/layout/protected-route'
import { AdminShell } from '@/components/layout/admin-shell'
import { XgitShell } from '@/components/layout/xgit-shell'
import { XcodespacesShell } from '@/components/layout/xcodespaces-shell'
import { UserShell } from '@/components/layout/user-shell'
import { PublicProfileShell, SocialShell } from '@/components/layout/social-shell'
import { ChatHost } from '@/components/layout/chat-host'
import { DocumentTitle, DocumentTitleProvider } from '@/components/layout/document-title'
import { PageFallback } from '@/components/layout/page-fallback'
import { LandingPage } from '@/pages/landing-page'
import { LoginPage } from '@/pages/login-page'
import { SSOLoginRedirect } from '@/pages/sso-login-redirect'
import { AdminHostRedirect, HostRedirect } from '@/pages/host-redirect'
import { CanonicalProfileRedirect } from '@/pages/canonical-profile-redirect'

const DashboardPage = lazy(() => import('@/pages/dashboard-page').then((m) => ({ default: m.DashboardPage })))
const UsersPage = lazy(() => import('@/pages/users-page').then((m) => ({ default: m.UsersPage })))
const UserDetailPage = lazy(() => import('@/pages/user-detail-page').then((m) => ({ default: m.UserDetailPage })))
const UserCreatePage = lazy(() => import('@/pages/user-create-page').then((m) => ({ default: m.UserCreatePage })))
const DevicesPage = lazy(() => import('@/pages/devices-page').then((m) => ({ default: m.DevicesPage })))
const SharesPage = lazy(() => import('@/pages/shares-page').then((m) => ({ default: m.SharesPage })))
const WaitlistPage = lazy(() => import('@/pages/waitlist-page').then((m) => ({ default: m.WaitlistPage })))
const MarketplacePage = lazy(() => import('@/pages/marketplace-page').then((m) => ({ default: m.MarketplacePage })))
const SettingsPage = lazy(() => import('@/pages/settings-page').then((m) => ({ default: m.SettingsPage })))
const BackupsPage = lazy(() => import('@/pages/backups-page').then((m) => ({ default: m.BackupsPage })))
const DNSPage = lazy(() => import('@/pages/dns-page').then((m) => ({ default: m.DNSPage })))
const PublicDNSPage = lazy(() => import('@/pages/public-dns-page').then((m) => ({ default: m.PublicDNSPage })))
const PublicDNSZonePage = lazy(() =>
  import('@/pages/public-dns-zone-page').then((m) => ({ default: m.PublicDNSZonePage })),
)
const DNSSettingsPage = lazy(() => import('@/pages/dns-settings-page').then((m) => ({ default: m.DNSSettingsPage })))
const AuditPage = lazy(() => import('@/pages/audit-page').then((m) => ({ default: m.AuditPage })))
const PortalPage = lazy(() => import('@/pages/portal-page').then((m) => ({ default: m.PortalPage })))
const ProfilePage = lazy(() => import('@/pages/profile-page').then((m) => ({ default: m.ProfilePage })))
const AccountPage = lazy(() => import('@/pages/account-page').then((m) => ({ default: m.AccountPage })))
const RbacPage = lazy(() => import('@/pages/rbac-page').then((m) => ({ default: m.RbacPage })))
const XGroupAdminPage = lazy(() => import('@/pages/xgroup-admin-page').then((m) => ({ default: m.XGroupAdminPage })))
const XgitReposPage = lazy(() => import('@/pages/xgit-repos-page').then((m) => ({ default: m.XgitReposPage })))
const XgitHomeLayout = lazy(() => import('@/pages/xgit-home-layout').then((m) => ({ default: m.XgitHomeLayout })))
const XgitOverviewPage = lazy(() =>
  import('@/pages/xgit-overview-page').then((m) => ({ default: m.XgitOverviewPage })),
)
const XgitPackagesPage = lazy(() =>
  import('@/pages/xgit-packages-page').then((m) => ({ default: m.XgitPackagesPage })),
)
const XgitRepoPackagesPage = lazy(() =>
  import('@/pages/xgit-packages-page').then((m) => ({ default: m.XgitRepoPackagesPage })),
)
const XgitStarsPage = lazy(() => import('@/pages/xgit-stars-page').then((m) => ({ default: m.XgitStarsPage })))
const XgitOrgPage = lazy(() => import('@/pages/xgit-org-page').then((m) => ({ default: m.XgitOrgPage })))
const XgitSettingsPage = lazy(() =>
  import('@/pages/xgit-settings-page').then((m) => ({ default: m.XgitSettingsPage })),
)
const XgitRepoLayout = lazy(() => import('@/pages/xgit-repo-page').then((m) => ({ default: m.XgitRepoLayout })))
const XgitCodePage = lazy(() => import('@/pages/xgit-repo-page').then((m) => ({ default: m.XgitCodePage })))
const XgitCommitsPage = lazy(() => import('@/pages/xgit-repo-page').then((m) => ({ default: m.XgitCommitsPage })))
const XgitMrsPage = lazy(() => import('@/pages/xgit-repo-page').then((m) => ({ default: m.XgitMrsPage })))
const XgitIssuesPage = lazy(() => import('@/pages/xgit-repo-page').then((m) => ({ default: m.XgitIssuesPage })))
const XgitIssueNewPage = lazy(() => import('@/pages/xgit-repo-page').then((m) => ({ default: m.XgitIssueNewPage })))
const XgitIssuePage = lazy(() => import('@/pages/xgit-repo-page').then((m) => ({ default: m.XgitIssuePage })))
const XgitMilestonesPage = lazy(() => import('@/pages/xgit-repo-page').then((m) => ({ default: m.XgitMilestonesPage })))
const XgitLabelsPage = lazy(() => import('@/pages/xgit-repo-page').then((m) => ({ default: m.XgitLabelsPage })))
const XgitProjectsPage = lazy(() => import('@/pages/xgit-repo-page').then((m) => ({ default: m.XgitProjectsPage })))
const XgitProjectBoardPage = lazy(() =>
  import('@/pages/xgit-repo-page').then((m) => ({ default: m.XgitProjectBoardPage })),
)
const XgitPullsPage = lazy(() => import('@/pages/xgit-repo-page').then((m) => ({ default: m.XgitPullsPage })))
const XgitEditPage = lazy(() => import('@/pages/xgit-edit-page').then((m) => ({ default: m.XgitEditPage })))
const XgitActionsPage = lazy(() => import('@/pages/xgit-actions-page').then((m) => ({ default: m.XgitActionsPage })))
const XgitWikiPage = lazy(() => import('@/pages/xgit-wiki-page').then((m) => ({ default: m.XgitWikiPage })))
const XgitSecurityPage = lazy(() => import('@/pages/xgit-security-page').then((m) => ({ default: m.XgitSecurityPage })))
const XgitAgentsPage = lazy(() => import('@/pages/xgit-agents-page').then((m) => ({ default: m.XgitAgentsPage })))
const XgitActionsNewPage = lazy(() =>
  import('@/pages/xgit-actions-new-page').then((m) => ({ default: m.XgitActionsNewPage })),
)
const XgitRepoSettingsPage = lazy(() =>
  import('@/pages/xgit-repo-page').then((m) => ({ default: m.XgitRepoSettingsPage })),
)
const XcodespacesHomePage = lazy(() =>
  import('@/pages/xcodespaces-home-page').then((m) => ({ default: m.XcodespacesHomePage })),
)
const XcodespacesIdePage = lazy(() =>
  import('@/pages/xcodespaces-ide-page').then((m) => ({ default: m.XcodespacesIdePage })),
)
const XmonitorPage = lazy(() => import('@/pages/xmonitor-page').then((m) => ({ default: m.XmonitorPage })))
const MergeRequestPage = lazy(() =>
  import('@/pages/merge-request-page').then((m) => ({ default: m.MergeRequestPage })),
)
const CiJobPage = lazy(() => import('@/pages/ci-job-page').then((m) => ({ default: m.CiJobPage })))
const NetworksPage = lazy(() => import('@/pages/networks-page').then((m) => ({ default: m.NetworksPage })))
const ServersPage = lazy(() => import('@/pages/servers-page').then((m) => ({ default: m.ServersPage })))
const ServerDetailPage = lazy(() =>
  import('@/pages/server-detail-page').then((m) => ({ default: m.ServerDetailPage })),
)
const ComputeSettingsPage = lazy(() =>
  import('@/pages/compute-settings-page').then((m) => ({ default: m.ComputeSettingsPage })),
)
const ServicesPage = lazy(() => import('@/pages/services-page').then((m) => ({ default: m.ServicesPage })))
const ServiceDetailPage = lazy(() =>
  import('@/pages/service-detail-page').then((m) => ({ default: m.ServiceDetailPage })),
)
const SocialFeedPage = lazy(() => import('@/pages/social-feed-page').then((m) => ({ default: m.SocialFeedPage })))
const SocialDirectoryPage = lazy(() =>
  import('@/pages/social-directory-page').then((m) => ({ default: m.SocialDirectoryPage })),
)
const SocialProfileGate = lazy(() =>
  import('@/pages/social-profile-page').then((m) => ({ default: m.SocialProfileGate })),
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
const XDriverEditPage = lazy(() => import('@/pages/xdriver-edit-page').then((m) => ({ default: m.XDriverEditPage })))
const XDriverViewPage = lazy(() => import('@/pages/xdriver-view-page').then((m) => ({ default: m.XDriverViewPage })))
const XvpnProductPortal = lazy(() =>
  import('@/pages/xvpn-portal-page').then((m) => ({ default: m.XvpnProductPortal })),
)
const XGroupPublicLanding = lazy(() =>
  import('@/pages/xgroup-landing-page').then((m) => ({ default: m.XGroupPublicLanding })),
)
const XChatPublicLanding = lazy(() =>
  import('@/pages/xchat-landing-page').then((m) => ({ default: m.XChatPublicLanding })),
)
const CorpHubPage = lazy(() => import('@/pages/corp-hub-page').then((m) => ({ default: m.CorpHubPage })))

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
    window.location.replace(path.startsWith('/admin') ? `${XADMIN_CORP_ORIGIN}/admin` : `${PANEL_ORIGIN}/`)
  }, [])
  return <PageFallback />
}

function MarketplaceApp() {
  return (
    <Routes>
      <Route path="/login" element={<SSOLoginRedirect />} />
      <Route path="/admin" element={<AdminHostRedirect />} />
      <Route path="/admin/*" element={<AdminHostRedirect />} />
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
      <Route path="/admin" element={<AdminHostRedirect />} />
      <Route path="/admin/*" element={<AdminHostRedirect />} />
      <Route element={<ProtectedRoute />}>
        <Route element={<XDriverLayout />}>
          <Route index element={<XDriverAppPage />} />
          <Route path="edit" element={<XDriverEditPage />} />
          <Route path="view" element={<XDriverViewPage />} />
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
      <Route path="/login" element={<SSOLoginRedirect />} />
      <Route path="/admin" element={<AdminHostRedirect />} />
      <Route path="/admin/*" element={<AdminHostRedirect />} />
      <Route path="/social/u/:username" element={<CanonicalProfileRedirect />} />
      <Route path="/social/*" element={<HostRedirect to={`${XGROUP_CORP_ORIGIN}/social`} />} />
      <Route element={<ProtectedRoute />}>
        <Route element={<PublicProfileShell />}>
          <Route path=":username" element={<SocialProfileGate />} />
        </Route>
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

function XChatPublicApp() {
  return (
    <Routes>
      <Route path="/" element={<XChatPublicLanding />} />
      <Route path="/admin" element={<AdminHostRedirect />} />
      <Route path="/admin/*" element={<AdminHostRedirect />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

function XChatCorpApp() {
  return (
    <ChatHost>
      <Routes>
        <Route path="/login" element={<SSOLoginRedirect />} />
        <Route path="/" element={<Navigate to="/social/messages" replace />} />
        <Route element={<ProtectedRoute />}>
          <Route path="/social" element={<SocialShell />}>
            <Route index element={<HostRedirect to={`${XGROUP_CORP_ORIGIN}/social`} />} />
            <Route path="explore" element={<HostRedirect to={`${XGROUP_CORP_ORIGIN}/social/explore`} />} />
            <Route path="messages" element={<SocialMessagesPage />} />
            <Route path="groups" element={<HostRedirect to={`${XGROUP_CORP_ORIGIN}/social/groups`} />} />
            <Route path="u/:username" element={<CanonicalProfileRedirect />} />
          </Route>
        </Route>
        <Route path="/admin" element={<AdminHostRedirect />} />
        <Route path="/admin/*" element={<AdminHostRedirect />} />
        <Route path="*" element={<Navigate to="/social/messages" replace />} />
      </Routes>
    </ChatHost>
  )
}

function XGroupCorpApp() {
  return (
    <ChatHost>
      <Routes>
        <Route path="/login" element={<SSOLoginRedirect />} />
        <Route path="/" element={<Navigate to="/social" replace />} />
        <Route path="/social/u/:username" element={<CanonicalProfileRedirect />} />
        <Route path="/:username" element={<CanonicalProfileRedirect />} />
        <Route element={<ProtectedRoute />}>
          <Route path="/social" element={<SocialShell />}>
            <Route index element={<SocialFeedPage />} />
            <Route path="explore" element={<SocialDirectoryPage />} />
            <Route path="messages" element={<HostRedirect to={`${XCHAT_CORP_ORIGIN}/social/messages`} />} />
            <Route path="groups" element={<SocialGroupsPage />} />
          </Route>
        </Route>
        <Route path="/admin" element={<AdminHostRedirect />} />
        <Route path="/admin/*" element={<AdminHostRedirect />} />
        <Route path="*" element={<Navigate to="/social" replace />} />
      </Routes>
    </ChatHost>
  )
}

function XGitCorpApp() {
  return (
    <ChatHost>
      <Routes>
        <Route path="/login" element={<SSOLoginRedirect />} />
        <Route path="/admin" element={<AdminHostRedirect />} />
        <Route path="/admin/*" element={<AdminHostRedirect />} />
        <Route element={<ProtectedRoute />}>
          <Route element={<XgitShell />}>
            <Route element={<XgitHomeLayout />}>
              <Route index element={<XgitOverviewPage />} />
              <Route path="repositories" element={<XgitReposPage />} />
              <Route path="packages" element={<XgitPackagesPage />} />
              <Route path="stars" element={<XgitStarsPage />} />
            </Route>
            <Route path=":org" element={<XgitOrgPage />} />
            <Route path=":org/:slug" element={<XgitRepoLayout />}>
              <Route index element={<XgitCodePage />} />
              <Route path="tree/*" element={<XgitCodePage />} />
              <Route path="blob/*" element={<XgitCodePage />} />
              <Route path="edit/:ref/*" element={<XgitEditPage />} />
              <Route path="commits" element={<XgitCommitsPage />} />
              <Route path="issues" element={<XgitIssuesPage />} />
              <Route path="issues/new" element={<XgitIssueNewPage />} />
              <Route path="issues/:n" element={<XgitIssuePage />} />
              <Route path="milestones" element={<XgitMilestonesPage />} />
              <Route path="labels" element={<XgitLabelsPage />} />
              <Route path="projects" element={<XgitProjectsPage />} />
              <Route path="projects/:n" element={<XgitProjectBoardPage />} />
              <Route path="pulls" element={<XgitPullsPage />} />
              <Route path="pulls/:iid" element={<MergeRequestPage />} />
              <Route path="agents" element={<XgitAgentsPage />} />
              <Route path="mrs" element={<XgitMrsPage />} />
              <Route path="mrs/:iid" element={<MergeRequestPage />} />
              <Route path="packages" element={<XgitRepoPackagesPage />} />
              <Route path="wiki" element={<XgitWikiPage />} />
              <Route path="wiki/:page" element={<XgitWikiPage />} />
              <Route path="security" element={<XgitSecurityPage />} />
              <Route path="actions" element={<XgitActionsPage />} />
              <Route path="actions/new" element={<XgitActionsNewPage />} />
              <Route path="actions/:n" element={<CiJobPage />} />
              <Route path="settings" element={<XgitRepoSettingsPage />} />
            </Route>
          </Route>
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </ChatHost>
  )
}

function XcodespacesCorpApp() {
  return (
    <ChatHost>
      <Routes>
        <Route path="/login" element={<SSOLoginRedirect />} />
        <Route path="/admin" element={<AdminHostRedirect />} />
        <Route path="/admin/*" element={<AdminHostRedirect />} />
        <Route element={<ProtectedRoute />}>
          <Route element={<XcodespacesShell />}>
            <Route index element={<XcodespacesHomePage />} />
            <Route path=":id" element={<XcodespacesIdePage />} />
          </Route>
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </ChatHost>
  )
}

function XmonitorCorpApp() {
  return (
    <Routes>
      <Route path="/login" element={<SSOLoginRedirect />} />
      <Route path="/admin" element={<AdminHostRedirect />} />
      <Route path="/admin/*" element={<AdminHostRedirect />} />
      <Route element={<ProtectedRoute allowedRoles={VIEWER_UP_ROLES} />}>
        <Route index element={<XmonitorPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

/** Membro usa o app em xgit.corp — o console lista todos os repos. */
function MemberLeaveXadminXgit() {
  const { user } = useAuth()
  const { pathname } = useLocation()
  if (user?.role === 'member') {
    if (pathname === '/admin/xgit/settings' || pathname.startsWith('/admin/xgit/settings/')) {
      return <HostRedirect to={XGIT_CORP_ORIGIN} />
    }
    const rest = pathname.replace(/^\/admin\/xgit\/?/, '')
    return <HostRedirect to={rest ? `${XGIT_CORP_ORIGIN}/${rest}` : XGIT_CORP_ORIGIN} />
  }
  return <Outlet />
}

function CorpHubApp() {
  return (
    <Routes>
      <Route path="/login" element={<SSOLoginRedirect />} />
      <Route element={<ProtectedRoute />}>
        <Route index element={<CorpHubPage />} />
      </Route>
      <Route path="/admin" element={<AdminHostRedirect />} />
      <Route path="/admin/*" element={<AdminHostRedirect />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

function ProjectToXgit() {
  const { org = '', slug = '', iid, n } = useParams()
  const { pathname } = useLocation()
  const base = `/admin/xgit/${org}/${slug}`
  if (iid) return <Navigate to={`${base}/mrs/${iid}`} replace />
  if (n && pathname.includes('/jobs/')) return <Navigate to={`${base}/actions/${n}`} replace />
  return <Navigate to={base} replace />
}

function AdminIndex() {
  const { user } = useAuth()
  if (user?.role === 'member') {
    return <HostRedirect to={user.xgit_enabled ? XGIT_CORP_ORIGIN : PANEL_ORIGIN} />
  }
  return <DashboardPage />
}

function XAdminCorpApp() {
  return (
    <ChatHost>
      <Routes>
        <Route path="/" element={<Navigate to="/admin" replace />} />
        <Route path="/login" element={<SSOLoginRedirect />} />
        <Route path="/admin/login" element={<SSOLoginRedirect />} />

        <Route element={<ProtectedRoute />}>
          <Route path="/admin" element={<AdminShell />}>
            <Route index element={<AdminIndex />} />
            <Route element={<ProtectedRoute allowedRoles={VIEWER_UP_ROLES} />}>
              <Route path="users" element={<UsersPage />} />
              <Route path="users/new" element={<UserCreatePage />} />
              <Route path="users/:id" element={<UserDetailPage />} />
              <Route path="rbac" element={<RbacPage />} />
              <Route path="devices" element={<DevicesPage />} />
              <Route path="networks" element={<NetworksPage />} />
              <Route path="shares" element={<SharesPage />} />
              <Route path="waitlist" element={<WaitlistPage />} />
              <Route path="download" element={<HostRedirect to={MARKETPLACE_ORIGIN} />} />
              <Route path="marketplace" element={<Navigate to="/admin/marketplace/catalog" replace />} />
              <Route path="marketplace/catalog" element={<MarketplacePage variant="manage" section="catalog" />} />
              <Route path="marketplace/acl" element={<MarketplacePage variant="manage" section="acl" />} />
              <Route path="xgroup" element={<XGroupAdminPage />} />
              <Route path="servers" element={<ServersPage />} />
              <Route path="servers/:id" element={<ServerDetailPage />} />
              <Route path="compute/settings" element={<ComputeSettingsPage />} />
              <Route path="services" element={<ServicesPage />} />
              <Route path="services/:slug" element={<ServiceDetailPage />} />
              <Route path="settings" element={<SettingsPage />} />
              <Route path="backups" element={<BackupsPage />} />
              <Route path="dns/settings" element={<DNSSettingsPage />} />
              <Route path="dns/public/:id" element={<PublicDNSZonePage />} />
              <Route path="dns/public" element={<PublicDNSPage />} />
              <Route path="dns" element={<DNSPage />} />
              <Route path="audit" element={<AuditPage />} />
            </Route>
            <Route path="projects" element={<Navigate to="/admin/xgit" replace />} />
            <Route path="projects/:org/:slug" element={<ProjectToXgit />} />
            <Route path="projects/:org/:slug/mrs/:iid" element={<ProjectToXgit />} />
            <Route path="projects/:org/:slug/jobs/:n" element={<ProjectToXgit />} />
            <Route element={<MemberLeaveXadminXgit />}>
              <Route path="xgit" element={<XgitReposPage />} />
              <Route path="xgit/settings" element={<XgitSettingsPage />} />
              <Route path="xgit/:org" element={<XgitOrgPage />} />
              <Route path="xgit/:org/:slug" element={<XgitRepoLayout />}>
                <Route index element={<XgitCodePage />} />
                <Route path="tree/*" element={<XgitCodePage />} />
                <Route path="blob/*" element={<XgitCodePage />} />
                <Route path="edit/:ref/*" element={<XgitEditPage />} />
                <Route path="commits" element={<XgitCommitsPage />} />
                <Route path="issues" element={<XgitIssuesPage />} />
                <Route path="issues/new" element={<XgitIssueNewPage />} />
                <Route path="issues/:n" element={<XgitIssuePage />} />
                <Route path="milestones" element={<XgitMilestonesPage />} />
                <Route path="labels" element={<XgitLabelsPage />} />
                <Route path="projects" element={<XgitProjectsPage />} />
                <Route path="projects/:n" element={<XgitProjectBoardPage />} />
                <Route path="pulls" element={<XgitPullsPage />} />
                <Route path="pulls/:iid" element={<MergeRequestPage />} />
                <Route path="agents" element={<XgitAgentsPage />} />
                <Route path="mrs" element={<XgitMrsPage />} />
                <Route path="mrs/:iid" element={<MergeRequestPage />} />
                <Route path="packages" element={<XgitRepoPackagesPage />} />
                <Route path="wiki" element={<XgitWikiPage />} />
                <Route path="wiki/:page" element={<XgitWikiPage />} />
                <Route path="security" element={<XgitSecurityPage />} />
                <Route path="actions" element={<XgitActionsPage />} />
                <Route path="actions/new" element={<XgitActionsNewPage />} />
                <Route path="actions/:n" element={<CiJobPage />} />
                <Route path="settings" element={<XgitRepoSettingsPage />} />
              </Route>
            </Route>
          </Route>
        </Route>

        <Route path="*" element={<Navigate to="/admin" replace />} />
      </Routes>
    </ChatHost>
  )
}

function BrandLandingApp() {
  return (
    <Routes>
      <Route path="/" element={<LandingPage />} />
      <Route path="/my/login" element={<SSOLoginRedirect />} />
      <Route path="/admin/login" element={<SSOLoginRedirect />} />
      <Route path="/admin" element={<AdminHostRedirect />} />
      <Route path="/admin/*" element={<AdminHostRedirect />} />
      <Route path="/my/*" element={<HostRedirect to={`${PANEL_ORIGIN}/`} />} />
      <Route path="/social/*" element={<HostRedirect to={`${XGROUP_CORP_ORIGIN}/social`} />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

function PanelApp() {
  return (
    <ChatHost>
      <Routes>
        <Route path="/" element={<XvpnProductPortal />} />

        <Route path="/my/login" element={<SSOLoginRedirect />} />
        <Route path="/admin/login" element={<SSOLoginRedirect />} />
        <Route path="/social/u/:username" element={<CanonicalProfileRedirect />} />
        <Route path="/xgroup/u/:username" element={<CanonicalProfileRedirect />} />

        <Route element={<ProtectedRoute />}>
          <Route path="/my" element={<UserShell />}>
            <Route index element={<Navigate to="/" replace />} />
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
            <Route path="messages" element={<SocialMessagesPage />} />
            <Route path="groups" element={<SocialGroupsPage />} />
          </Route>
          <Route path="/xgroup" element={<SocialShell />}>
            <Route index element={<SocialFeedPage />} />
            <Route path="explore" element={<SocialDirectoryPage />} />
            <Route path="messages" element={<SocialMessagesPage />} />
            <Route path="groups" element={<SocialGroupsPage />} />
          </Route>
          <Route path="/xchat/messages" element={<Navigate to="/social/messages" replace />} />
        </Route>

        <Route path="/admin" element={<AdminHostRedirect />} />
        <Route path="/admin/*" element={<AdminHostRedirect />} />

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
        <DocumentTitleProvider>
          <DocumentTitle />
          <Suspense fallback={<PageFallback />}>
            {kind === 'xauth' ? (
              <XAuthApp />
            ) : kind === 'marketplace' ? (
              <MarketplaceApp />
            ) : kind === 'xdriver-corp' ? (
              <XDriverCorpApp />
            ) : kind === 'xdriver' ? (
              <XDriverPublicApp />
            ) : kind === 'xchat' ? (
              <XChatPublicApp />
            ) : kind === 'xchat-corp' ? (
              <XChatCorpApp />
            ) : kind === 'xgroup' ? (
              <XGroupPublicApp />
            ) : kind === 'xgroup-corp' ? (
              <XGroupCorpApp />
            ) : kind === 'corp' ? (
              <CorpHubApp />
            ) : kind === 'xadmin-corp' ? (
              <XAdminCorpApp />
            ) : kind === 'xgit-corp' ? (
              <XGitCorpApp />
            ) : kind === 'xcodespaces-corp' ? (
              <XcodespacesCorpApp />
            ) : kind === 'xmonitor-corp' ? (
              <XmonitorCorpApp />
            ) : kind === 'xvpn' ? (
              <PanelApp />
            ) : (
              <BrandLandingApp />
            )}
          </Suspense>
        </DocumentTitleProvider>
      </AuthProvider>
    </BrowserRouter>
  )
}
