import { useState, type FormEvent } from 'react'
import { Plus, UsersRound } from 'lucide-react'
import { useChat } from '@chat/messenger/ChatProvider'
import { ChatButton, ChatInput } from '@chat/messenger/ui'
import { cn } from '@chat/lib/utils'

export function NewChatDialog({ alignEnd }: { alignEnd?: boolean }) {
  const { people, searchPeople, startDM, createGroupChat, error } = useChat()
  const [open, setOpen] = useState(false)
  const [tab, setTab] = useState<'dm' | 'group'>('dm')
  const [q, setQ] = useState('')
  const [groupName, setGroupName] = useState('')
  const [picked, setPicked] = useState<string[]>([])

  async function onSearch(e: FormEvent) {
    e.preventDefault()
    await searchPeople(q.trim())
  }

  return (
    <div className="p-2">
      <ChatButton variant="safe" className="w-full" onClick={() => setOpen((v) => !v)}>
        <Plus className="size-4" strokeWidth={2.25} />
        Nova conversa
      </ChatButton>
      {open && (
        <div className="mt-2 rounded-[18px] p-2 watch-complication">
          <div className={cn('mb-2 flex gap-1', alignEnd && 'flex-row-reverse')}>
            <ChatButton
              variant={tab === 'dm' ? 'default' : 'outline'}
              className="h-8 flex-1 px-2 text-xs"
              onClick={() => setTab('dm')}
            >
              Direta
            </ChatButton>
            <ChatButton
              variant={tab === 'group' ? 'default' : 'outline'}
              className="h-8 flex-1 px-2 text-xs"
              onClick={() => setTab('group')}
            >
              <UsersRound className="size-3.5" />
              Grupo
            </ChatButton>
          </div>
          {tab === 'group' && (
            <ChatInput
              value={groupName}
              onChange={(e) => setGroupName(e.target.value)}
              placeholder="Nome do grupo"
              className={cn('mb-2 h-9', alignEnd && 'text-right')}
            />
          )}
          <form className={alignEnd ? 'flex flex-row-reverse gap-1' : 'flex gap-1'} onSubmit={onSearch}>
            <ChatInput
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="username"
              aria-label="Buscar membro"
              className={cn('h-9', alignEnd ? 'text-right' : undefined)}
            />
            <ChatButton type="submit" className="h-9 px-3 text-xs">
              Buscar
            </ChatButton>
          </form>
          {error && <p className={`mt-1 text-xs text-destructive ${alignEnd ? 'text-right' : ''}`}>{error}</p>}
          <ul className="mt-1 max-h-32 overflow-y-auto">
            {people.map((p) => {
              const selected = picked.includes(p.username)
              return (
                <li key={p.user_id}>
                  <button
                    type="button"
                    className={`w-full rounded-[12px] px-2 py-1.5 text-sm hover:bg-white/8 ${alignEnd ? 'text-right' : 'text-left'} ${selected ? 'bg-white/10' : ''}`}
                    onClick={async () => {
                      if (tab === 'dm') {
                        await startDM(p.username)
                        setOpen(false)
                        return
                      }
                      setPicked((prev) =>
                        prev.includes(p.username) ? prev.filter((u) => u !== p.username) : [...prev, p.username],
                      )
                    }}
                  >
                    {p.display_name || p.username}{' '}
                    <span className="text-xs text-muted-foreground">@{p.username}</span>
                  </button>
                </li>
              )
            })}
          </ul>
          {tab === 'group' && (
            <ChatButton
              variant="safe"
              className="mt-2 h-8 w-full text-xs"
              disabled={!groupName.trim()}
              onClick={async () => {
                await createGroupChat(groupName.trim(), picked)
                setGroupName('')
                setPicked([])
                setOpen(false)
              }}
            >
              Criar grupo{picked.length ? ` (${picked.length})` : ''}
            </ChatButton>
          )}
        </div>
      )}
    </div>
  )
}
