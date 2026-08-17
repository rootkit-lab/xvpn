import { ExternalLink, ShieldCheck } from 'lucide-react'
import { ProductHeader } from '@xvpn/ui/react/product-header'
import { XGROUP_CORP_ORIGIN } from '@/lib/product-host'
import { Button } from '@/components/ui/button'

export function XGroupPublicLanding() {
  return (
    <div data-product="xgroup" className="watch-face relative flex min-h-svh flex-col overflow-hidden">
      <div className="watch-vignette pointer-events-none absolute inset-0" aria-hidden="true" />
      <ProductHeader product="xgroup" href="/" />

      <main className="relative z-10 mx-auto flex w-full max-w-xl flex-1 flex-col justify-center gap-6 px-6 py-16">
        <p className="hud-label text-muted-foreground/70">Social</p>
        <h1 className="font-display text-3xl font-semibold tracking-tight">A rede social fica na VPN</h1>
        <p className="text-sm leading-relaxed text-muted-foreground">
          Perfil amigável: <code className="font-mono text-xs">xgroup.ihuull.com/&lt;user&gt;</code> (com
          login). Feed, explorar e grupos abrem em{' '}
          <code className="font-mono text-xs">xgroup.corp.ihuull.com</code> — só na VPN. Sem WebSocket
          neste host.
        </p>
        <div className="watch-complication flex items-start gap-3 rounded-[18px] p-4">
          <ShieldCheck className="mt-0.5 size-5 shrink-0 text-muted-foreground" />
          <p className="text-sm text-muted-foreground">
            Conecte o cliente XVPN. No Chrome, desligue DNS seguro (DoH) — senão o nome{' '}
            <code className="font-mono text-xs">.corp</code> não resolve pelo DNS da VPN.
          </p>
        </div>
        <Button size="lg" className="self-start rounded-full" asChild>
          <a href={XGROUP_CORP_ORIGIN}>
            <ExternalLink className="size-4" />
            Abrir XGROUP (só com VPN)
          </a>
        </Button>
      </main>
    </div>
  )
}
