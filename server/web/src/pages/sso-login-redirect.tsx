import { useEffect } from 'react'
import { ssoLoginURL } from '@/lib/product-host'
import { PageFallback } from '@/components/layout/page-fallback'

/** Hosts que não são o xauth mandam o login para o SSO com return seguro. */
export function SSOLoginRedirect() {
  useEffect(() => {
    window.location.replace(ssoLoginURL())
  }, [])
  return <PageFallback />
}
