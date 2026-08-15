import { useMemo, type ReactNode } from 'react'
import { useLocation } from 'react-router-dom'
import { ChatThemeProvider } from '@chat/theme/ThemeProvider'
import { ChatProvider } from '@chat/messenger/ChatProvider'
import { ChatDock } from '@chat/messenger/ChatDock'
import { createWebChatAPI } from '@chat/chatapi/web'
import { useAuth } from '@/lib/auth-context'
import { clearToken } from '@/lib/api'

/** Dock + provider acima dos shells — sobrevive à troca /my ↔ /admin ↔ /social. */
export function ChatHost({ children }: { children: ReactNode }) {
  const { isAuthenticated } = useAuth()
  const { pathname } = useLocation()
  const api = useMemo(
    () =>
      createWebChatAPI(() => {
        clearToken()
        const here = window.location.pathname
        const login = here.startsWith('/admin') ? '/admin/login' : '/my/login'
        if (here !== login) window.location.href = login
      }),
    [],
  )

  const hideDock =
    !isAuthenticated ||
    pathname === '/' ||
    pathname === '/my/login' ||
    pathname === '/admin/login' ||
    pathname.startsWith('/social/messages')

  return (
    <ChatThemeProvider>
      <ChatProvider api={api} mode="web" enabled={isAuthenticated}>
        {children}
        <ChatDock hidden={hideDock} />
      </ChatProvider>
    </ChatThemeProvider>
  )
}
