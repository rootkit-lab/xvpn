import { UsersRound } from 'lucide-react'
import { cn } from '@chat/lib/utils'
import { useChat } from '@chat/messenger/ChatProvider'
import { initials, StatusDot } from '@chat/messenger/StatusDot'
import { ChatRoot } from '@chat/messenger/ui'

/** Barra inferior com as contas/conversas já existentes. */
export function ChatAccountsBar() {
  const { session, contacts, activeKey, setActiveKey, presence, unread } = useChat()

  if (!session?.loggedIn) return null

  return (
    <ChatRoot
      theme="inherit"
      className="flex h-12 shrink-0 items-center gap-1 overflow-x-auto border-t border-white/8 bg-card/70 px-2 backdrop-blur-xl"
    >
      {contacts.length === 0 && (
        <p className="px-2 text-[11px] text-muted-foreground">Nenhuma conta ainda — abra o Chat para iniciar.</p>
      )}
      {contacts.map((c) => {
        const st =
          c.kind === 'dm' && c.peerUserId ? (presence[c.peerUserId] ?? 'offline') : c.kind === 'group' ? 'online' : 'offline'
        const n = unread[c.key] ?? 0
        const selected = activeKey === c.key
        return (
          <button
            key={c.key}
            type="button"
            onClick={() => setActiveKey(c.key)}
            className={cn(
              'relative flex shrink-0 items-center gap-2 rounded-full border px-2 py-1 text-left transition-colors',
              selected
                ? 'border-primary/40 bg-primary/15 text-foreground'
                : 'border-white/8 bg-white/5 text-muted-foreground hover:bg-white/10 hover:text-foreground',
            )}
            aria-pressed={selected}
            aria-label={`Abrir conversa com ${c.title}`}
          >
            <span className="relative shrink-0">
              <span className="flex size-7 items-center justify-center rounded-full bg-primary/20 text-[10px] font-semibold text-primary">
                {c.kind === 'group' ? <UsersRound className="size-3.5" /> : initials(c.title)}
              </span>
              {c.kind === 'dm' && <StatusDot status={st} className="absolute -bottom-0.5 -right-0.5 size-2 ring-1" />}
            </span>
            <span className="max-w-[7rem] truncate text-xs font-medium">{c.title}</span>
            {n > 0 && (
              <span className="flex h-4 min-w-4 items-center justify-center rounded-full bg-primary px-1 text-[10px] font-bold text-primary-foreground">
                {n > 9 ? '9+' : n}
              </span>
            )}
          </button>
        )
      })}
    </ChatRoot>
  )
}
