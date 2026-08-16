import { ShieldX } from 'lucide-react'
import { PANEL_ORIGIN } from '@/lib/product-host'
import { Button } from '@/components/ui/button'

export function ForbiddenPage() {
  return (
    <div className="watch-face relative flex min-h-svh items-center justify-center overflow-hidden p-4">
      <div className="watch-vignette pointer-events-none absolute inset-0" aria-hidden="true" />
      <div className="relative z-10 flex max-w-md flex-col items-center gap-4 text-center">
        <span className="icon-well flex size-14 items-center justify-center rounded-[14px]">
          <ShieldX className="size-7 text-destructive" />
        </span>
        <p className="hud-label text-muted-foreground/70">403</p>
        <h1 className="font-display text-2xl font-semibold tracking-tight">Sem permissão neste portal</h1>
        <p className="text-sm leading-relaxed text-muted-foreground">
          Você já está autenticado. Esta área não faz parte do seu acesso — não é um segundo cadastro.
        </p>
        <Button asChild variant="outline">
          <a href={PANEL_ORIGIN}>Voltar ao portal XVPN</a>
        </Button>
      </div>
    </div>
  )
}
