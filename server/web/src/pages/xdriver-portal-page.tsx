import { ExternalLink, ShieldCheck } from 'lucide-react'
import { ProductHeader } from '@xvpn/ui/react/product-header'
import { XDRIVER_CORP_ORIGIN } from '@/lib/product-host'
import { Button } from '@/components/ui/button'

export function XDriverPublicLanding() {
  return (
    <div data-product="xdriver" className="watch-face relative flex min-h-svh flex-col overflow-hidden">
      <div className="watch-vignette pointer-events-none absolute inset-0" aria-hidden="true" />
      <ProductHeader product="xdriver" href="/" productHref="/" />
      <main className="relative z-10 mx-auto flex w-full max-w-xl flex-1 flex-col justify-center gap-6 px-6 py-16">
        <p className="hud-label text-muted-foreground/70">Drive</p>
        <h1 className="font-display text-3xl font-semibold tracking-tight">Seus arquivos ficam na VPN</h1>
        <p className="text-sm leading-relaxed text-muted-foreground">
          O Drive abre só em <code className="font-mono text-xs">xdriver.corp.ihuull.com</code>, dentro do túnel.
          Este endereço público não lista, envia nem baixa arquivo.
        </p>
        <div className="watch-complication flex items-start gap-3 rounded-[18px] p-4">
          <ShieldCheck className="mt-0.5 size-5 shrink-0 text-muted-foreground" />
          <p className="text-sm text-muted-foreground">
            Conecte o cliente XVPN. No Chrome, desligue DNS seguro (DoH) — senão o nome{' '}
            <code className="font-mono text-xs">.corp</code> não resolve pelo DNS da VPN.
          </p>
        </div>
        <Button size="lg" className="rounded-full self-start" asChild>
          <a href={XDRIVER_CORP_ORIGIN}>
            <ExternalLink className="size-4" />
            Abrir XDRIVER (só com VPN)
          </a>
        </Button>
      </main>
    </div>
  )
}
