import { ExternalLink, ShieldCheck } from 'lucide-react'
import { ProductHeader } from '@xvpn/ui/react/product-header'
import { XCHAT_CORP_ORIGIN } from '@/lib/product-host'
import { Button } from '@/components/ui/button'

export function XChatPublicLanding() {
  return (
    <div data-product="xchat" className="watch-face relative flex min-h-svh flex-col overflow-hidden">
      <div className="watch-vignette pointer-events-none absolute inset-0" aria-hidden="true" />
      <ProductHeader product="xchat" href="/" productHref="/" />

      <main className="relative z-10 mx-auto flex w-full max-w-xl flex-1 flex-col justify-center gap-6 px-6 py-16">
        <p className="hud-label text-muted-foreground/70">Messenger</p>
        <h1 className="font-display text-3xl font-semibold tracking-tight">O chat fica na VPN</h1>
        <p className="text-sm leading-relaxed text-muted-foreground">
          O app abre só em <code className="font-mono text-xs">xchat.corp.ihuull.com</code>, dentro do túnel.
          Este endereço público não serve mensagens, API nem WebSocket — e não tem painel de administração.
        </p>
        <div className="watch-complication flex items-start gap-3 rounded-[18px] p-4">
          <ShieldCheck className="mt-0.5 size-5 shrink-0 text-muted-foreground" />
          <p className="text-sm text-muted-foreground">
            Conecte o cliente XVPN. No Chrome, desligue DNS seguro (DoH) — senão o nome{' '}
            <code className="font-mono text-xs">.corp</code> não resolve pelo DNS da VPN.
          </p>
        </div>
        <Button size="lg" className="self-start rounded-full" asChild>
          <a href={XCHAT_CORP_ORIGIN}>
            <ExternalLink className="size-4" />
            Abrir XCHAT (só com VPN)
          </a>
        </Button>
      </main>
    </div>
  )
}
