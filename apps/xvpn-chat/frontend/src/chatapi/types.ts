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

export type MessageKind = 'text' | 'image' | 'file' | 'audio' | string

export type Message = {
  id: number
  thread_kind: string
  thread_id: number
  author_id: number
  kind?: MessageKind
  body: string
  attachment_id?: number
  filename?: string
  mime?: string
  size_bytes?: number
  delivered?: boolean
  read?: boolean
  created_at: string
}

export type Attachment = {
  id: number
  filename: string
  mime: string
  size_bytes: number
  kind: string
}

export type StoryItem = {
  id: number
  author_id: number
  username: string
  kind: 'text' | 'image' | string
  body: string
  attachment_id?: number
  mime?: string
  viewed: boolean
  expires_at: string
  created_at: string
}

export type StoryAuthor = {
  author_id: number
  username: string
  unseen: boolean
  items: StoryItem[]
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

export type PostMessageExtra = {
  kind?: MessageKind
  attachment_id?: number
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
  postMessage: (kind: string, id: number, body: string, extra?: PostMessageExtra) => Promise<Message>
  createGroup: (name: string, description: string) => Promise<Group>
  inviteToGroup: (groupId: number, username: string) => Promise<void>
  uploadAttachment: (file: File) => Promise<Attachment>
  fetchAttachment: (id: number) => Promise<Blob>
  listStories: () => Promise<StoryAuthor[]>
  createStory: (body: string, extra?: PostMessageExtra) => Promise<StoryItem>
  viewStory: (id: number) => Promise<void>
  connectEvents: (onEvent: (ev: WSEvent) => void) => () => void
  ackMessages: (ids: number[], state: 'delivered' | 'read') => Promise<void>
  sendTyping: (kind: string, threadId: number) => void
  setPresence: (status: string) => void
  sendSignal: (type: string, payload: Record<string, unknown>) => void
}
