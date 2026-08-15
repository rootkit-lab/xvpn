import { UsersRound } from 'lucide-react'
import { cn } from '@chat/lib/utils'
import { useChat } from '@chat/messenger/ChatProvider'
import { initials, StatusDot } from '@chat/messenger/StatusDot'

/** Rodapé do rail direito — contas/conversas já existentes, alinhadas à direita. */
export function ChatAccountsBar() {
  const { session, contacts, activeKey, setActiveKey, presence, unread } = useChat()

  if (!session?.loggedIn) return null

  return (
    <div className="flex h-12 shrink-0 flex-row-reverse items-center gap-1 overflow-x-auto border-t border-white/8 bg-card px-2">
      {contacts.length === 0 && (
        <p className="px-2 text-right text-[11px] text-muted-foreground">Nenhuma conta ainda.</p>
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
              'relative flex shrink-0 flex-row-reverse items-center gap-2 rounded-full border px-2 py-1 text-right transition-colors',
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
              {c.kind === 'dm' && <StatusDot status={st} className="absolute -bottom-0.5 -left-0.5 size-2 ring-1" />}
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
    </div>
  )
}
