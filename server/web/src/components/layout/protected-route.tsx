import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { useAuth } from '@/lib/auth-context'
import { loginPathForLocation, type Role } from '@/lib/roles'
import { productKind, ssoLoginURL } from '@/lib/product-host'
import { PageFallback } from '@/components/layout/page-fallback'
import { ForbiddenPage } from '@/pages/forbidden-page'

// ProtectedRoute exige sessão válida e, opcionalmente (allowedRoles),
// restringe por papel — ver PLAN.md §6.7. Sem allowedRoles, qualquer papel
// autenticado passa (usado pelo painel do usuário em /my/*).
// Sem sessão: SSO no xauth. Com sessão sem permissão: 403, não outro login.
export function ProtectedRoute({ allowedRoles }: { allowedRoles?: Role[] }) {
  const { isAuthenticated, user, isLoadingUser } = useAuth()
  const location = useLocation()
  const loginPath = loginPathForLocation(location.pathname)

  if (isLoadingUser) {
    return <PageFallback />
  }

  if (!isAuthenticated) {
    if (productKind() === 'xauth') {
      return <Navigate to={loginPath} replace state={{ from: location.pathname }} />
    }
    window.location.replace(ssoLoginURL())
    return <PageFallback />
  }

  if (allowedRoles && (!user || !allowedRoles.includes(user.role))) {
    return <ForbiddenPage />
  }

  return <Outlet />
}
