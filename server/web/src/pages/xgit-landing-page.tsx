import { ProductHeader } from '@xvpn/ui/react/product-header'
import { XADMIN_CORP_ORIGIN, XGIT_CORP_ORIGIN } from '@/lib/product-host'

export function XGitLandingPage() {
  return (
    <div data-product="xadmin" className="watch-face relative flex min-h-svh flex-col overflow-hidden">
      <div className="watch-vignette pointer-events-none absolute inset-0" aria-hidden="true" />
      <ProductHeader product="xadmin" href={`${XADMIN_CORP_ORIGIN}/admin/projects`} />
      <main className="relative z-10 mx-auto flex w-full max-w-xl flex-1 flex-col justify-center gap-6 px-6 py-16">
        <p className="hud-label text-muted-foreground/70">Forge</p>
        <h1 className="font-display text-3xl font-semibold tracking-tight">XGIT</h1>
        <p className="text-sm leading-relaxed text-muted-foreground">
          Smart HTTP só na VPN. Usuário e senha da conta ihuull (o mesmo do xadmin). Merge em{' '}
          <code className="font-mono text-xs">main</code> via MR no xadmin. Sem{' '}
          <code className="font-mono text-xs">git://</code> público e sem shell SSH.
        </p>
        <pre className="watch-complication overflow-x-auto rounded-[18px] p-4 font-mono text-xs leading-relaxed">
          {`git clone ${XGIT_CORP_ORIGIN}/<slug>`}
        </pre>
        <a href={`${XADMIN_CORP_ORIGIN}/admin/projects`} className="text-sm text-primary hover:underline">
          Projetos no xadmin
        </a>
      </main>
    </div>
  )
}
