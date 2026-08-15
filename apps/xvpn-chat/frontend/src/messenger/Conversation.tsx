import { useEffect, useRef, useState, type FormEvent, type KeyboardEvent } from 'react'
import { cn } from '@chat/lib/utils'
import { useChat } from '@chat/messenger/ChatProvider'
import { ChatButton, ChatInput } from '@chat/messenger/ui'

function dayLabel(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleDateString('pt-BR', { day: '2-digit', month: 'short' })
}

export function Conversation({
  threadKey,
  onClose,
  alignEnd,
}: {
  threadKey: string
  onClose?: () => void
  alignEnd?: boolean
}) {
  const { messages, typing, send, session, contactByKey, api, activeKey } = useChat()
  const contact = contactByKey(threadKey)
  const list = messages[threadKey] ?? []
  const [body, setBody] = useState('')
  const bottom = useRef<HTMLDivElement>(null)

  useEffect(() => {
    bottom.current?.scrollIntoView({ block: 'end' })
  }, [list.length, threadKey])

  if (!contact) {
    return <p className="p-4 text-sm text-muted-foreground">Selecione um contato.</p>
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    const text = body.trim()
    if (!text) return
    setBody('')
    await send(text, threadKey)
  }

  function onKey(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      void (async () => {
        const text = body.trim()
        if (!text) return
        setBody('')
        await send(text, threadKey)
      })()
    }
  }

  let lastDay = ''

  return (
    <div className="flex h-full min-h-0 flex-col">
      <header
        className={cn(
          'flex items-center gap-2 border-b border-border px-3 py-2',
          alignEnd ? 'flex-row-reverse' : 'justify-between',
        )}
      >
        <div className={cn('min-w-0 flex-1', alignEnd && 'text-right')}>
          <p className="truncate text-sm font-semibold">{contact.title}</p>
          {typing[threadKey] && <p className="text-[11px] text-muted-foreground">digitando…</p>}
        </div>
        {onClose && (
          <ChatButton variant="ghost" className="size-8 shrink-0 px-0" aria-label="Fechar conversa" onClick={onClose}>
            ×
          </ChatButton>
        )}
      </header>
      <div className="min-h-0 flex-1 overflow-y-auto bg-background/40 px-3 py-2">
        {list.map((m) => {
          const day = dayLabel(m.created_at)
          const showDay = day !== lastDay
          lastDay = day
          const mine = Number(m.author_id) === Number(session?.userId)
          return (
            <div key={m.id}>
              {showDay && <p className="my-2 text-center text-[10px] uppercase tracking-wide text-muted-foreground">{day}</p>}
              <div className={cn('mb-1.5 flex', mine ? 'justify-end' : 'justify-start')}>
                <p
                  className={cn(
                    'max-w-[80%] break-words px-3 py-1.5 text-sm leading-snug shadow-sm',
                    mine
                      ? 'rounded-2xl rounded-br-md bg-primary text-primary-foreground'
                      : 'rounded-2xl rounded-bl-md border border-white/8 bg-muted text-foreground',
                  )}
                >
                  {m.body}
                </p>
              </div>
            </div>
          )
        })}
        <div ref={bottom} />
      </div>
      <form className="flex gap-2 border-t border-border p-2" onSubmit={onSubmit}>
        <ChatInput
          value={body}
          onChange={(e) => {
            setBody(e.target.value)
            if (contact && activeKey === threadKey) api.sendTyping(contact.kind, contact.id)
          }}
          onKeyDown={onKey}
          placeholder="Mensagem"
          aria-label="Mensagem"
          autoComplete="off"
        />
        <ChatButton type="submit">Enviar</ChatButton>
      </form>
    </div>
  )
}
