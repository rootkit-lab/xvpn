import { Plus } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useChat } from '@chat/messenger/ChatProvider'
import { initials } from '@chat/messenger/StatusDot'
import { ChatButton, ChatInput } from '@chat/messenger/ui'
import { cn } from '@chat/lib/utils'
import type { StoryAuthor, StoryItem } from '@chat/chatapi/types'

export function StoriesRail() {
  const { stories, session, publishStory } = useChat()
  const [composer, setComposer] = useState(false)
  const [viewer, setViewer] = useState<StoryAuthor | null>(null)
  const [text, setText] = useState('')
  const fileRef = useRef<HTMLInputElement>(null)

  return (
    <div className="shrink-0 border-b border-white/8 px-2 py-2">
      <div className="flex gap-2 overflow-x-auto pb-1">
        <button
          type="button"
          onClick={() => setComposer(true)}
          className="flex w-14 shrink-0 flex-col items-center gap-1"
        >
          <span className="relative flex size-12 items-center justify-center rounded-full bg-gradient-to-b from-white/14 to-white/6 text-foreground">
            <Plus className="size-5" strokeWidth={2.25} />
          </span>
          <span className="w-full truncate text-center font-display text-[10px] text-muted-foreground">Seu status</span>
        </button>
        {stories.map((s) => (
          <button
            key={s.author_id}
            type="button"
            onClick={() => setViewer(s)}
            className="flex w-14 shrink-0 flex-col items-center gap-1"
          >
            <span
              className={cn(
                'flex size-12 items-center justify-center rounded-full text-[11px] font-semibold',
                s.unseen
                  ? 'bg-[var(--safe)]/15 text-[var(--safe)] ring-2 ring-[var(--safe)]'
                  : 'bg-white/8 text-muted-foreground ring-2 ring-white/15',
              )}
            >
              {initials(s.username)}
            </span>
            <span className="w-full truncate text-center font-display text-[10px] text-muted-foreground">
              {s.author_id === session?.userId ? 'Você' : s.username}
            </span>
          </button>
        ))}
      </div>
      {composer && (
        <div className="mt-2 rounded-[16px] p-2 watch-complication">
          <ChatInput
            value={text}
            onChange={(e) => setText(e.target.value)}
            placeholder="Escreva um status…"
            className="h-9"
          />
          <div className="mt-2 flex gap-1.5">
            <ChatButton className="h-8 px-3 text-xs" variant="outline" onClick={() => fileRef.current?.click()}>
              Foto
            </ChatButton>
            <ChatButton
              variant="safe"
              className="h-8 px-3 text-xs"
              onClick={async () => {
                if (!text.trim()) return
                await publishStory(text.trim())
                setText('')
                setComposer(false)
              }}
            >
              Publicar
            </ChatButton>
            <ChatButton variant="ghost" className="h-8 px-3 text-xs" onClick={() => setComposer(false)}>
              Cancelar
            </ChatButton>
          </div>
          <input
            ref={fileRef}
            type="file"
            accept="image/*"
            className="hidden"
            onChange={async (e) => {
              const f = e.target.files?.[0]
              e.target.value = ''
              if (!f) return
              await publishStory(text, f)
              setText('')
              setComposer(false)
            }}
          />
        </div>
      )}
      {viewer && <StoryViewer author={viewer} onClose={() => setViewer(null)} />}
    </div>
  )
}

function StoryViewer({ author, onClose }: { author: StoryAuthor; onClose: () => void }) {
  const { viewStory, api } = useChat()
  const [idx, setIdx] = useState(0)
  const [url, setUrl] = useState<string | null>(null)
  const item: StoryItem | undefined = author.items[idx]
  const attachmentId = item?.attachment_id

  useEffect(() => {
    if (!attachmentId) {
      setUrl(null)
      return
    }
    let alive = true
    void api.fetchAttachment(attachmentId).then((b) => {
      if (alive) setUrl(URL.createObjectURL(b))
    })
    return () => {
      alive = false
    }
  }, [api, attachmentId])

  if (!item) return null
  const current = item

  function next() {
    void viewStory(current.id)
    if (idx + 1 >= author.items.length) onClose()
    else setIdx((i) => i + 1)
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4" onClick={onClose}>
      <div
        className="relative w-full max-w-sm overflow-hidden rounded-[22px] bg-black watch-complication"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex gap-1 p-2">
          {author.items.map((s, i) => (
            <span key={s.id} className={cn('h-0.5 flex-1 rounded-full', i <= idx ? 'bg-[var(--safe)]' : 'bg-white/20')} />
          ))}
        </div>
        <button type="button" className="absolute right-3 top-4 text-sm text-white/70" onClick={onClose}>
          Fechar
        </button>
        <button type="button" className="flex min-h-[22rem] w-full flex-col items-center justify-center p-6" onClick={next}>
          {item.kind === 'image' && url ? (
            <img src={url} alt="" className="max-h-[22rem] w-full object-contain" />
          ) : (
            <p className="font-display text-xl font-semibold text-white">{item.body}</p>
          )}
          <p className="mt-4 font-display text-[11px] text-white/50">{author.username}</p>
        </button>
      </div>
    </div>
  )
}
