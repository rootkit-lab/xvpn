import { useEffect, useState } from 'react'
import { ChevronLeft, MessageCircle } from 'lucide-react'
import { cn } from '@chat/lib/utils'
import type { PresenceStatus } from '@chat/chatapi/types'
import { useChat } from '@chat/messenger/ChatProvider'
import { ContactList } from '@chat/messenger/ContactList'
import { Conversation } from '@chat/messenger/Conversation'
import { NewChatDialog } from '@chat/messenger/NewChatDialog'
import { StatusDot } from '@chat/messenger/StatusDot'
import { ChatRoot } from '@chat/messenger/ui'

const STATUSES: Exclude<PresenceStatus, 'offline'>[] = ['online', 'away', 'dnd', 'invisible']
const STATUS_LABEL: Record<string, string> = {
  online: 'Online',
  away: 'Ausente',
  dnd: 'Ocupado',
  invisible: 'Invisível',
}

/** Messenger no aside esquerdo do SystemChrome — acionado pela status bar. */
export function ChatSidebar() {
  const { session, setDockOpen, activeKey, setActiveKey, myStatus, setMyStatus } = useChat()
  const [statusOpen, setStatusOpen] = useState(false)

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key !== 'Escape') return
      if (activeKey) {
        setActiveKey(null)
        return
      }
      setDockOpen(false)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [activeKey, setActiveKey, setDockOpen])

  if (!session?.loggedIn) return null

  return (
    <ChatRoot theme="inherit" className="flex min-h-0 flex-1 flex-col">
      <header className="flex shrink-0 items-center gap-1 border-b border-white/8 px-2 py-2">
        <button
          type="button"
          className="inline-flex size-8 items-center justify-center rounded-lg text-muted-foreground hover:bg-white/5 hover:text-foreground"
          onClick={() => setDockOpen(false)}
          aria-label="Voltar ao menu"
        >
          <ChevronLeft className="size-4" />
        </button>
        <MessageCircle className="size-4 text-primary" aria-hidden />
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-semibold tracking-tight">Chat</p>
          <button
            type="button"
            className="flex items-center gap-1 text-[11px] text-muted-foreground hover:text-foreground"
            onClick={() => setStatusOpen((v) => !v)}
            aria-expanded={statusOpen}
            aria-haspopup="listbox"
          >
            <StatusDot status={myStatus === 'invisible' ? 'offline' : myStatus} className="size-2 ring-0" />
            {STATUS_LABEL[myStatus]}
          </button>
        </div>
      </header>
      {statusOpen && (
        <div className="flex flex-wrap gap-1 border-b border-white/8 p-2" role="listbox" aria-label="Status">
          {STATUSES.map((s) => (
            <button
              key={s}
              type="button"
              role="option"
              aria-selected={myStatus === s}
              className={cn(
                'rounded-md px-2 py-1 text-[11px]',
                myStatus === s ? 'bg-primary/15 text-primary' : 'text-muted-foreground hover:bg-white/5 hover:text-foreground',
              )}
              onClick={() => {
                setMyStatus(s)
                setStatusOpen(false)
              }}
            >
              {STATUS_LABEL[s]}
            </button>
          ))}
        </div>
      )}
      {activeKey ? (
        <div className="min-h-0 flex-1">
          <Conversation threadKey={activeKey} onClose={() => setActiveKey(null)} />
        </div>
      ) : (
        <>
          <NewChatDialog />
          <div className="min-h-0 flex-1">
            <ContactList onSelect={(k) => setActiveKey(k)} />
          </div>
        </>
      )}
    </ChatRoot>
  )
}
