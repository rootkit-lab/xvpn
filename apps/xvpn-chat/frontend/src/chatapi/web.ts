import type { ChatAPI, Group, Message, Page, Profile, Thread, WSEvent } from './types'

const TOKEN_KEY = 'xvpn_token'

function token(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function createWebChatAPI(onUnauthorized?: () => void): ChatAPI {
  let ws: WebSocket | null = null

  async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
    const headers = new Headers(options.headers)
    headers.set('Content-Type', 'application/json')
    const t = token()
    if (t) headers.set('Authorization', `Bearer ${t}`)
    const res = await fetch(`/api${path}`, { ...options, headers })
    if (res.status === 401) {
      onUnauthorized?.()
      throw new Error('sessão expirada')
    }
    if (!res.ok) {
      let msg = `Erro ${res.status}`
      try {
        const body = (await res.json()) as { error?: string }
        if (body.error) msg = body.error
      } catch {
        // corpo não-JSON
      }
      throw new Error(msg)
    }
    if (res.status === 204) return undefined as T
    return (await res.json()) as T
  }

  return {
    async session() {
      const t = token()
      if (!t) return { loggedIn: false, username: '', role: '', userId: 0 }
      const me = await request<{ id: number; username: string; role: string }>('/auth/me')
      return { loggedIn: true, username: me.username, role: me.role, userId: me.id }
    },
    async logout() {
      ws?.close()
      ws = null
    },
    listPeople: (page, q) =>
      request<Page<Profile>>(`/social/people?page=${page}&per_page=25${q ? `&q=${encodeURIComponent(q)}` : ''}`),
    listThreads: (page) => request<Page<Thread>>(`/social/threads?page=${page}&per_page=50`),
    listGroups: (page) => request<Page<Group>>(`/social/groups?page=${page}&per_page=50`),
    openThread: (username) =>
      request<Thread>('/social/threads', { method: 'POST', body: JSON.stringify({ username }) }),
    listMessages: (kind, id, page) =>
      request<Page<Message>>(`/social/threads/${kind}/${id}/messages?page=${page}&per_page=50`),
    postMessage: (kind, id, body) =>
      request<Message>(`/social/threads/${kind}/${id}/messages`, {
        method: 'POST',
        body: JSON.stringify({ body }),
      }),
    createGroup: (name, description) =>
      request<Group>('/social/groups', { method: 'POST', body: JSON.stringify({ name, description }) }),
    connectEvents(onEvent) {
      const t = token()
      if (!t) return () => {}
      const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const socket = new WebSocket(`${proto}//${window.location.host}/api/ws`)
      ws = socket
      socket.addEventListener('open', () => {
        socket.send(JSON.stringify({ type: 'auth', token: t }))
      })
      socket.addEventListener('message', (e) => {
        try {
          onEvent(JSON.parse(String(e.data)) as WSEvent)
        } catch {
          // frame inválido
        }
      })
      return () => {
        if (ws === socket) ws = null
        socket.close()
      }
    },
    sendTyping(kind, threadId) {
      ws?.send(JSON.stringify({ type: 'typing', payload: { thread_kind: kind, thread_id: threadId } }))
    },
    setPresence(status) {
      ws?.send(JSON.stringify({ type: 'presence', payload: { status } }))
    },
  }
}
