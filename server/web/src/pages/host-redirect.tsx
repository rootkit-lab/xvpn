import { useEffect } from 'react'
import { PageFallback } from '@/components/layout/page-fallback'

/** Manda o browser para o host de produto (origem diferente — não é <Navigate>). */
export function HostRedirect({ to }: { to: string }) {
  useEffect(() => {
    window.location.replace(to)
  }, [to])
  return <PageFallback />
}
