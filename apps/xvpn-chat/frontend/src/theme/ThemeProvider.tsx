import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from 'react'
import type { ChatTheme } from '@chat/chatapi/types'

const KEY = 'xvpn-chat-theme'

function readTheme(): ChatTheme {
  try {
    const v = localStorage.getItem(KEY)
    if (v === 'light' || v === 'dark' || v === 'icq') return v
  } catch {
    // storage indisponível
  }
  return 'icq'
}

const ThemeCtx = createContext<{ theme: ChatTheme; setTheme: (t: ChatTheme) => void } | null>(null)

export function ChatThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<ChatTheme>(readTheme)
  const setTheme = useCallback((t: ChatTheme) => {
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
