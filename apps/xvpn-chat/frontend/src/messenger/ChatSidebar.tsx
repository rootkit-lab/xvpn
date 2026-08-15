import { MessageCircle, X } from 'lucide-react'
import { useChat } from '@chat/messenger/ChatProvider'
import { ContactList } from '@chat/messenger/ContactList'
import { NewChatDialog } from '@chat/messenger/NewChatDialog'
import { StatusDot } from '@chat/messenger/StatusDot'
import { ChatRoot } from '@chat/messenger/ui'
import { useState } from 'react'
import { cn } from '@chat/lib/utils'
import type { PresenceStatus } from '@chat/chatapi/types'

const STATUSES: Exclude<PresenceStatus, 'offline'>[] = ['online', 'away', 'dnd', 'invisible']
const STATUS_LABEL: Record<string, string> = {
  online: 'Online',
  away: 'Ausente',
  dnd: 'Ocupado',
  invisible: 'Invisível',
}

/** Lista de contatos no aside direito — a conversa abre no modal. */
export function ChatSidebar() {
  const { session, setDockOpen, setActiveKey, myStatus, setMyStatus } = useChat()
  const [statusOpen, setStatusOpen] = useState(false)

  if (!session?.loggedIn) return null

  return (
    <ChatRoot theme="inherit" className="flex h-full min-h-0 flex-col">
      <header className="flex shrink-0 items-center gap-2 border-b border-white/8 px-3 py-2">
        <MessageCircle className="size-4 text-primary" aria-hidden />
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-semibold tracking-tight">Contatos</p>
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
        <button
          type="button"
          className="inline-flex size-8 items-center justify-center rounded-lg text-muted-foreground hover:bg-white/5 hover:text-foreground"
          onClick={() => setDockOpen(false)}
          aria-label="Fechar lista de contatos"
        >
          <X className="size-4" />
        </button>
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
      <NewChatDialog />
      <div className="min-h-0 flex-1">
        <ContactList onSelect={(k) => setActiveKey(k)} />
      </div>
    </ChatRoot>
  )
}
