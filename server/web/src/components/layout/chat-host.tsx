import { useMemo, type ReactNode } from 'react'
import { ChatThemeProvider } from '@chat/theme/ThemeProvider'
import { ChatSettingsProvider } from '@chat/messenger/ChatSettings'
import { ChatProvider } from '@chat/messenger/ChatProvider'
import { createWebChatAPI } from '@chat/chatapi/web'
import { useAuth } from '@/lib/auth-context'
import { clearToken } from '@/lib/api'

/** Provider acima dos shells — o painel vive na sidebar + status bar do SystemChrome. */
export function ChatHost({ children }: { children: ReactNode }) {
  const { isAuthenticated } = useAuth()
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

  return (
    <ChatThemeProvider>
      <ChatSettingsProvider>
        <ChatProvider api={api} mode="web" enabled={isAuthenticated}>
          {children}
        </ChatProvider>
      </ChatSettingsProvider>
    </ChatThemeProvider>
  )
}
