import { useState } from 'react'
import { useChat } from '@chat/messenger/ChatProvider'
import { ContactList } from '@chat/messenger/ContactList'
import { Conversation } from '@chat/messenger/Conversation'
import { NewChatDialog } from '@chat/messenger/NewChatDialog'
import { StatusDot } from '@chat/messenger/StatusDot'
import { ChatButton, ChatRoot } from '@chat/messenger/ui'
import { useChatTheme } from '@chat/theme/ThemeProvider'
import type { ChatTheme, PresenceStatus } from '@chat/chatapi/types'

const STATUSES: Exclude<PresenceStatus, 'offline'>[] = ['online', 'away', 'dnd', 'invisible']
const THEMES: ChatTheme[] = ['icq', 'dark', 'light']

const STATUS_LABEL: Record<string, string> = {
  online: 'Online',
  away: 'Ausente',
  dnd: 'Ocupado',
  invisible: 'Invisível',
}

export function Messenger({ className }: { className?: string }) {
  const { session, myStatus, setMyStatus, setActiveKey, logout, mode, activeKey } = useChat()
  const { theme, setTheme } = useChatTheme()
  const [picker, setPicker] = useState(false)

  return (
    <ChatRoot theme={theme} className={className ?? 'flex h-full min-h-0 overflow-hidden'}>
      <aside className="flex w-72 shrink-0 flex-col border-r border-border bg-card">
        <div className="flex items-center gap-2 border-b border-border px-3 py-2">
          <button
            type="button"
            className="flex min-w-0 flex-1 items-center gap-2 rounded-md px-1 py-1 text-left hover:bg-secondary"
            onClick={() => setPicker((v) => !v)}
            aria-haspopup="listbox"
            aria-expanded={picker}
          >
            <StatusDot status={myStatus === 'invisible' ? 'offline' : myStatus} />
            <span className="truncate text-sm font-medium">{session?.username}</span>
          </button>
          {mode === 'desktop' && (
            <ChatButton variant="ghost" onClick={() => void logout()}>
              Sair
            </ChatButton>
          )}
        </div>
        {picker && (
          <div className="border-b border-border p-2 text-xs">
            <p className="mb-1 text-muted-foreground">Status</p>
            <div className="flex flex-wrap gap-1">
              {STATUSES.map((s) => (
                <ChatButton key={s} variant={myStatus === s ? 'default' : 'outline'} onClick={() => setMyStatus(s)}>
                  {STATUS_LABEL[s]}
                </ChatButton>
              ))}
            </div>
            <p className="mb-1 mt-2 text-muted-foreground">Tema</p>
            <div className="flex flex-wrap gap-1">
              {THEMES.map((t) => (
                <ChatButton key={t} variant={theme === t ? 'default' : 'outline'} onClick={() => setTheme(t)}>
                  {t}
                </ChatButton>
              ))}
            </div>
          </div>
        )}
        <NewChatDialog />
        <div className="min-h-0 flex-1">
          <ContactList onSelect={(k) => setActiveKey(k)} />
        </div>
      </aside>
      <section className="min-w-0 flex-1 bg-background">
        <Conversation threadKey={activeKey ?? ''} />
      </section>
    </ChatRoot>
  )
}
