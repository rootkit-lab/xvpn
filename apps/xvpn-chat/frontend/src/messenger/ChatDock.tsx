import { useMemo } from 'react'
import { cn } from '@chat/lib/utils'
import { useChat } from '@chat/messenger/ChatProvider'
import { ContactList } from '@chat/messenger/ContactList'
import { Conversation } from '@chat/messenger/Conversation'
import { ChatRoot } from '@chat/messenger/ui'
import { useChatTheme } from '@chat/theme/ThemeProvider'

function Flower() {
  return (
    <span className="xvpn-chat-flower" aria-hidden>
      <span />
      <span />
      <span />
      <span />
    </span>
  )
}

export function ChatDock({ hidden }: { hidden?: boolean }) {
  const { theme } = useChatTheme()
  const { session, dockOpen, setDockOpen, dockWindows, openDockWindow, closeDockWindow, unread, contactByKey } =
    useChat()

  const totalUnread = useMemo(() => Object.values(unread).reduce((a, b) => a + b, 0), [unread])

  if (hidden || !session?.loggedIn) return null

  return (
    <ChatRoot theme={theme} className="pointer-events-none fixed inset-x-0 bottom-0 z-50 flex justify-end gap-2 p-3">
      {dockWindows.map((key) => {
        const c = contactByKey(key)
        if (!c) return null
        return (
          <div
            key={key}
            className={cn(
              'pointer-events-auto flex h-[22rem] w-[18rem] flex-col overflow-hidden rounded-t-xl border border-border bg-card shadow-2xl max-sm:fixed max-sm:inset-0 max-sm:h-auto max-sm:w-auto max-sm:rounded-none',
            )}
          >
            <Conversation threadKey={key} onClose={() => closeDockWindow(key)} />
          </div>
        )
      })}
      <div className="pointer-events-auto flex flex-col items-end">
        {dockOpen && (
          <div className="mb-2 h-[22rem] w-[17rem] overflow-hidden rounded-xl border border-border bg-card shadow-2xl">
            <ContactList compact onSelect={(k) => openDockWindow(k)} />
          </div>
        )}
        <button
          type="button"
          className="relative flex h-12 items-center gap-2 rounded-full border border-border bg-card px-4 shadow-lg hover:bg-secondary"
          onClick={() => setDockOpen(!dockOpen)}
          aria-label={dockOpen ? 'Fechar chat' : 'Abrir chat'}
        >
          <Flower />
          <span className="text-sm font-medium">Chat</span>
          {totalUnread > 0 && (
            <span className="absolute -right-1 -top-1 flex size-5 items-center justify-center rounded-full bg-primary text-[10px] font-bold text-primary-foreground">
              {totalUnread > 9 ? '9+' : totalUnread}
            </span>
          )}
        </button>
      </div>
    </ChatRoot>
  )
}

