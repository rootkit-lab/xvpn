import { useEffect } from 'react'
import { useParams } from 'react-router-dom'
import { PANEL_ORIGIN } from '@/lib/product-host'
import { PageFallback } from '@/components/layout/page-fallback'

/** Manda o browser para o host de produto (origem diferente — não é <Navigate>). */
export function HostRedirect({ to }: { to: string }) {
  useEffect(() => {
    window.location.replace(to)
  }, [to])
  return <PageFallback />
}

/** `/admin` só existe em xvpn.ihuull.com — PLAN.md §6.13. */
export function AdminHostRedirect() {
  const path = `${window.location.pathname}${window.location.search}`
  return <HostRedirect to={`${PANEL_ORIGIN}${path.startsWith('/admin') ? path : '/admin'}`} />
}

export function HostRedirectJoin({ to }: { to: string }) {
  const params = useParams()
  const suffix = Object.values(params)
    .filter((v): v is string => Boolean(v))
    .map((v) => encodeURIComponent(v))
    .join('/')
  const dest = suffix ? `${to.replace(/\/+$/, '')}/${suffix}` : to
  return <HostRedirect to={dest} />
}
