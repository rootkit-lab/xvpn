import { UsersRound } from 'lucide-react'
import { cn } from '@chat/lib/utils'
import { useChat } from '@chat/messenger/ChatProvider'
import { initials, StatusDot } from '@chat/messenger/StatusDot'

function formatTime(iso?: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' })
}

export function ContactList({
  onSelect,
  compact,
}: {
  onSelect: (key: string) => void
  compact?: boolean
}) {
  const { contacts, activeKey, presence, unread, query, setQuery, session } = useChat()

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="p-2">
        <input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Buscar conversas"
          aria-label="Buscar conversas"
          className="h-8 w-full rounded-md border border-input bg-transparent px-2 text-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        />
      </div>
      <ul className="min-h-0 flex-1 overflow-y-auto" role="listbox" aria-label="Contatos">
        {contacts.map((c) => {
          const st =
            c.kind === 'dm' && c.peerUserId
              ? (presence[c.peerUserId] ?? 'offline')
              : c.kind === 'group'
                ? 'online'
                : 'offline'
          const n = unread[c.key] ?? 0
          return (
            <li key={c.key}>
              <button
                type="button"
                role="option"
                aria-selected={activeKey === c.key}
                onClick={() => onSelect(c.key)}
                className={cn(
                  'flex w-full items-center gap-2 px-3 py-2 text-left hover:bg-secondary/80',
                  activeKey === c.key && 'bg-secondary',
                  compact && 'py-1.5',
                )}
              >
                <span className="relative shrink-0">
                  <span className="flex size-9 items-center justify-center rounded-full bg-primary/20 text-xs font-semibold text-primary">
                    {c.kind === 'group' ? <UsersRound className="size-4" /> : initials(c.title)}
                  </span>
                  {c.kind === 'dm' && <StatusDot status={st} className="absolute -bottom-0.5 -right-0.5" />}
                </span>
                <span className="min-w-0 flex-1">
                  <span className="flex items-center justify-between gap-2">
                    <span className="truncate text-sm font-medium">{c.title}</span>
                    <span className="shrink-0 text-[10px] text-muted-foreground">{formatTime(c.lastAt)}</span>
                  </span>
                  <span className="block truncate text-xs text-muted-foreground">
                    {c.lastBody || (c.kind === 'group' ? 'grupo' : session?.username === c.title ? '' : 'sem mensagens')}
                  </span>
                </span>
                {n > 0 && (
                  <span className="flex size-5 items-center justify-center rounded-full bg-primary text-[10px] font-bold text-primary-foreground">
                    {n > 9 ? '9+' : n}
                  </span>
                )}
              </button>
            </li>
          )
        })}
        {contacts.length === 0 && (
          <li className="px-3 py-6 text-center text-xs text-muted-foreground">Nenhuma conversa ainda.</li>
        )}
      </ul>
    </div>
  )
}
