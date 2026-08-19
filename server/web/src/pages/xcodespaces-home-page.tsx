import { useCallback } from 'react'
import { toast } from 'sonner'
import { api, ApiError } from '@/lib/api'
import { formatRelativeTime } from '@/lib/format'
import { usePollingData } from '@/hooks/use-polling-data'
import { XGIT_CORP_ORIGIN, codespaceOpenHref } from '@/lib/product-host'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'

export function XcodespacesHomePage() {
  const fetchList = useCallback(() => api.listCodespaces(), [])
  const { data, loading, error, reload } = usePollingData(fetchList, 15_000)

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 p-6 md:p-8">
      <div>
        <p className="hud-label text-muted-foreground/70">XCODESPACES</p>
        <h1 className="font-display text-2xl font-semibold">Your workspaces</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Codespace remoto (VS Code + clone + terminal no container) ou editor rápido (Monaco). Crie a partir do botão Code no repositório.
        </p>
      </div>
      <div className="watch-complication overflow-hidden rounded-[18px]">
        {loading || !data ? (
          error ? <p className="p-5 text-sm text-destructive">{error}</p> : <p className="p-5 text-sm text-muted-foreground">Carregando…</p>
        ) : (data.items ?? []).length === 0 ? (
          <div className="px-4 py-16 text-center">
            <p className="text-sm font-medium">No codespaces</p>
            <p className="mt-1 text-xs text-muted-foreground">
              Abra um repo no XGIT → Code → XCODESPACES → Create codespace.
            </p>
            <Button asChild className="btn-glow mt-4">
              <a href={XGIT_CORP_ORIGIN}>Abrir XGIT</a>
            </Button>
          </div>
        ) : (
          <ul className="divide-y divide-border/60">
            {data.items.map((cs) => (
              <li key={cs.id} className="flex flex-wrap items-center justify-between gap-3 px-4 py-3">
                <div>
                  <a href={codespaceOpenHref(cs)} className="text-sm font-medium hover:underline">
                    {cs.slug}
                  </a>
                  <p className="text-xs text-muted-foreground">
                    {cs.status} · {cs.branch} · {formatRelativeTime(cs.updated_at)}
                    {cs.demo_host ? ` · ${cs.demo_host}:*` : ''}
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  <Badge variant="outline">{cs.status || cs.branch}</Badge>
                  <Button asChild size="sm" className="btn-glow">
                    <a href={codespaceOpenHref(cs)}>Abrir VS Code</a>
                  </Button>
                  {cs.kind === 'remote' ? (
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      onClick={() => {
                        const name = window.prompt(
                          'Nome do preview (vira demo-<nome>.corp.ihuull.com)',
                          cs.demo_name || `cs-${cs.id}`,
                        )
                        if (!name) return
                        void api
                          .patchCodespaceDemo(cs.id, name)
                          .then((out) => {
                            toast.success(out.demo_host ? `${out.demo_host}:*` : 'demo gravado')
                            reload()
                          })
                          .catch((err) => toast.error(err instanceof ApiError ? err.message : 'Falha'))
                      }}
                    >
                      Demo
                    </Button>
                  ) : null}
                  <Button
                    type="button"
                    size="sm"
                    variant="ghost"
                    onClick={() => {
                      void api
                        .deleteCodespace(cs.id)
                        .then(() => {
                          toast.success('Codespace removido')
                          reload()
                        })
                        .catch((err) => toast.error(err instanceof ApiError ? err.message : 'Falha'))
                    }}
                  >
                    Delete
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}
