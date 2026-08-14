import { Navigate, Outlet } from 'react-router-dom'
import { useAuth } from '@/lib/auth-context'
import { defaultRouteForRole, type Role } from '@/lib/roles'
import { PageFallback } from '@/components/layout/page-fallback'

// ProtectedRoute exige sessão válida e, opcionalmente (allowedRoles),
// restringe por papel — ver PLAN.md §6.7. Sem allowedRoles, qualquer papel
// autenticado passa (usado pelas telas comuns a todos, ex.: /portal,
// /download).
export function ProtectedRoute({ allowedRoles }: { allowedRoles?: Role[] }) {
  const { isAuthenticated, user, isLoadingUser } = useAuth()

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }

  // Sem o papel carregado ainda não dá pra saber se allowedRoles autoriza
  // esta rota — evita um flash da tela errada antes de GET /auth/me voltar.
  if (isLoadingUser) {
    return <PageFallback />
  }

  if (allowedRoles && (!user || !allowedRoles.includes(user.role))) {
    return <Navigate to={user ? defaultRouteForRole(user.role) : '/login'} replace />
  }

  return <Outlet />
}
