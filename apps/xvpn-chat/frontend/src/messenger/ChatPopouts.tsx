import { UsersRound, X } from 'lucide-react'
import { useEffect, useMemo } from 'react'
import { cn } from '@chat/lib/utils'
import { useChat } from '@chat/messenger/ChatProvider'
import { Conversation } from '@chat/messenger/Conversation'
import { initials, StatusDot } from '@chat/messenger/StatusDot'
import { ChatRoot } from '@chat/messenger/ui'

/** Janelas de conversa no rodapé, estilo Facebook Messenger — sem overlay. */
export function ChatPopouts({ railOpen }: { railOpen: boolean }) {
  const {
    session,
    popouts,
    closePopout,
    minimizePopout,
    openPopout,
    setDockOpen,
    dockOpen,
    activeKey,
    presence,
    unread,
    contactByKey,
  } = useChat()

  const expanded = useMemo(() => popouts.filter((p) => !p.minimized), [popouts])
  const minimized = useMemo(() => popouts.filter((p) => p.minimized), [popouts])

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key !== 'Escape') return
      const focused = expanded.find((p) => p.key === activeKey) ?? expanded[expanded.length - 1]
      if (focused) {
        closePopout(focused.key)
        return
      }
      if (dockOpen) setDockOpen(false)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [activeKey, closePopout, dockOpen, expanded, setDockOpen])

  if (!session?.loggedIn) return null
  if (expanded.length === 0 && minimized.length === 0) return null

  return (
    <div
      className={cn(
        'pointer-events-none fixed z-40 flex flex-row-reverse items-end gap-2',
        'bottom-8',
        railOpen ? 'right-80 pr-2' : 'right-3',
      )}
    >
      {expanded.map((p, i) => (
        <ChatRoot
          key={p.key}
          theme="inherit"
          className={cn(
            'pointer-events-auto flex h-[26rem] w-[20.5rem] flex-col overflow-hidden rounded-t-[18px] border border-white/10 bg-card shadow-2xl',
            i < expanded.length - 1 && 'max-md:hidden',
            i < expanded.length - 2 && 'md:max-lg:hidden',
          )}
        >
          <Conversation
            threadKey={p.key}
            variant="popout"
            onClose={() => closePopout(p.key)}
            onMinimize={() => minimizePopout(p.key)}
          />
        </ChatRoot>
      ))}
      {minimized.map((p) => {
        const c = contactByKey(p.key)
        if (!c) return null
        const st =
          c.kind === 'dm' && c.peerUserId ? (presence[c.peerUserId] ?? 'offline') : c.kind === 'group' ? 'online' : 'offline'
        const n = unread[c.key] ?? 0
        return (
          <div key={p.key} className="pointer-events-auto relative">
            <button
              type="button"
              className="relative flex size-12 items-center justify-center rounded-full border border-white/10 bg-card shadow-lg transition-transform hover:scale-105"
              onClick={() => openPopout(p.key)}
              aria-label={`Restaurar conversa com ${c.title}`}
            >
              <span className="flex size-12 items-center justify-center rounded-full bg-primary/20 text-xs font-semibold text-primary">
                {c.kind === 'group' ? <UsersRound className="size-5" /> : initials(c.title)}
              </span>
              {c.kind === 'dm' && <StatusDot status={st} className="absolute bottom-0 left-0 size-2.5 ring-1" />}
              {n > 0 && (
                <span className="absolute -left-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-primary px-1 text-[10px] font-bold text-primary-foreground">
                  {n > 9 ? '9+' : n}
                </span>
              )}
            </button>
            <button
              type="button"
              className="absolute -right-1 -top-1 flex size-5 items-center justify-center rounded-full bg-card text-muted-foreground shadow hover:bg-destructive hover:text-white"
              onClick={() => closePopout(p.key)}
              aria-label={`Fechar conversa com ${c.title}`}
            >
              <X className="size-3" />
            </button>
          </div>
        )
      })}
    </div>
  )
}
