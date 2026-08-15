import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { api, ApiError, type SocialMessage, type SocialThread } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { connectSocialWS } from '@/lib/social-ws'
import { FilterBar } from '@/components/filter-bar'
import { DataTable, type DataTableColumn } from '@/components/data-table'
import { EmptyState } from '@/components/pagination'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

export function SocialMessagesPage() {
  const [page, setPage] = useState(1)
  const [q, setQ] = useState('')
  const [peer, setPeer] = useState('')
  const [active, setActive] = useState<SocialThread | null>(null)
  const [live, setLive] = useState<SocialMessage[]>([])
  const [typing, setTyping] = useState<string | null>(null)

  const fetchThreads = useCallback(() => api.listSocialThreads({ page, per_page: 25, q }), [page, q])
  const { data, loading, reload } = usePollingData(fetchThreads, 15_000)

  const fetchMessages = useCallback(() => {
    if (!active) return Promise.resolve({ items: [] as SocialMessage[], total: 0, page: 1, per_page: 50 })
    return api.listSocialMessages(active.kind, active.id, { page: 1, per_page: 50 })
  }, [active])
  const { data: history, reload: reloadHistory } = usePollingData(fetchMessages, 15_000)

  useEffect(() => {
    setLive([])
  }, [active?.id, active?.kind])

  useEffect(() => {
    return connectSocialWS((ev) => {
      if (ev.type === 'message.new' && ev.payload && typeof ev.payload === 'object') {
        const msg = ev.payload as SocialMessage
        if (active && msg.thread_kind === active.kind && msg.thread_id === active.id) {
          setLive((prev) => [...prev, msg])
        }
      }
      if (ev.type === 'typing') {
        setTyping('alguém está digitando…')
        window.setTimeout(() => setTyping(null), 1500)
      }
    })
  }, [active])

  const columns: DataTableColumn<SocialThread>[] = [
    { key: 'title', header: 'Conversa', cell: (t) => <span className="font-medium">{t.title}</span> },
    { key: 'kind', header: 'Tipo', cell: (t) => <span className="text-muted-foreground">{t.kind}</span> },
  ]

  async function openPeer(e: FormEvent) {
    e.preventDefault()
    const username = peer.trim()
    if (!username) return
    try {
      const th = await api.openSocialThread(username)
      setActive(th)
      setPeer('')
      reload()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Não foi possível abrir a conversa')
    }
  }

  const messages = [...(history?.items ?? []), ...live.filter((m) => !(history?.items ?? []).some((h) => h.id === m.id))]

  return (
    <div className="grid gap-6 lg:grid-cols-[minmax(0,18rem)_1fr]">
      <div className="flex flex-col gap-4">
        <form className="flex gap-2" onSubmit={openPeer}>
          <Input value={peer} onChange={(e) => setPeer(e.target.value)} placeholder="username para DM" />
          <Button type="submit">Abrir</Button>
        </form>
        <FilterBar
          q={q}
          onQChange={(next) => {
            setQ(next)
            setPage(1)
          }}
          placeholder="Filtrar conversas"
        />
        <Card>
          <CardContent className="pt-6">
            <DataTable
              columns={columns}
              rows={data?.items ?? []}
              rowKey={(t) => `${t.kind}-${t.id}`}
              loading={loading || !data}
              emptyTitle="Nenhuma conversa ainda."
              onRowClick={setActive}
              page={data?.page ?? page}
              perPage={data?.per_page ?? 25}
              total={data?.total ?? 0}
              onPageChange={setPage}
            />
          </CardContent>
        </Card>
      </div>
      <Card className="min-h-[24rem]">
        <CardHeader>
          <CardTitle className="text-base">{active ? active.title : 'Selecione uma conversa'}</CardTitle>
        </CardHeader>
        <CardContent>
          {!active ? (
            <EmptyState title="Nenhuma conversa selecionada." description="Abra um DM pelo username ou clique numa linha." />
          ) : (
            <ThreadPane
              kind={active.kind}
              threadId={active.id}
              messages={messages}
              typing={typing}
              onSent={() => {
                reloadHistory()
              }}
            />
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function ThreadPane({
  kind,
  threadId,
  messages,
  typing,
  onSent,
}: {
  kind: string
  threadId: number
  messages: SocialMessage[]
  typing: string | null
  onSent: () => void
}) {
  const [body, setBody] = useState('')
  const [sending, setSending] = useState(false)

  async function send(e: FormEvent) {
    e.preventDefault()
    const text = body.trim()
    if (!text) return
    setSending(true)
    try {
      await api.postSocialMessage(kind, threadId, text)
      setBody('')
      onSent()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao enviar')
    } finally {
      setSending(false)
    }
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex max-h-[22rem] flex-col gap-2 overflow-y-auto rounded-lg border border-white/5 p-3">
        {messages.length === 0 ? (
          <p className="text-sm text-muted-foreground">Ainda não há mensagens.</p>
        ) : (
          messages.map((m) => (
            <div key={m.id} className="rounded-md bg-white/5 px-3 py-2 text-sm">
              <p>{m.body}</p>
              <p className="mt-1 text-[10px] text-muted-foreground">{new Date(m.created_at).toLocaleString()}</p>
            </div>
          ))
        )}
      </div>
      {typing && <p className="text-xs text-muted-foreground">{typing}</p>}
      <form className="flex gap-2" onSubmit={send}>
        <Input value={body} onChange={(e) => setBody(e.target.value)} placeholder="Escreva uma mensagem" />
        <Button type="submit" disabled={sending}>
          Enviar
        </Button>
      </form>
    </div>
  )
}
