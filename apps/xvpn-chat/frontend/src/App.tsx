import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { Events } from '@wailsio/runtime'
import { ChatService } from '../bindings/github.com/rootkit-lab/xvpn/chat'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

type Session = { loggedIn: boolean; username: string; role: string }
type Profile = { user_id: number; username: string; display_name: string; bio: string }
type Thread = { id: number; kind: string; title: string }
type Message = { id: number; body: string; author_id: number; created_at: string }
type Group = { id: number; name: string; description: string; member_count: number }

type View = 'people' | 'messages' | 'groups'

export default function App() {
  const [session, setSession] = useState<Session | null>(null)
  const [view, setView] = useState<View>('messages')
  const [error, setError] = useState<string | null>(null)

  const refreshSession = useCallback(async () => {
    const s = await ChatService.Session()
    setSession(s)
  }, [])

  useEffect(() => {
    refreshSession()
  }, [refreshSession])

  useEffect(() => {
    if (!session?.loggedIn) return
    ChatService.StartEvents().catch(() => {})
    if (typeof Notification !== 'undefined' && Notification.permission === 'default') {
      Notification.requestPermission().catch(() => {})
    }
  }, [session?.loggedIn])

  if (!session) {
    return <p className="p-6 text-sm text-muted-foreground">Carregando…</p>
  }
  if (!session.loggedIn) {
    return <LoginForm onLoggedIn={refreshSession} error={error} setError={setError} />
  }

  return (
    <div className="flex h-full flex-col">
      <header className="flex items-center justify-between border-b border-white/10 px-4 py-2">
        <nav className="flex gap-2">
          <Button variant={view === 'people' ? 'default' : 'ghost'} size="sm" onClick={() => setView('people')}>
            Pessoas
          </Button>
          <Button variant={view === 'messages' ? 'default' : 'ghost'} size="sm" onClick={() => setView('messages')}>
            Mensagens
          </Button>
          <Button variant={view === 'groups' ? 'default' : 'ghost'} size="sm" onClick={() => setView('groups')}>
            Grupos
          </Button>
        </nav>
        <div className="flex items-center gap-2 text-sm">
          <span>{session.username}</span>
          <Button
            variant="ghost"
            size="sm"
            onClick={async () => {
              await ChatService.Logout()
              await refreshSession()
            }}
          >
            Sair
          </Button>
        </div>
      </header>
      <main className="min-h-0 flex-1 overflow-hidden p-4">
        {view === 'people' && <PeoplePane />}
        {view === 'messages' && <MessagesPane />}
        {view === 'groups' && <GroupsPane />}
      </main>
    </div>
  )
}

function LoginForm({
  onLoggedIn,
  error,
  setError,
}: {
  onLoggedIn: () => Promise<void>
  error: string | null
  setError: (s: string | null) => void
}) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      await ChatService.Login(username, password)
      await onLoggedIn()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex h-full items-center justify-center p-6">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>XVPN Chat</CardTitle>
        </CardHeader>
        <CardContent>
          <form className="flex flex-col gap-3" onSubmit={submit}>
            <div className="flex flex-col gap-1">
              <Label htmlFor="user">Usuário</Label>
              <Input id="user" value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" />
            </div>
            <div className="flex flex-col gap-1">
              <Label htmlFor="pass">Senha</Label>
              <Input
                id="pass"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete="current-password"
              />
            </div>
            {error && <p className="text-sm text-destructive">{error}</p>}
            <Button type="submit" disabled={busy}>
              {busy ? 'Entrando…' : 'Entrar'}
            </Button>
            <p className="text-xs text-muted-foreground">Mesma conta do painel. Conecta só em vpn.officeempresa.com.</p>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}

function PeoplePane() {
  const [q, setQ] = useState('')
  const [items, setItems] = useState<Profile[]>([])

  async function load() {
    const page = await ChatService.ListPeople(1, q)
    setItems(page.items ?? [])
  }

  useEffect(() => {
    load().catch(() => {})
  }, [])

  return (
    <div className="flex h-full flex-col gap-3">
      <form
        className="flex gap-2"
        onSubmit={(e) => {
          e.preventDefault()
          load().catch(() => {})
        }}
      >
        <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Buscar" />
        <Button type="submit">Buscar</Button>
      </form>
      <div className="flex-1 overflow-y-auto">
        {items.map((p) => (
          <button
            key={p.user_id}
            type="button"
            className="block w-full rounded-md px-3 py-2 text-left hover:bg-white/5"
            onClick={() => ChatService.OpenThread(p.username)}
          >
            <span className="font-medium">{p.display_name || p.username}</span>
            <span className="ml-2 text-sm text-muted-foreground">@{p.username}</span>
          </button>
        ))}
      </div>
    </div>
  )
}

function MessagesPane() {
  const [threads, setThreads] = useState<Thread[]>([])
  const [active, setActive] = useState<Thread | null>(null)
  const [messages, setMessages] = useState<Message[]>([])
  const [body, setBody] = useState('')
  const [peer, setPeer] = useState('')
  const [typing, setTyping] = useState(false)

  const reloadThreads = useCallback(async () => {
    const page = await ChatService.ListThreads(1)
    setThreads(page.items ?? [])
  }, [])

  const reloadMessages = useCallback(async () => {
    if (!active) return
    const page = await ChatService.ListMessages(active.kind, active.id, 1)
    setMessages(page.items ?? [])
  }, [active])

  useEffect(() => {
    reloadThreads().catch(() => {})
  }, [reloadThreads])

  useEffect(() => {
    reloadMessages().catch(() => {})
  }, [reloadMessages])

  useEffect(() => {
    const off = Events.On('social:event', (ev: { data?: { type?: string } }) => {
      const t = ev?.data?.type
      if (t === 'message.new') {
        reloadMessages().catch(() => {})
        if (typeof Notification !== 'undefined' && Notification.permission === 'granted' && document.hidden) {
          new Notification('XVPN Chat', { body: 'Nova mensagem' })
        }
      }
      if (t === 'typing') {
        setTyping(true)
        window.setTimeout(() => setTyping(false), 1500)
      }
    })
    return () => off()
  }, [reloadMessages])

  async function send(e: FormEvent) {
    e.preventDefault()
    if (!active || !body.trim()) return
    await ChatService.PostMessage(active.kind, active.id, body.trim())
    setBody('')
    await reloadMessages()
  }

  return (
    <div className="grid h-full grid-cols-[16rem_1fr] gap-3">
      <div className="flex flex-col gap-2 overflow-hidden">
        <form
          className="flex gap-1"
          onSubmit={async (e) => {
            e.preventDefault()
            if (!peer.trim()) return
            const th = await ChatService.OpenThread(peer.trim())
            setPeer('')
            setActive(th)
            await reloadThreads()
          }}
        >
          <Input value={peer} onChange={(e) => setPeer(e.target.value)} placeholder="DM username" />
        </form>
        <div className="flex-1 overflow-y-auto">
          {threads.map((th) => (
            <button
              key={`${th.kind}-${th.id}`}
              type="button"
              className="block w-full rounded-md px-3 py-2 text-left hover:bg-white/5"
              onClick={() => setActive(th)}
            >
              {th.title}
            </button>
          ))}
        </div>
      </div>
      <div className="flex min-h-0 flex-col">
        <div className="flex-1 overflow-y-auto rounded-lg border border-white/10 p-3">
          {messages.map((m) => (
            <p key={m.id} className="mb-2 text-sm">
              {m.body}
            </p>
          ))}
          {typing && <p className="text-xs text-muted-foreground">digitando…</p>}
        </div>
        <form className="mt-2 flex gap-2" onSubmit={send}>
          <Input value={body} onChange={(e) => setBody(e.target.value)} placeholder="Mensagem" disabled={!active} />
          <Button type="submit" disabled={!active}>
            Enviar
          </Button>
        </form>
      </div>
    </div>
  )
}

function GroupsPane() {
  const [groups, setGroups] = useState<Group[]>([])
  const [name, setName] = useState('')

  const load = useCallback(async () => {
    const page = await ChatService.ListGroups(1)
    setGroups(page.items ?? [])
  }, [])

  useEffect(() => {
    load().catch(() => {})
  }, [load])

  return (
    <div className="flex h-full flex-col gap-3">
      <form
        className="flex gap-2"
        onSubmit={async (e) => {
          e.preventDefault()
          if (!name.trim()) return
          await ChatService.CreateGroup(name.trim(), '')
          setName('')
          await load()
        }}
      >
        <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="Novo grupo" />
        <Button type="submit">Criar</Button>
      </form>
      <div className="flex-1 overflow-y-auto">
        {groups.map((g) => (
          <p key={g.id} className="px-3 py-2">
            {g.name} <span className="text-xs text-muted-foreground">{g.member_count} membros</span>
          </p>
        ))}
      </div>
    </div>
  )
}
