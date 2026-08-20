import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { toast } from 'sonner'
import { api, ApiError, type MarketplaceNetwork, type MarketplaceVisibility } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { canWriteAdminProduct, isAdminRole } from '@/lib/roles'
import { xgitPath } from '@/lib/xgit'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Checkbox } from '@/components/ui/checkbox'
import { Skeleton } from '@/components/ui/skeleton'

export function XgitSettingsPage() {
  const { user } = useAuth()
  const canWrite = isAdminRole(user?.role) && canWriteAdminProduct(user?.role, user?.products, 'forge')
  const fetchSettings = useCallback(() => api.getXgitSettings(), [])
  const fetchRepos = useCallback(() => api.listProjects('all'), [])
  const { data, loading, error, reload } = usePollingData(fetchSettings, 30_000)
  const { data: repos } = usePollingData(fetchRepos, 20_000)
  const [visibility, setVisibility] = useState<MarketplaceVisibility>('global')
  const [network, setNetwork] = useState<MarketplaceNetwork>('vpn')
  const [allowMember, setAllowMember] = useState(false)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!data) return
    setVisibility(data.default_visibility)
    setNetwork(data.default_network)
    setAllowMember(data.allow_member_create)
  }, [data])

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      await api.updateXgitSettings({
        default_visibility: visibility,
        default_network: network,
        allow_member_create: allowMember,
      })
      toast.success('Configurações do XGIT salvas')
      reload()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao salvar')
    } finally {
      setBusy(false)
    }
  }

  if (loading || !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-48 w-full" />
  }

  return (
    <div className="flex flex-col gap-6">
      <div>
        <p className="hud-label text-muted-foreground/70">XGIT</p>
        <h2 className="font-display text-2xl font-semibold tracking-tight">Configurações</h2>
      </div>
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Geral</CardTitle>
          <CardDescription>
            Clone só em <code className="font-mono text-xs">{data.clone_host}</code> com VPN. Sem git:// público. Sem
            shell SSH. Membros veem só os repositórios em que participam.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={submit} className="grid max-w-xl gap-4">
            <div className="space-y-1.5">
              <Label>Visibility padrão</Label>
              <Select value={visibility} onValueChange={(v) => setVisibility(v as MarketplaceVisibility)} disabled={!canWrite}>
                <SelectTrigger className="field-glass">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="global">global</SelectItem>
                  <SelectItem value="restricted">restricted</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label>Network padrão</Label>
              <Select value={network} onValueChange={(v) => setNetwork(v as MarketplaceNetwork)} disabled={!canWrite}>
                <SelectTrigger className="field-glass">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="vpn">vpn</SelectItem>
                  <SelectItem value="public">public</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <label className="flex items-center gap-2 text-sm">
              <Checkbox checked={allowMember} onCheckedChange={(v) => setAllowMember(v === true)} disabled={!canWrite} />
              Membros podem criar repositório
            </label>
            {canWrite ? (
              <Button type="submit" disabled={busy}>
                {busy ? 'Salvando…' : 'Salvar'}
              </Button>
            ) : null}
          </form>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle className="text-base">ACL do app</CardTitle>
          <CardDescription>
            Quem vê o tile XGIT no waffle: membros de um projeto (<code className="font-mono text-xs">ProjectMember</code>)
            ou ACL do app <code className="font-mono text-xs">xgit</code> no Marketplace. Papel viewer+ não libera o app —
            o console lista todos os repos aqui.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Link to="/admin/marketplace/acl" className="text-sm text-primary hover:underline">
            Liberar usuários no Marketplace → ACL
          </Link>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Repositórios</CardTitle>
          <CardDescription>Catálogo completo no xadmin. Membros do repo entram em Settings de cada um.</CardDescription>
        </CardHeader>
        <CardContent>
          {(repos?.items ?? []).length === 0 ? (
            <p className="text-sm text-muted-foreground">Nenhum repositório.</p>
          ) : (
            <ul className="divide-y divide-border/60">
              {repos?.items.map((p) => (
                <li key={p.slug} className="flex items-center justify-between gap-3 py-2 text-sm">
                  <Link to={xgitPath(`${p.org}/${p.slug}`)} className="text-primary hover:underline">
                    {p.full_name || `${p.org}/${p.slug}`}
                  </Link>
                  <span className="text-xs text-muted-foreground">
                    {p.visibility} · {p.network} · {p.member_count} membros
                  </span>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
