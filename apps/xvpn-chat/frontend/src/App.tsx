import { useState, type FormEvent } from 'react'
import { ChatThemeProvider } from '@chat/theme/ThemeProvider'
import { ChatSettingsProvider } from '@chat/messenger/ChatSettings'
import { ChatProvider, useChat } from '@chat/messenger/ChatProvider'
import { Messenger } from '@chat/messenger/Messenger'
import { ChatButton, ChatInput, ChatRoot } from '@chat/messenger/ui'
import { ChatShell } from '@chat/messenger/chrome'
import { useChatTheme } from '@chat/theme/ThemeProvider'
import { createDesktopChatAPI } from '@chat/chatapi/desktop'

const desktopApi = createDesktopChatAPI()

export default function App() {
  return (
    <ChatThemeProvider>
      <ChatSettingsProvider>
        <ChatProvider api={desktopApi} mode="desktop">
          <DesktopShell />
        </ChatProvider>
      </ChatSettingsProvider>
    </ChatThemeProvider>
  )
}

function DesktopShell() {
  const { session, loading, login, error } = useChat()
  const { theme } = useChatTheme()

  if (loading) {
    return (
      <ChatRoot theme={theme} className="h-full">
        <ChatShell className="items-center justify-center">
          <p className="relative z-10 font-display text-sm text-muted-foreground">Carregando…</p>
        </ChatShell>
      </ChatRoot>
    )
  }
  if (!session?.loggedIn) {
    return (
      <ChatRoot theme={theme} className="h-full">
        <ChatShell className="items-center justify-center">
          <LoginForm onLogin={login} error={error} />
        </ChatShell>
      </ChatRoot>
    )
  }
  return <Messenger />
}

function LoginForm({
  onLogin,
  error,
}: {
  onLogin: (u: string, p: string) => Promise<void>
  error: string | null
}) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [localError, setLocalError] = useState<string | null>(null)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setLocalError(null)
    try {
      await onLogin(username, password)
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <form className="relative z-10 w-full max-w-sm rounded-[22px] p-5 watch-complication" onSubmit={submit}>
      <img src="/logo-192.png" alt="" className="mx-auto mb-3 size-12 rounded-full" />
      <h1 className="mb-1 text-center font-display text-lg font-semibold tracking-tight">XVPN Chat</h1>
      <p className="mb-4 text-center font-display text-xs text-muted-foreground">
        Mesma conta do painel. Só vpn.officeempresa.com.
      </p>
      <label className="mb-1 block font-display text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground/75" htmlFor="user">
        Usuário
      </label>
      <ChatInput id="user" value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" />
      <label className="mb-1 mt-3 block font-display text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground/75" htmlFor="pass">
        Senha
      </label>
      <ChatInput
        id="pass"
        type="password"
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        autoComplete="current-password"
      />
      {(localError || error) && <p className="mt-2 text-sm text-destructive">{localError || error}</p>}
      <ChatButton type="submit" variant="safe" className="mt-4 w-full" disabled={busy}>
        {busy ? 'Entrando…' : 'Entrar'}
      </ChatButton>
    </form>
  )
}
