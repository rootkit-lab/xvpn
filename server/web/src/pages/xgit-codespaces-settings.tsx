import { type FormEvent, useCallback, useEffect, useState } from 'react'
import { toast } from 'sonner'
import { api, ApiError, type ProjectCodespaceEnv, type ProjectRole } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { canWriteAdminProduct, isAdminRole } from '@/lib/roles'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Checkbox } from '@/components/ui/checkbox'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

const ROLE_RANK: Record<ProjectRole, number> = {
  guest: 0,
  reporter: 1,
  developer: 2,
  maintainer: 3,
  owner: 4,
}

type DraftEnv = {
  name: string
  value: string
  secret: boolean
  has_value: boolean
}

export function CodespacesEnvCard({
  slug,
  myRole,
}: {
  slug: string
  myRole?: ProjectRole
}) {
  const { user } = useAuth()
  const canWrite =
    (isAdminRole(user?.role) && canWriteAdminProduct(user?.role, user?.products, 'forge')) ||
    (myRole != null && ROLE_RANK[myRole] >= ROLE_RANK.maintainer)
  const fetchEnvs = useCallback(() => api.getProjectCodespaceEnvs(slug), [slug])
  const { data, reload } = usePollingData(fetchEnvs, 20_000)
  const [rows, setRows] = useState<DraftEnv[]>([])
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!data) return
    setRows(
      data.items.map((it) => ({
        name: it.name,
        value: it.value ?? '',
        secret: it.secret,
        has_value: it.has_value,
      })),
    )
  }, [data])

  function addRow() {
    setRows((cur) => [...cur, { name: '', value: '', secret: false, has_value: false }])
  }

  async function save(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      await api.putProjectCodespaceEnvs(
        slug,
        rows
          .filter((r) => r.name.trim())
          .map((r) => ({ name: r.name.trim(), value: r.value, secret: r.secret })),
      )
      toast.success('ENVs do codespace salvos')
      await reload()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao salvar ENVs')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Codespaces</CardTitle>
        <CardDescription>
          ENVs injetados no <code className="text-xs">docker run</code> no Create. Secrets de LLM (
          <code className="text-xs">XCS_LLM_*</code>) o proxy lê no servidor — a key não entra no container.
          Mudança de ENV exige Recreate do codespace.
        </CardDescription>
      </CardHeader>
      <CardContent>
        {canWrite ? (
          <form className="flex flex-col gap-4" onSubmit={save}>
            {rows.map((row, i) => (
              <div key={i} className="grid gap-2 sm:grid-cols-[1fr_1fr_auto_auto] sm:items-end">
                <div className="grid gap-1">
                  <Label htmlFor={`env-name-${i}`}>Nome</Label>
                  <Input
                    id={`env-name-${i}`}
                    value={row.name}
                    spellCheck={false}
                    placeholder="APP_URL"
                    onChange={(ev) =>
                      setRows((cur) => cur.map((r, j) => (j === i ? { ...r, name: ev.target.value } : r)))
                    }
                  />
                </div>
                <div className="grid gap-1">
                  <Label htmlFor={`env-val-${i}`}>Valor</Label>
                  <Input
                    id={`env-val-${i}`}
                    type={row.secret ? 'password' : 'text'}
                    value={row.value}
                    spellCheck={false}
                    placeholder={row.secret && row.has_value ? '•••• (vazio mantém)' : ''}
                    onChange={(ev) =>
                      setRows((cur) => cur.map((r, j) => (j === i ? { ...r, value: ev.target.value } : r)))
                    }
                  />
                </div>
                <label className="flex items-center gap-2 pb-2 text-sm">
                  <Checkbox
                    checked={row.secret}
                    onCheckedChange={(v) =>
                      setRows((cur) => cur.map((r, j) => (j === i ? { ...r, secret: v === true } : r)))
                    }
                  />
                  Secret
                </label>
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() => setRows((cur) => cur.filter((_, j) => j !== i))}
                >
                  Remover
                </Button>
              </div>
            ))}
            <div className="flex gap-2">
              <Button type="button" variant="outline" onClick={addRow} disabled={rows.length >= 32}>
                Adicionar
              </Button>
              <Button type="submit" disabled={busy}>
                Salvar
              </Button>
            </div>
          </form>
        ) : (
          <EnvReadOnly items={data?.items ?? []} />
        )}
      </CardContent>
    </Card>
  )
}

function EnvReadOnly({ items }: { items: ProjectCodespaceEnv[] }) {
  if (items.length === 0) {
    return <p className="text-sm text-muted-foreground">Nenhum ENV. Maintainer+ grava nesta seção.</p>
  }
  return (
    <ul className="flex flex-col gap-1 text-sm">
      {items.map((it) => (
        <li key={it.name} className="flex gap-2">
          <code>{it.name}</code>
          <span className="text-muted-foreground">{it.secret ? '••••' : '(sem valor nesta conta)'}</span>
        </li>
      ))}
    </ul>
  )
}
