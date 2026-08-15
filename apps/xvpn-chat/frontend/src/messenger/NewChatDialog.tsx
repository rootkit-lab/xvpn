import { useState, type FormEvent } from 'react'
import { useChat } from '@chat/messenger/ChatProvider'
import { ChatButton, ChatInput } from '@chat/messenger/ui'

export function NewChatDialog({ alignEnd }: { alignEnd?: boolean }) {
  const { people, searchPeople, startDM, error } = useChat()
  const [open, setOpen] = useState(false)
  const [q, setQ] = useState('')

  async function onSearch(e: FormEvent) {
    e.preventDefault()
    await searchPeople(q.trim())
  }

  return (
    <div className="border-b border-border p-2">
      <ChatButton variant="outline" className="w-full" onClick={() => setOpen((v) => !v)}>
        Nova conversa
      </ChatButton>
      {open && (
        <div className="mt-2 rounded-md border border-border p-2">
          <form className={alignEnd ? 'flex flex-row-reverse gap-1' : 'flex gap-1'} onSubmit={onSearch}>
            <ChatInput
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="username"
              aria-label="Buscar membro"
              className={alignEnd ? 'text-right' : undefined}
            />
            <ChatButton type="submit">Buscar</ChatButton>
          </form>
          {error && <p className={`mt-1 text-xs text-destructive ${alignEnd ? 'text-right' : ''}`}>{error}</p>}
          <ul className="mt-1 max-h-32 overflow-y-auto">
            {people.map((p) => (
              <li key={p.user_id}>
                <button
                  type="button"
                  className={`w-full rounded px-2 py-1 text-sm hover:bg-secondary ${alignEnd ? 'text-right' : 'text-left'}`}
                  onClick={async () => {
                    await startDM(p.username)
                    setOpen(false)
                  }}
                >
                  {p.display_name || p.username}{' '}
                  <span className="text-xs text-muted-foreground">@{p.username}</span>
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}
