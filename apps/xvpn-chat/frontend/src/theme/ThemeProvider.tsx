import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from 'react'
import type { ChatTheme } from '@chat/chatapi/types'

const KEY = 'xvpn-chat-theme'
const MIGRATED = 'xvpn-chat-theme-v2'

function readTheme(): ChatTheme {
  try {
    if (!localStorage.getItem(MIGRATED)) {
      localStorage.setItem(MIGRATED, '1')
      const prev = localStorage.getItem(KEY)
      // icq era o default, não uma escolha — alinha ao xvpn-client.
      if (!prev || prev === 'icq') {
        localStorage.setItem(KEY, 'dark')
        return 'dark'
      }
    }
    const v = localStorage.getItem(KEY)
    if (v === 'light' || v === 'dark' || v === 'icq') return v
  } catch {
    // storage indisponível
  }
  return 'dark'
}

const ThemeCtx = createContext<{ theme: ChatTheme; setTheme: (t: ChatTheme) => void } | null>(null)

export function ChatThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<ChatTheme>(readTheme)
  const setTheme = useCallback((t: ChatTheme) => {
    if (t === 'inherit') return
    setThemeState(t)
    try {
      localStorage.setItem(KEY, t)
    } catch {
      // ignore
    }
  }, [])
  const value = useMemo(() => ({ theme, setTheme }), [theme, setTheme])
  return <ThemeCtx.Provider value={value}>{children}</ThemeCtx.Provider>
}

export function useChatTheme() {
  const ctx = useContext(ThemeCtx)
  if (!ctx) throw new Error('useChatTheme fora do ChatThemeProvider')
  return ctx
}
