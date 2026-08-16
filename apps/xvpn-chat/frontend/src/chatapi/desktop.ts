import { Events } from '@wailsio/runtime'
import { ChatService } from '../../bindings/github.com/rootkit-lab/xvpn/chat'
import type { Attachment, ChatAPI, Group, Message, Page, Profile, StoryAuthor, StoryItem, Thread, WSEvent } from './types'

type Envelope = {
  items?: unknown[] | null
  total?: number
  page?: number
  per_page?: number
}

function asPage<T>(raw: Envelope | null | undefined, items: T[]): Page<T> {
  return {
    items,
    total: raw?.total ?? 0,
    page: raw?.page ?? 1,
    per_page: raw?.per_page ?? 25,
  }
}

function bytesToBase64(bytes: Uint8Array): string {
  let bin = ''
  const chunk = 0x8000
  for (let i = 0; i < bytes.length; i += chunk) {
    bin += String.fromCharCode(...bytes.subarray(i, i + chunk))
  }
  return btoa(bin)
}

function base64ToArrayBuffer(b64: string): ArrayBuffer {
  const bin = atob(b64)
  const out = new Uint8Array(bin.length)
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i)
  return out.buffer
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
      const raw = await ChatService.ListPeople(page, q)
      return asPage<Profile>(raw, (raw.items ?? []) as Profile[])
    },
    async listThreads(page) {
      const raw = await ChatService.ListThreads(page)
      return asPage(raw, (raw.items ?? []).map(mapThread))
    },
    async listGroups(page) {
      const raw = await ChatService.ListGroups(page)
      return asPage<Group>(raw, (raw.items ?? []) as Group[])
    },
    async openThread(username) {
      return mapThread(await ChatService.OpenThread(username))
    },
    async listMessages(kind, id, page) {
      const raw = await ChatService.ListMessages(kind, id, page)
      return asPage<Message>(raw, (raw.items ?? []) as Message[])
    },
    async postMessage(kind, id, body, extra) {
      if (extra?.attachment_id) {
        return (await ChatService.PostMediaMessage(kind, id, body, extra.kind ?? 'file', extra.attachment_id)) as Message
      }
      return (await ChatService.PostMessage(kind, id, body)) as Message
    },
    async createGroup(name, description) {
      return (await ChatService.CreateGroup(name, description)) as Group
    },
    async inviteToGroup(groupId, username) {
      await ChatService.InviteToGroup(groupId, username)
    },
    async uploadAttachment(file) {
      const buf = new Uint8Array(await file.arrayBuffer())
      return (await ChatService.UploadAttachment(file.name, file.type, bytesToBase64(buf))) as Attachment
    },
    async fetchAttachment(id) {
      const raw = await ChatService.DownloadAttachment(id)
      if (typeof raw !== 'string' || !raw) return new Blob()
      return new Blob([base64ToArrayBuffer(raw)])
    },
    async listStories() {
      return ((await ChatService.ListStories()) ?? []) as StoryAuthor[]
    },
    async createStory(body, extra) {
      return (await ChatService.CreateStory(body, extra?.kind ?? 'text', extra?.attachment_id ?? 0)) as StoryItem
    },
    async viewStory(id) {
      await ChatService.ViewStory(id)
    },
    async ackMessages(ids, state) {
      if (!ids.length) return
      await ChatService.AckMessages(ids, state)
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
    sendSignal(type, payload) {
      void ChatService.SendSignal(type, JSON.stringify(payload))
    },
  }
}
