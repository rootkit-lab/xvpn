import { useCallback, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import { api, ApiError, type ManagedService } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { canWriteAdminProduct, isAdminRole } from '@/lib/roles'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { ServiceStatusBadge } from '@/pages/services-page'

export function ServiceDetailPage() {
  const { slug = '' } = useParams()
  const { user } = useAuth()
  const navigate = useNavigate()
  const canWrite = isAdminRole(user?.role) && canWriteAdminProduct(user?.role, user?.products, 'managed')
  const fetchService = useCallback(() => api.getService(slug), [slug])
  const { data, loading, error, reload } = usePollingData(fetchService, 10_000)
  const [busy, setBusy] = useState('')
  const [once, setOnce] = useState('')

  if (loading || !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-48 w-full" />
  }

  async function run(label: string, fn: () => Promise<ManagedService | { ok: boolean }>) {
    setBusy(label)
    try {
      const out = await fn()
      if ('password' in out && out.password) setOnce(out.password)
      toast.success('Atualizado')
      reload()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha')
    } finally {
      setBusy('')
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <p className="text-sm text-muted-foreground">
        <Link to="/admin/services" className="hover:underline">
          Serviços
        </Link>
        <span className="px-1.5">/</span>
        <span className="text-foreground">{data.slug}</span>
      </p>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{data.slug}</CardTitle>
          <CardDescription>
            {data.kind} · {data.host === 'local' ? 'este VPS' : data.mesh_hostname || 'malha'} · bind {data.bind}
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-3 text-sm">
          <div className="flex flex-wrap items-center gap-2">
            <ServiceStatusBadge status={data.status} />
            {data.project_slug ? (
              <Link to={`/admin/xgit/${data.project_slug}`} className="hover:underline">
                {data.project_slug}
              </Link>
            ) : null}
          </div>
          <p className="font-mono text-xs break-all">{data.endpoint}</p>
          {data.hostname ? <p className="text-muted-foreground">{data.hostname} → {data.listen}:{data.port}</p> : null}
          {data.error ? <p className="text-destructive">{data.error}</p> : null}
          {once ? (
            <pre className="watch-complication overflow-x-auto rounded-[18px] p-4 font-mono text-xs">
              {`senha (copie agora):\n${once}`}
            </pre>
          ) : (
            <p className="text-muted-foreground">A senha só aparece na criação ou ao rotacionar.</p>
          )}
          {canWrite ? (
            <div className="flex flex-wrap gap-2">
              <Button type="button" variant="outline" disabled={!!busy} onClick={() => void run('apply', () => api.applyService(data.slug))}>
                {busy === 'apply' ? 'Aplicando…' : 'Reaplicar'}
              </Button>
              <Button type="button" variant="outline" disabled={!!busy} onClick={() => void run('rotate', () => api.rotateService(data.slug))}>
                {busy === 'rotate' ? 'Gerando…' : 'Nova senha'}
              </Button>
              <Button type="button" variant="outline" disabled={!!busy} onClick={() => void run('stop', () => api.stopService(data.slug))}>
                {busy === 'stop' ? 'Parando…' : 'Parar'}
              </Button>
              <Button
                type="button"
                variant="destructive"
                disabled={!!busy}
                onClick={() => {
                  if (!window.confirm(`Apagar ${data.slug}? A unit some deste host.`)) return
                  void (async () => {
                    setBusy('del')
                    try {
                      await api.deleteService(data.slug)
                      toast.success('Removido')
                      navigate('/admin/services')
                    } catch (err) {
                      toast.error(err instanceof ApiError ? err.message : 'Falha')
                    } finally {
                      setBusy('')
                    }
                  })()
                }}
              >
                Apagar
              </Button>
            </div>
          ) : null}
        </CardContent>
      </Card>
    </div>
  )
}
