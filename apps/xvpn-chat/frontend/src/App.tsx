import { useState, type FormEvent } from 'react'
import { ChatThemeProvider } from '@chat/theme/ThemeProvider'
import { ChatProvider, useChat } from '@chat/messenger/ChatProvider'
import { Messenger } from '@chat/messenger/Messenger'
import { ChatButton, ChatInput, ChatRoot } from '@chat/messenger/ui'
import { useChatTheme } from '@chat/theme/ThemeProvider'
import { createDesktopChatAPI } from '@chat/chatapi/desktop'

const desktopApi = createDesktopChatAPI()

export default function App() {
  return (
    <ChatThemeProvider>
      <ChatProvider api={desktopApi} mode="desktop">
        <DesktopShell />
      </ChatProvider>
    </ChatThemeProvider>
  )
}

function DesktopShell() {
  const { session, loading, login, error } = useChat()
  const { theme } = useChatTheme()

  if (loading) {
    return (
      <ChatRoot theme={theme} className="flex h-full items-center justify-center text-sm text-muted-foreground">
        Carregando…
      </ChatRoot>
    )
  }
  if (!session?.loggedIn) {
    return (
      <ChatRoot theme={theme} className="flex h-full items-center justify-center p-6">
        <LoginForm onLogin={login} error={error} />
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
    <form className="w-full max-w-sm rounded-xl border border-border bg-card p-5" onSubmit={submit}>
      <h1 className="mb-1 text-lg font-semibold">XVPN Chat</h1>
      <p className="mb-4 text-xs text-muted-foreground">Mesma conta do painel. Só vpn.officeempresa.com.</p>
      <label className="mb-1 block text-sm" htmlFor="user">
        Usuário
      </label>
      <ChatInput id="user" value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" />
      <label className="mb-1 mt-3 block text-sm" htmlFor="pass">
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
      <ChatButton type="submit" className="mt-4 w-full" disabled={busy}>
        {busy ? 'Entrando…' : 'Entrar'}
      </ChatButton>
    </form>
  )
}
