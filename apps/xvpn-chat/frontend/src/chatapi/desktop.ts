import { Events } from '@wailsio/runtime'
import { ChatService } from '../../bindings/github.com/rootkit-lab/xvpn/chat'
import type { ChatAPI, Group, Message, Page, Profile, Session, Thread, WSEvent } from './types'

function asPage<T>(raw: Page<T> | null | undefined): Page<T> {
  return {
    items: raw?.items ?? [],
    total: raw?.total ?? 0,
    page: raw?.page ?? 1,
    per_page: raw?.per_page ?? 25,
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
    openThread: (username) => ChatService.OpenThread(username),
    async listMessages(kind, id, page) {
      return asPage<Message>(await ChatService.ListMessages(kind, id, page))
    },
    postMessage: (kind, id, body) => ChatService.PostMessage(kind, id, body),
    createGroup: (name, description) => ChatService.CreateGroup(name, description),
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
