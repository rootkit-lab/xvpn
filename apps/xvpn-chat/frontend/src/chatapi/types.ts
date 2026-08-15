export type PresenceStatus = 'online' | 'away' | 'dnd' | 'invisible' | 'offline'

export type ChatTheme = 'light' | 'dark' | 'icq' | 'inherit'

export type Session = {
  loggedIn: boolean
  username: string
  role: string
  userId: number
}

export type Profile = {
  user_id: number
  username: string
  display_name: string
  bio: string
  avatar_url: string
}

export type Group = {
  id: number
  name: string
  description: string
  owner_user_id: number
  member_count: number
}

export type Thread = {
  id: number
  kind: 'dm' | 'group' | string
  title: string
  peer_user_id?: number
  last_body?: string
  last_at?: string
}

export type Message = {
  id: number
  thread_kind: string
  thread_id: number
  author_id: number
  body: string
  created_at: string
}

export type Page<T> = {
  items: T[]
  total: number
  page: number
  per_page: number
}

export type WSEvent = {
  type: string
  payload?: unknown
}

export type ChatAPI = {
  session: () => Promise<Session>
  login?: (username: string, password: string) => Promise<Session>
  logout: () => Promise<void>
  listPeople: (page: number, q: string) => Promise<Page<Profile>>
  listThreads: (page: number) => Promise<Page<Thread>>
  listGroups: (page: number) => Promise<Page<Group>>
  openThread: (username: string) => Promise<Thread>
  listMessages: (kind: string, id: number, page: number) => Promise<Page<Message>>
  postMessage: (kind: string, id: number, body: string) => Promise<Message>
  createGroup: (name: string, description: string) => Promise<Group>
  connectEvents: (onEvent: (ev: WSEvent) => void) => () => void
  sendTyping: (kind: string, threadId: number) => void
  setPresence: (status: string) => void
}
