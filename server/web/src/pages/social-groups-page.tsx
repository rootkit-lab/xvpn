import { useCallback, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { api, ApiError, type SocialGroup, type SocialMessage } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { FilterBar } from '@/components/filter-bar'
import { DataTable, type DataTableColumn } from '@/components/data-table'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { EmptyState } from '@/components/pagination'

export function SocialGroupsPage() {
  const [page, setPage] = useState(1)
  const [q, setQ] = useState('')
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [active, setActive] = useState<SocialGroup | null>(null)
  const [invitee, setInvitee] = useState('')
  const [body, setBody] = useState('')

  const fetchGroups = useCallback(() => api.listSocialGroups({ page, per_page: 25, q }), [page, q])
  const { data, loading, reload } = usePollingData(fetchGroups, 15_000)

  const fetchMessages = useCallback(() => {
    if (!active) return Promise.resolve({ items: [] as SocialMessage[], total: 0, page: 1, per_page: 50 })
    return api.listSocialMessages('group', active.id, { page: 1, per_page: 50 })
  }, [active])
  const { data: history, reload: reloadHistory } = usePollingData(fetchMessages, 8_000)

  const columns: DataTableColumn<SocialGroup>[] = [
    { key: 'name', header: 'Grupo', cell: (g) => <span className="font-medium">{g.name}</span> },
    { key: 'desc', header: 'Descrição', cell: (g) => <span className="text-muted-foreground">{g.description || '—'}</span> },
    { key: 'n', header: 'Membros', cell: (g) => <span className="text-muted-foreground">{g.member_count}</span> },
  ]

  async function createGroup(e: FormEvent) {
    e.preventDefault()
    try {
      const g = await api.createSocialGroup(name.trim(), description.trim())
      setName('')
      setDescription('')
      setActive(g)
      reload()
      toast.success(`Grupo "${g.name}" criado`)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao criar grupo')
    }
  }

  async function invite(e: FormEvent) {
    e.preventDefault()
    if (!active) return
    try {
      await api.inviteToSocialGroup(active.id, invitee.trim())
      setInvitee('')
      toast.success('Convite enviado')
      reload()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao convidar')
    }
  }

  async function send(e: FormEvent) {
    e.preventDefault()
    if (!active) return
    try {
      await api.postSocialMessage('group', active.id, body.trim())
      setBody('')
      reloadHistory()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao enviar')
    }
  }

  return (
    <div className="grid gap-6 lg:grid-cols-[minmax(0,20rem)_1fr]">
      <div className="flex flex-col gap-4">
        <form className="flex flex-col gap-2" onSubmit={createGroup}>
          <Label htmlFor="group-name">Novo grupo</Label>
          <Input id="group-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="Nome" required />
          <Input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Descrição (opcional)" />
          <Button type="submit">Criar</Button>
        </form>
        <FilterBar
          q={q}
          onQChange={(next) => {
            setQ(next)
            setPage(1)
          }}
          placeholder="Filtrar grupos"
        />
        <Card>
          <CardContent className="pt-6">
            <DataTable
              columns={columns}
              rows={data?.items ?? []}
              rowKey={(g) => g.id}
              loading={loading || !data}
              emptyTitle="Você ainda não está em nenhum grupo."
              onRowClick={setActive}
              page={data?.page ?? page}
              perPage={data?.per_page ?? 25}
              total={data?.total ?? 0}
              onPageChange={setPage}
            />
          </CardContent>
        </Card>
      </div>
      <Card>
        <CardHeader>
          <CardTitle className="text-base">{active ? active.name : 'Selecione um grupo'}</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          {!active ? (
            <EmptyState title="Nenhum grupo selecionado." />
          ) : (
            <>
              <form className="flex gap-2" onSubmit={invite}>
                <Input value={invitee} onChange={(e) => setInvitee(e.target.value)} placeholder="username para convidar" />
                <Button type="submit" variant="secondary">
                  Convidar
                </Button>
              </form>
              <div className="flex max-h-[18rem] flex-col gap-2 overflow-y-auto rounded-lg border border-white/5 p-3">
                {(history?.items ?? []).map((m) => (
                  <p key={m.id} className="text-sm">
                    {m.body}
                  </p>
                ))}
                {(history?.items ?? []).length === 0 && (
                  <p className="text-sm text-muted-foreground">Ainda não há mensagens neste grupo.</p>
                )}
              </div>
              <form className="flex gap-2" onSubmit={send}>
                <Input value={body} onChange={(e) => setBody(e.target.value)} placeholder="Mensagem no grupo" />
                <Button type="submit">Enviar</Button>
              </form>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
