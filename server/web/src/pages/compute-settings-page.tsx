import { useCallback, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { api, ApiError, type BitLaunchAccount } from '@/lib/api'
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

export function ComputeSettingsPage() {
  const { user } = useAuth()
  const canWrite = isAdminRole(user?.role) && canWriteAdminProduct(user?.role, user?.products, 'compute')
  const fetchSettings = useCallback(() => api.getComputeSettings(), [])
  const { data, loading, reload } = usePollingData(fetchSettings, 20_000)

  const columns: DataTableColumn<BitLaunchAccount>[] = [
    { key: 'name', header: 'Nome', cell: (a) => <span className="font-medium">{a.name}</span> },
    { key: 'email', header: 'E-mail', cell: (a) => <span className="text-muted-foreground">{a.email}</span> },
    { key: 'hint', header: 'Token', cell: (a) => <code className="text-xs">{a.token_hint}</code> },
    {
      key: 'act',
      header: '',
      cell: (a) =>
        canWrite ? (
          <div className="flex justify-end gap-1">
            <EditAccount account={a} onSaved={reload} />
            <DeleteAccount account={a} onDeleted={reload} />
          </div>
        ) : null,
    },
  ]

  return (
    <div className="flex flex-col gap-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Contas BitLaunch</CardTitle>
          <CardDescription>
            Várias APIs/e-mails. O token fica só no VPS — a lista mostra só as últimas letras. Nunca
            commitar no Git.
          </CardDescription>
        </CardHeader>
      </Card>

      {canWrite ? <AddAccountForm onCreated={reload} /> : null}

      <DataTable
        columns={columns}
        rows={data?.accounts ?? []}
        rowKey={(a) => String(a.id)}
        loading={loading || !data}
        emptyTitle="Nenhuma conta ainda. Cadastre um e-mail + token da API."
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
      await api.createBitLaunchAccount({ name: name.trim(), email: email.trim(), token: token.trim() })
      setName('')
      setEmail('')
      setToken('')
      toast.success('Conta BitLaunch salva no VPS')
      onCreated()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao salvar conta')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Nova API</CardTitle>
        <CardDescription>E-mail da conta BitLaunch e o JWT da API. O token não volta na listagem.</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={submit} className="grid gap-3 sm:grid-cols-2">
          <div className="space-y-1.5">
            <Label htmlFor="bl-name">Nome</Label>
            <Input id="bl-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="Pessoal" required />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="bl-email">E-mail</Label>
            <Input
              id="bl-email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="voce@ihuull.com"
              required
            />
          </div>
          <div className="space-y-1.5 sm:col-span-2">
            <Label htmlFor="bl-token">Token da API</Label>
            <Input
              id="bl-token"
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

function EditAccount({ account, onSaved }: { account: BitLaunchAccount; onSaved: () => void }) {
  const [open, setOpen] = useState(false)
  const [name, setName] = useState(account.name)
  const [email, setEmail] = useState(account.email)
  const [token, setToken] = useState('')
  const [busy, setBusy] = useState(false)

  async function save() {
    setBusy(true)
    try {
      await api.updateBitLaunchAccount(account.id, {
        name: name.trim(),
        email: email.trim(),
        token: token.trim() || undefined,
      })
      setToken('')
      setOpen(false)
      toast.success('Conta atualizada')
      onSaved()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao atualizar')
    } finally {
      setBusy(false)
    }
  }

  return (
    <AlertDialog open={open} onOpenChange={setOpen}>
      <AlertDialogTrigger asChild>
        <Button type="button" variant="ghost" size="sm">
          Editar
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Editar {account.email}</AlertDialogTitle>
          <AlertDialogDescription>
            Token vazio mantém o atual. O valor novo não volta na listagem.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <div className="grid gap-3">
          <div className="space-y-1.5">
            <Label htmlFor={`edit-name-${account.id}`}>Nome</Label>
            <Input id={`edit-name-${account.id}`} value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor={`edit-email-${account.id}`}>E-mail</Label>
            <Input
              id={`edit-email-${account.id}`}
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor={`edit-token-${account.id}`}>Novo token (opcional)</Label>
            <Input
              id={`edit-token-${account.id}`}
              type="password"
              autoComplete="off"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              placeholder={account.token_hint}
            />
          </div>
        </div>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancelar</AlertDialogCancel>
          <AlertDialogAction onClick={save} disabled={busy || !name.trim() || !email.trim()}>
            {busy ? 'Salvando…' : 'Salvar'}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

function DeleteAccount({ account, onDeleted }: { account: BitLaunchAccount; onDeleted: () => void }) {
  const [busy, setBusy] = useState(false)
  async function destroy() {
    setBusy(true)
    try {
      await api.deleteBitLaunchAccount(account.id)
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
          <AlertDialogDescription>O token some do VPS. Servidores já importados continuam no inventário.</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancelar</AlertDialogCancel>
          <AlertDialogAction onClick={destroy}>Remover</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
