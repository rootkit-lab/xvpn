import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { useAuth } from '@/lib/auth-context'
import { defaultRouteForRole, loginPathForLocation, type Role } from '@/lib/roles'
import { PageFallback } from '@/components/layout/page-fallback'

// ProtectedRoute exige sessão válida e, opcionalmente (allowedRoles),
// restringe por papel — ver PLAN.md §6.7. Sem allowedRoles, qualquer papel
// autenticado passa (usado pelo painel do usuário em /app/*).
export function ProtectedRoute({ allowedRoles }: { allowedRoles?: Role[] }) {
  const { isAuthenticated, user, isLoadingUser } = useAuth()
  const location = useLocation()
  const loginPath = loginPathForLocation(location.pathname)

  if (!isAuthenticated) {
    return <Navigate to={loginPath} replace state={{ from: location.pathname }} />
  }

  if (isLoadingUser) {
    return <PageFallback />
  }

  if (allowedRoles && (!user || !allowedRoles.includes(user.role))) {
    return <Navigate to={user ? defaultRouteForRole(user.role) : loginPath} replace />
  }

  return <Outlet />
}
