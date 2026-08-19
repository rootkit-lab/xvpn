import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { useLocation } from 'react-router-dom'
import { useChat } from '@chat/messenger/ChatProvider'
import { documentTitle, focusedChatKey } from '@/lib/document-title'

type ChatFocus = { title: string; unread: number }

const TitleCtx = createContext<{
  override: string
  setOverride: (label: string) => void
  chat: ChatFocus
  setChat: (next: ChatFocus) => void
}>({
  override: '',
  setOverride: () => {},
  chat: { title: '', unread: 0 },
  setChat: () => {},
})

export function DocumentTitleProvider({ children }: { children: ReactNode }) {
  const [override, setOverride] = useState('')
  const [chat, setChat] = useState<ChatFocus>({ title: '', unread: 0 })
  const value = useMemo(() => ({ override, setOverride, chat, setChat }), [override, chat])
  return <TitleCtx.Provider value={value}>{children}</TitleCtx.Provider>
}

/** Página que não cabe no path (Drive: pasta atual) empurra o rótulo aqui. */
export function useDocumentTitleOverride(label: string) {
  const { setOverride } = useContext(TitleCtx)
  useEffect(() => {
    setOverride(label)
    return () => setOverride('')
  }, [label, setOverride])
}

/** Dentro do ChatHost — conversa aberta e não-lidas no título da aba. */
export function ChatDocumentTitle() {
  const { pathname } = useLocation()
  const chat = useChat()
  const { setChat } = useContext(TitleCtx)

  const title = useMemo(() => {
    const key = focusedChatKey({ pathname, activeKey: chat.activeKey, popouts: chat.popouts })
    if (!key) return ''
    return chat.contactByKey(key)?.title ?? ''
  }, [chat, pathname])

  const unread = useMemo(() => Object.values(chat.unread).reduce((sum, n) => sum + n, 0), [chat.unread])

  useEffect(() => {
    setChat({ title, unread })
    return () => setChat({ title: '', unread: 0 })
  }, [setChat, title, unread])

  return null
}

export function DocumentTitle() {
  const { pathname } = useLocation()
  const { override, chat } = useContext(TitleCtx)

  useEffect(() => {
    document.title = documentTitle({
      hostname: window.location.hostname,
      pathname,
      pageOverride: override,
      chatTitle: chat.title,
      unread: chat.unread,
    })
  }, [pathname, override, chat.title, chat.unread])

  return null
}
