import { useCallback, useState } from 'react'
import { useParams } from 'react-router-dom'
import { toast } from 'sonner'
import { api, ApiError } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Skeleton } from '@/components/ui/skeleton'
import { MarkdownDoc } from '@/components/markdown-doc'
import { Badge } from '@/components/ui/badge'

const KINDS = [
  { id: 'deps', label: 'Dependabot' },
  { id: 'code', label: 'Code scanning' },
  { id: 'secret', label: 'Secret scanning' },
] as const

export function XgitSecurityPage() {
  const { org = '', slug = '' } = useParams()
  const repo = `${org}/${slug}`
  const fetchSec = useCallback(() => api.getProjectSecurity(repo), [repo])
  const { data, loading, error, reload } = usePollingData(fetchSec, 20_000)
  const [kind, setKind] = useState<(typeof KINDS)[number]['id']>('deps')
  const [title, setTitle] = useState('')
  const [body, setBody] = useState('')
  const [busy, setBusy] = useState(false)

  if (loading && !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-64 w-full" />
  }
  const alerts = (data?.alerts ?? []).filter((a) => a.kind === kind)
  const setup = data?.setup?.[kind] ?? 'needs_setup'

  async function report() {
    setBusy(true)
    try {
      await api.createSecurityReport(repo, title, body)
      toast.success('Relatório privado aberto')
      setTitle('')
      setBody('')
      reload()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao reportar')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="grid gap-6 md:grid-cols-[14rem_1fr]">
      <aside className="flex flex-col gap-1">
        <p className="text-xs uppercase tracking-wide text-muted-foreground">Findings</p>
        {KINDS.map((k) => (
          <button
            key={k.id}
            type="button"
            onClick={() => setKind(k.id)}
            className={`rounded-md px-2 py-1 text-left text-sm ${kind === k.id ? 'bg-muted font-medium' : 'text-muted-foreground'}`}
          >
            {k.label}
          </button>
        ))}
        <p className="mt-4 text-xs uppercase tracking-wide text-muted-foreground">Reporting</p>
        <p className="text-sm text-muted-foreground">Policy · Advisories · Private report</p>
      </aside>
      <section className="flex min-w-0 flex-col gap-4">
        <div className="flex items-center gap-2">
          <h1 className="text-lg font-semibold">{KINDS.find((k) => k.id === kind)?.label}</h1>
          <Badge variant="outline">{setup === 'enabled' ? 'Enabled' : setup === 'disabled' ? 'Disabled' : 'Needs setup'}</Badge>
        </div>
        {alerts.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            {setup === 'needs_setup'
              ? 'Sem findings. Adicione um workflow npm-audit / govulncheck / gosec em Actions.'
              : 'Nenhum alerta aberto.'}
          </p>
        ) : (
          <ul className="flex flex-col gap-2">
            {alerts.map((a) => (
              <li key={a.id} className="rounded-md border border-border/60 px-3 py-2 text-sm">
                <span className="font-medium">{a.title}</span>
                <span className="ml-2 text-xs text-muted-foreground">
                  {a.tool} · {a.severity}
                </span>
              </li>
            ))}
          </ul>
        )}
        <div className="border-t border-border/60 pt-4">
          <h2 className="mb-2 text-sm font-semibold">Security policy</h2>
          {data?.policy ? (
            <MarkdownDoc text={data.policy} label="SECURITY.md" />
          ) : (
            <p className="text-sm text-muted-foreground">Sem SECURITY.md no default branch.</p>
          )}
        </div>
        {data?.can_report ? (
          <div className="flex flex-col gap-2">
            <h2 className="text-sm font-semibold">Private vulnerability reporting</h2>
            <Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Título" />
            <Textarea value={body} onChange={(e) => setBody(e.target.value)} placeholder="Detalhes (só maintainers)" />
            <Button disabled={busy || !title.trim()} onClick={() => void report()}>
              Abrir issue restrita
            </Button>
          </div>
        ) : null}
      </section>
    </div>
  )
}
