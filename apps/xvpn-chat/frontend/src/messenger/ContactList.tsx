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
  alignEnd,
}: {
  onSelect: (key: string) => void
  compact?: boolean
  alignEnd?: boolean
}) {
  const { contacts, popouts, activeKey, presence, unread, query, setQuery, session } = useChat()

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="p-2">
        <input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Buscar conversas"
          aria-label="Buscar conversas"
          className={cn(
            'h-9 w-full rounded-[14px] border-0 bg-foreground/[0.06] px-3 text-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
            alignEnd && 'text-right',
          )}
        />
      </div>
      <ul className="min-h-0 flex-1 overflow-y-auto px-1.5 pb-2" role="listbox" aria-label="Contatos">
        {contacts.map((c) => {
          const st =
            c.kind === 'dm' && c.peerUserId
              ? (presence[c.peerUserId] ?? 'offline')
              : c.kind === 'group'
                ? 'online'
                : 'offline'
          const n = unread[c.key] ?? 0
          const selected = popouts.some((p) => p.key === c.key) || activeKey === c.key
          return (
            <li key={c.key} className="mb-1.5">
              <button
                type="button"
                role="option"
                aria-selected={selected}
                onClick={() => onSelect(c.key)}
                className={cn(
                  'flex w-full items-center gap-2 rounded-[18px] px-3 py-2.5 hover:bg-white/8',
                  alignEnd ? 'flex-row-reverse text-right' : 'text-left',
                  selected && 'bg-white/10 ring-1 ring-[var(--safe)]/45',
                  compact && 'py-1.5',
                )}
              >
                <span className="relative shrink-0">
                  <span className="flex size-9 items-center justify-center rounded-full bg-primary/20 text-xs font-semibold text-primary">
                    {c.kind === 'group' ? <UsersRound className="size-4" /> : initials(c.title)}
                  </span>
                  {c.kind === 'dm' && (
                    <StatusDot
                      status={st}
                      className={cn('absolute -bottom-0.5 size-2.5', alignEnd ? '-left-0.5' : '-right-0.5')}
                    />
                  )}
                </span>
                <span className="min-w-0 flex-1">
                  <span className={cn('flex items-center gap-2', alignEnd && 'flex-row-reverse')}>
                    <span className="truncate font-display text-sm font-medium">{c.title}</span>
                    <span className="shrink-0 font-display text-[10px] text-muted-foreground">{formatTime(c.lastAt)}</span>
                  </span>
                  <span className="block truncate text-xs text-muted-foreground">
                    {c.lastBody || (c.kind === 'group' ? 'grupo' : session?.username === c.title ? '' : 'sem mensagens')}
                  </span>
                </span>
                {n > 0 && (
                  <span className="flex size-5 items-center justify-center rounded-full bg-[var(--safe)] text-[10px] font-bold text-[var(--safe-foreground)]">
                    {n > 9 ? '9+' : n}
                  </span>
                )}
              </button>
            </li>
          )
        })}
        {contacts.length === 0 && (
          <li className={cn('px-3 py-6 text-xs text-muted-foreground', alignEnd ? 'text-right' : 'text-center')}>
            Nenhuma conversa ainda.
          </li>
        )}
      </ul>
    </div>
  )
}
