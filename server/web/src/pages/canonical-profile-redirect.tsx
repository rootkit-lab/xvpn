import { Navigate, useParams } from 'react-router-dom'
import { HostRedirect } from '@/pages/host-redirect'
import { isProfileUsername, profileLocation } from '@/lib/social-profile'

/** `/social/u/:user` e `/:user` no corp/painel → `xgroup.ihuull.com/<user>`. */
export function CanonicalProfileRedirect() {
  const { username } = useParams()
  if (!username || !isProfileUsername(username)) {
    return <Navigate to="/" replace />
  }
  const loc = profileLocation(username)
  if (loc.external) return <HostRedirect to={loc.href} />
  return <Navigate to={loc.href} replace />
}
