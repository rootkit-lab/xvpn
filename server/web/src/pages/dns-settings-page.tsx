import { useCallback, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { api, ApiError, type CloudflareAccount } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { canWriteAdminProduct, isAdminRole } from '@/lib/roles'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { DataTable, type DataTableColumn } from '@/components/data-table'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'

export function DNSSettingsPage() {
  const { user } = useAuth()
  const canWrite = isAdminRole(user?.role) && canWriteAdminProduct(user?.role, user?.products, 'dns')
  const fetchSettings = useCallback(() => api.getPublicDNSSettings(), [])
  const { data, loading, reload } = usePollingData(fetchSettings, 20_000)

  const columns: DataTableColumn<CloudflareAccount>[] = [
    { key: 'name', header: 'Nome', cell: (a) => <span className="font-medium">{a.name}</span> },
    { key: 'email', header: 'E-mail', cell: (a) => <span className="text-muted-foreground">{a.email}</span> },
    { key: 'hint', header: 'Token', cell: (a) => <code className="text-xs">{a.token_hint}</code> },
    {
      key: 'act',
      header: '',
      cell: (a) => (canWrite ? <DeleteAccount account={a} onDeleted={reload} /> : null),
    },
  ]

  return (
    <div className="flex flex-col gap-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Contas Cloudflare</CardTitle>
          <CardDescription>
            Os nameservers do stack saem desta conta. Token só no VPS — a lista mostra um hint. Sem
            :53 na eth0.
          </CardDescription>
        </CardHeader>
      </Card>
      {canWrite ? <AddAccountForm onCreated={reload} /> : null}
      <DataTable
        columns={columns}
        rows={data?.accounts ?? []}
        rowKey={(a) => String(a.id)}
        loading={loading || !data}
        emptyTitle="Nenhuma conta. Cadastre o token da API (Zone + DNS Edit)."
        page={1}
        perPage={50}
        total={data?.accounts.length ?? 0}
        onPageChange={() => undefined}
      />
    </div>
  )
}

function AddAccountForm({ onCreated }: { onCreated: () => void }) {
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [token, setToken] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      await api.createCloudflareAccount({ name: name.trim(), email: email.trim(), token: token.trim() })
      setName('')
      setEmail('')
      setToken('')
      toast.success('Conta Cloudflare salva no VPS')
      onCreated()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao salvar')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Nova API</CardTitle>
        <CardDescription>O mesmo token do Certbot DNS-01 serve. Nunca commitar no Git.</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={submit} className="grid gap-3 sm:grid-cols-2">
          <div className="space-y-1.5">
            <Label htmlFor="cf-name">Nome</Label>
            <Input id="cf-name" value={name} onChange={(e) => setName(e.target.value)} required />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="cf-email">E-mail</Label>
            <Input id="cf-email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
          </div>
          <div className="space-y-1.5 sm:col-span-2">
            <Label htmlFor="cf-token">Token</Label>
            <Input
              id="cf-token"
              type="password"
              autoComplete="off"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              required
            />
          </div>
          <div className="sm:col-span-2">
            <Button type="submit" disabled={busy || token.trim().length < 16}>
              {busy ? 'Salvando…' : 'Salvar no VPS'}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  )
}

function DeleteAccount({ account, onDeleted }: { account: CloudflareAccount; onDeleted: () => void }) {
  const [busy, setBusy] = useState(false)
  async function destroy() {
    setBusy(true)
    try {
      await api.deleteCloudflareAccount(account.id)
      toast.success('Conta removida')
      onDeleted()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao remover')
      setBusy(false)
    }
  }
  return (
    <AlertDialog>
      <AlertDialogTrigger asChild>
        <Button type="button" variant="ghost" size="sm" disabled={busy}>
          Remover
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Remover {account.email}?</AlertDialogTitle>
          <AlertDialogDescription>O token some do VPS. Zonas já importadas continuam no inventário.</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancelar</AlertDialogCancel>
          <AlertDialogAction onClick={destroy}>Remover</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
