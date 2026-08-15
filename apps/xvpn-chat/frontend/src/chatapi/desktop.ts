import { Events } from '@wailsio/runtime'
import { ChatService } from '../../bindings/github.com/rootkit-lab/xvpn/chat'
import type { ChatAPI, Group, Message, Page, Profile, Thread, WSEvent } from './types'

type WailsPage<T> = {
  items?: T[] | null
  total?: number
  page?: number
  per_page?: number
}

function asPage<T>(raw: WailsPage<T> | null | undefined): Page<T> {
  return {
    items: raw?.items ?? [],
    total: raw?.total ?? 0,
    page: raw?.page ?? 1,
    per_page: raw?.per_page ?? 25,
  }
}

function mapThread(th: {
  id: number
  kind?: string
  title?: string
  peer_user_id?: number | null
  last_body?: string | null
  last_at?: string | null
}): Thread {
  return {
    id: th.id,
    kind: th.kind ?? 'dm',
    title: th.title ?? '',
    peer_user_id: th.peer_user_id ?? undefined,
    last_body: th.last_body ?? undefined,
    last_at: th.last_at ?? undefined,
  }
}

export function createDesktopChatAPI(): ChatAPI {
  return {
    async session() {
      const s = await ChatService.Session()
      return {
        loggedIn: Boolean(s.loggedIn),
        username: s.username ?? '',
        role: s.role ?? '',
        userId: Number(s.userId ?? 0),
      }
    },
    async login(username, password) {
      const s = await ChatService.Login(username, password)
      return {
        loggedIn: Boolean(s.loggedIn),
        username: s.username ?? '',
        role: s.role ?? '',
        userId: Number(s.userId ?? 0),
      }
    },
    async logout() {
      await ChatService.Logout()
    },
    async listPeople(page, q) {
      return asPage<Profile>(await ChatService.ListPeople(page, q))
    },
    async listThreads(page) {
      return asPage<Thread>(await ChatService.ListThreads(page))
    },
    async listGroups(page) {
      return asPage<Group>(await ChatService.ListGroups(page))
    },
    async openThread(username) {
      return mapThread(await ChatService.OpenThread(username))
    },
    async listMessages(kind, id, page) {
      return asPage<Message>(await ChatService.ListMessages(kind, id, page))
    },
    async postMessage(kind, id, body) {
      return (await ChatService.PostMessage(kind, id, body)) as Message
    },
    async createGroup(name, description) {
      return (await ChatService.CreateGroup(name, description)) as Group
    },
    connectEvents(onEvent) {
      void ChatService.StartEvents()
      const off = Events.On('social:event', (ev: { data?: WSEvent }) => {
        if (ev?.data?.type) onEvent(ev.data)
      })
      return () => off()
    },
    sendTyping(kind, threadId) {
      void ChatService.SendTyping(kind, threadId)
    },
    setPresence(status) {
      void ChatService.SetPresence(status)
    },
  }
}
