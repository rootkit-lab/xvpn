import { getToken } from '@/lib/api'

export type SocialWSEvent = {
  type: string
  payload?: unknown
}

export function connectSocialWS(onEvent: (ev: SocialWSEvent) => void): () => void {
  const token = getToken()
  if (!token) return () => {}

  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const ws = new WebSocket(`${proto}//${window.location.host}/api/ws`)
  let closed = false

  ws.addEventListener('open', () => {
    ws.send(JSON.stringify({ type: 'auth', token }))
  })
  ws.addEventListener('message', (e) => {
    try {
      const ev = JSON.parse(String(e.data)) as SocialWSEvent
      onEvent(ev)
    } catch {
      // frame inválido
    }
  })

  return () => {
    if (closed) return
    closed = true
    ws.close()
  }
}

export function sendTyping(wsSend: (raw: string) => void, threadKind: string, threadId: number) {
  wsSend(JSON.stringify({ type: 'typing', payload: { thread_kind: threadKind, thread_id: threadId } }))
}
