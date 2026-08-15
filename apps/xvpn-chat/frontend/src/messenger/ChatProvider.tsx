import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import type { ChatAPI, Message, PresenceStatus, Profile, Session, Thread } from '@chat/chatapi/types'
import { OPEN_CHAT_EVENT, type OpenChatDetail } from '@chat/messenger/open-chat'
import { playMessageSound } from '@chat/messenger/sound'

export type Contact = {
  key: string
  kind: 'dm' | 'group'
  id: number
  title: string
  peerUserId?: number
  lastBody?: string
  lastAt?: string
}

function contactKey(kind: string, id: number): string {
  return `${kind}:${id}`
}

function threadToContact(t: Thread): Contact {
  const kind = t.kind === 'group' ? 'group' : 'dm'
  return {
    key: contactKey(kind, t.id),
    kind,
    id: t.id,
    title: t.title,
    peerUserId: t.peer_user_id,
    lastBody: t.last_body,
    lastAt: t.last_at,
  }
}

export type ChatPopout = { key: string; minimized: boolean }

const MAX_EXPANDED_POPOUTS = 3

function upsertPopout(prev: ChatPopout[], key: string): ChatPopout[] {
  const rest = prev.filter((p) => p.key !== key)
  let next: ChatPopout[] = [...rest, { key, minimized: false }]
  const expanded = next.filter((p) => !p.minimized)
  if (expanded.length <= MAX_EXPANDED_POPOUTS) return next
  const oldest = expanded[0]
  return next.map((p) => (p.key === oldest.key ? { ...p, minimized: true } : p))
}

type ChatContextValue = {
  api: ChatAPI
  mode: 'web' | 'desktop'
  session: Session | null
  myStatus: Exclude<PresenceStatus, 'offline'>
  setMyStatus: (s: Exclude<PresenceStatus, 'offline'>) => void
  contacts: Contact[]
  presence: Record<number, PresenceStatus>
  messages: Record<string, Message[]>
  typing: Record<string, boolean>
  unread: Record<string, number>
  activeKey: string | null
  setActiveKey: (k: string | null) => void
  popouts: ChatPopout[]
  openPopout: (key: string, hint?: Pick<Contact, 'kind' | 'id'>) => void
  closePopout: (key: string) => void
  minimizePopout: (key: string) => void
  dockOpen: boolean
  setDockOpen: (v: boolean) => void
  query: string
  setQuery: (q: string) => void
  people: Profile[]
  searchPeople: (q: string) => Promise<void>
  startDM: (username: string) => Promise<Contact | null>
  openGroup: (id: number, title: string) => Contact
  send: (body: string, key?: string) => Promise<void>
  login: (username: string, password: string) => Promise<void>
  logout: () => Promise<void>
  error: string | null
  loading: boolean
  contactByKey: (key: string) => Contact | undefined
}

const ChatCtx = createContext<ChatContextValue | null>(null)

export function ChatProvider({
  api,
  mode,
  enabled = true,
  children,
}: {
  api: ChatAPI
  mode: 'web' | 'desktop'
  enabled?: boolean
  children: ReactNode
}) {
  const [session, setSession] = useState<Session | null>(null)
  const [myStatus, setMyStatusState] = useState<Exclude<PresenceStatus, 'offline'>>('online')
  const [contacts, setContacts] = useState<Contact[]>([])
  const [presence, setPresence] = useState<Record<number, PresenceStatus>>({})
  const [messages, setMessages] = useState<Record<string, Message[]>>({})
  const [typing, setTyping] = useState<Record<string, boolean>>({})
  const [unread, setUnread] = useState<Record<string, number>>({})
  const [activeKey, setActiveKeyState] = useState<string | null>(null)
  const [popouts, setPopouts] = useState<ChatPopout[]>([])
  const [dockOpen, setDockOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [people, setPeople] = useState<Profile[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  const refreshSession = useCallback(async () => {
    try {
      const s = await api.session()
      setSession(s.loggedIn ? s : null)
    } catch {
      setSession(null)
    } finally {
      setLoading(false)
    }
  }, [api])

  const loadLists = useCallback(async () => {
    const [th, gr] = await Promise.all([api.listThreads(1), api.listGroups(1)])
    const dms = (th.items ?? []).map(threadToContact)
    const groups: Contact[] = (gr.items ?? []).map((g) => ({
      key: contactKey('group', g.id),
      kind: 'group',
      id: g.id,
      title: g.name,
    }))
    setContacts([...dms, ...groups])
  }, [api])

  const popoutsRef = useRef<ChatPopout[]>([])
  const contactsRef = useRef<Contact[]>([])
  const loadListsRef = useRef(loadLists)
  popoutsRef.current = popouts
  contactsRef.current = contacts
  loadListsRef.current = loadLists

  useEffect(() => {
    if (!enabled) {
      setLoading(false)
      return
    }
    void refreshSession()
  }, [enabled, refreshSession])

  useEffect(() => {
    if (!session?.loggedIn) return
    void loadLists().catch((err: unknown) => setError(err instanceof Error ? err.message : String(err)))
  }, [session?.loggedIn, loadLists])

  const loadMessages = useCallback(
    async (key: string, kind: string, id: number) => {
      const page = await api.listMessages(kind, id, 1)
      setMessages((prev) => ({ ...prev, [key]: page.items ?? [] }))
    },
    [api],
  )

  const openPopout = useCallback(
    (key: string, hint?: Pick<Contact, 'kind' | 'id'>) => {
      setPopouts((prev) => upsertPopout(prev, key))
      setActiveKeyState(key)
      setUnread((u) => ({ ...u, [key]: 0 }))
      const c = hint ?? contactsRef.current.find((x) => x.key === key)
      if (c) void loadMessages(key, c.kind, c.id)
    },
    [loadMessages],
  )
  const openPopoutRef = useRef(openPopout)
  openPopoutRef.current = openPopout

  const closePopout = useCallback((key: string) => {
    setPopouts((prev) => prev.filter((p) => p.key !== key))
    setActiveKeyState((a) => (a === key ? null : a))
  }, [])

  const minimizePopout = useCallback((key: string) => {
    setPopouts((prev) => prev.map((p) => (p.key === key ? { ...p, minimized: true } : p)))
    setActiveKeyState((a) => (a === key ? null : a))
  }, [])

  const setActiveKey = useCallback(
    (k: string | null) => {
      if (k) openPopout(k)
      else setActiveKeyState(null)
    },
    [openPopout],
  )

  useEffect(() => {
    if (!session?.loggedIn || !enabled) return
    const close = api.connectEvents((ev) => {
      if (ev.type === 'presence.snapshot' && Array.isArray(ev.payload)) {
        const next: Record<number, PresenceStatus> = {}
        for (const row of ev.payload as { user_id?: number; status?: PresenceStatus }[]) {
          if (row.user_id) next[row.user_id] = row.status ?? 'offline'
        }
        setPresence(next)
      }
      if (ev.type === 'presence' && ev.payload && typeof ev.payload === 'object') {
        const p = ev.payload as { user_id?: number; status?: PresenceStatus }
        if (p.user_id && p.status) {
          setPresence((prev) => ({ ...prev, [p.user_id!]: p.status! }))
        }
      }
      if (ev.type === 'typing' && ev.payload && typeof ev.payload === 'object') {
        const p = ev.payload as { thread_kind?: string; thread_id?: number; user_id?: number }
        if (!p.thread_kind || !p.thread_id) return
        if (p.user_id && p.user_id === session.userId) return
        const key = contactKey(p.thread_kind, p.thread_id)
        setTyping((prev) => ({ ...prev, [key]: true }))
        window.setTimeout(() => setTyping((prev) => ({ ...prev, [key]: false })), 1600)
      }
      if (ev.type === 'message.new' && ev.payload && typeof ev.payload === 'object') {
        const msg = ev.payload as Message
        const key = contactKey(msg.thread_kind, msg.thread_id)
        setMessages((prev) => {
          const list = prev[key] ?? []
          if (list.some((m) => m.id === msg.id)) return prev
          return { ...prev, [key]: [...list, msg] }
        })
        setContacts((prev) =>
          prev.map((c) => (c.key === key ? { ...c, lastBody: msg.body, lastAt: msg.created_at } : c)),
        )
        const focused = popoutsRef.current.some((p) => p.key === key && !p.minimized)
        if (!focused || document.hidden) {
          setUnread((prev) => ({ ...prev, [key]: (prev[key] ?? 0) + 1 }))
          if (document.hidden) {
            playMessageSound()
            if (typeof Notification !== 'undefined' && Notification.permission === 'granted') {
              new Notification('XVPN Chat', { body: msg.body.slice(0, 80) })
            }
          }
        }
      }
      if (ev.type === 'group.updated') {
        void loadListsRef.current()
      }
    })
    return close
  }, [api, session?.loggedIn, session?.userId, enabled])

  useEffect(() => {
    const onOpen = (e: Event) => {
      const detail = (e as CustomEvent<OpenChatDetail>).detail
      if (!detail) return
      setDockOpen(true)
      if (detail.username) {
        void (async () => {
          const th = await api.openThread(detail.username!)
          const c = threadToContact(th)
          setContacts((prev) => (prev.some((x) => x.key === c.key) ? prev : [c, ...prev]))
          openPopoutRef.current(c.key, c)
        })()
      }
      if (detail.groupId) {
        const c: Contact = {
          key: contactKey('group', detail.groupId),
          kind: 'group',
          id: detail.groupId,
          title: detail.title || `Grupo #${detail.groupId}`,
        }
        setContacts((prev) => (prev.some((x) => x.key === c.key) ? prev : [c, ...prev]))
        openPopoutRef.current(c.key, c)
      }
    }
    window.addEventListener(OPEN_CHAT_EVENT, onOpen)
    return () => window.removeEventListener(OPEN_CHAT_EVENT, onOpen)
  }, [api])

  const setMyStatus = useCallback(
    (s: Exclude<PresenceStatus, 'offline'>) => {
      setMyStatusState(s)
      api.setPresence(s)
    },
    [api],
  )

  const searchPeople = useCallback(
    async (q: string) => {
      const page = await api.listPeople(1, q)
      setPeople(page.items ?? [])
    },
    [api],
  )

  const startDM = useCallback(
    async (username: string) => {
      const th = await api.openThread(username)
      const c = threadToContact(th)
      setContacts((prev) => (prev.some((x) => x.key === c.key) ? prev : [c, ...prev]))
      openPopout(c.key, c)
      setDockOpen(true)
      return c
    },
    [api, openPopout],
  )

  const openGroup = useCallback(
    (id: number, title: string) => {
      const c: Contact = { key: contactKey('group', id), kind: 'group', id, title }
      setContacts((prev) => (prev.some((x) => x.key === c.key) ? prev : [c, ...prev]))
      openPopout(c.key, c)
      return c
    },
    [openPopout],
  )

  const send = useCallback(
    async (body: string, key?: string) => {
      const k = key ?? activeKey
      if (!k || !body.trim()) return
      const c = contacts.find((x) => x.key === k)
      if (!c) return
      const msg = await api.postMessage(c.kind, c.id, body.trim())
      setMessages((prev) => {
        const list = prev[k] ?? []
        if (list.some((m) => m.id === msg.id)) return prev
        return { ...prev, [k]: [...list, msg] }
      })
    },
    [activeKey, api, contacts],
  )

  const login = useCallback(
    async (username: string, password: string) => {
      if (!api.login) throw new Error('login indisponível')
      setError(null)
      const s = await api.login(username, password)
      setSession(s)
      if (typeof Notification !== 'undefined' && Notification.permission === 'default') {
        void Notification.requestPermission()
      }
    },
    [api],
  )

  const logout = useCallback(async () => {
    await api.logout()
    setSession(null)
    setContacts([])
    setMessages({})
    setPopouts([])
    setActiveKeyState(null)
  }, [api])

  const contactByKey = useCallback((key: string) => contacts.find((c) => c.key === key), [contacts])

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!q) return contacts
    return contacts.filter((c) => c.title.toLowerCase().includes(q) || (c.lastBody ?? '').toLowerCase().includes(q))
  }, [contacts, query])

  const value = useMemo(
    () => ({
      api,
      mode,
      session,
      myStatus,
      setMyStatus,
      contacts: filtered,
      presence,
      messages,
      typing,
      unread,
      activeKey,
      setActiveKey,
      popouts,
      openPopout,
      closePopout,
      minimizePopout,
      dockOpen,
      setDockOpen,
      query,
      setQuery,
      people,
      searchPeople,
      startDM,
      openGroup,
      send,
      login,
      logout,
      error,
      loading,
      contactByKey,
    }),
    [
      api,
      mode,
      session,
      myStatus,
      setMyStatus,
      filtered,
      presence,
      messages,
      typing,
      unread,
      activeKey,
      setActiveKey,
      popouts,
      openPopout,
      closePopout,
      minimizePopout,
      dockOpen,
      query,
      people,
      searchPeople,
      startDM,
      openGroup,
      send,
      login,
      logout,
      error,
      loading,
      contactByKey,
    ],
  )

  return <ChatCtx.Provider value={value}>{children}</ChatCtx.Provider>
}

export function useChat(): ChatContextValue {
  const ctx = useContext(ChatCtx)
  if (!ctx) throw new Error('useChat fora do ChatProvider')
  return ctx
}
