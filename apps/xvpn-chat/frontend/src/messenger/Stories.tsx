import { ImagePlus, Plus, X } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useChat } from '@chat/messenger/ChatProvider'
import { initials } from '@chat/messenger/StatusDot'
import { ChatButton, ChatInput } from '@chat/messenger/ui'
import { cn } from '@chat/lib/utils'
import type { StoryAuthor, StoryItem } from '@chat/chatapi/types'

export function StoriesRail() {
  const { stories, session } = useChat()
  const [composer, setComposer] = useState(false)
  const [viewer, setViewer] = useState<StoryAuthor | null>(null)

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
      {composer && <StoryComposer onClose={() => setComposer(false)} />}
      {viewer && <StoryViewer author={viewer} onClose={() => setViewer(null)} />}
    </div>
  )
}

function StoryComposer({ onClose }: { onClose: () => void }) {
  const { publishStory } = useChat()
  const [text, setText] = useState('')
  const [photo, setPhoto] = useState<File | null>(null)
  const [preview, setPreview] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const fileRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [onClose])

  useEffect(() => {
    if (!photo) {
      setPreview(null)
      return
    }
    const url = URL.createObjectURL(photo)
    setPreview(url)
    return () => URL.revokeObjectURL(url)
  }, [photo])

  async function publish() {
    if (!text.trim() && !photo) {
      setErr('Escreva um texto ou escolha uma foto.')
      return
    }
    setBusy(true)
    setErr(null)
    try {
      await publishStory(text.trim(), photo ?? undefined)
      onClose()
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="fixed inset-0 z-[55] flex items-center justify-center bg-black/75 p-4" onClick={onClose}>
      <div
        className="relative w-full max-w-sm rounded-[22px] p-5 watch-complication"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="mb-3 flex items-center justify-between">
          <h2 className="font-display text-lg font-semibold">Novo status</h2>
          <button
            type="button"
            className="inline-flex size-8 items-center justify-center rounded-[10px] text-muted-foreground hover:bg-white/10 hover:text-foreground"
            aria-label="Fechar"
            onClick={onClose}
          >
            <X className="size-4" />
          </button>
        </div>
        <div className="mb-3 flex min-h-[12rem] items-center justify-center overflow-hidden rounded-[18px] bg-black/40">
          {preview ? (
            <img src={preview} alt="" className="max-h-64 w-full object-contain" />
          ) : (
            <p className="px-4 text-center font-display text-xl font-semibold">
              {text.trim() || 'O que você está pensando?'}
            </p>
          )}
        </div>
        <ChatInput
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder="Escreva um status…"
          aria-label="Texto do status"
        />
        {err && <p className="mt-2 text-sm text-destructive">{err}</p>}
        <div className="mt-3 flex gap-1.5">
          <ChatButton variant="outline" className="flex-1" onClick={() => fileRef.current?.click()}>
            <ImagePlus className="size-4" />
            Foto
          </ChatButton>
          <ChatButton variant="safe" className="flex-1" disabled={busy} onClick={() => void publish()}>
            {busy ? 'Publicando…' : 'Publicar'}
          </ChatButton>
        </div>
        <input
          ref={fileRef}
          type="file"
          accept="image/*"
          className="hidden"
          onChange={(e) => {
            const f = e.target.files?.[0]
            e.target.value = ''
            if (f) setPhoto(f)
          }}
        />
      </div>
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
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [onClose])

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

  function prev() {
    if (idx === 0) return
    setIdx((i) => i - 1)
  }

  return (
    <div className="fixed inset-0 z-[55] flex items-center justify-center bg-black/80 p-4" onClick={onClose}>
      <div
        className="relative w-full max-w-sm overflow-hidden rounded-[22px] bg-black watch-complication"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex gap-1 p-2">
          {author.items.map((s, i) => (
            <span key={s.id} className={cn('h-0.5 flex-1 rounded-full', i <= idx ? 'bg-[var(--safe)]' : 'bg-white/20')} />
          ))}
        </div>
        <button
          type="button"
          className="absolute right-3 top-4 z-10 inline-flex size-8 items-center justify-center rounded-full bg-black/40 text-white/80"
          aria-label="Fechar status"
          onClick={onClose}
        >
          <X className="size-4" />
        </button>
        <div className="relative flex min-h-[22rem] w-full items-center justify-center p-6">
          <button type="button" className="absolute inset-y-0 left-0 w-1/3" aria-label="Anterior" onClick={prev} />
          <button type="button" className="absolute inset-y-0 right-0 w-2/3" aria-label="Próximo" onClick={next} />
          {item.kind === 'image' && url ? (
            <img src={url} alt="" className="pointer-events-none max-h-[22rem] w-full object-contain" />
          ) : (
            <p className="pointer-events-none font-display text-xl font-semibold text-white">{item.body}</p>
          )}
        </div>
        <p className="pb-4 text-center font-display text-[11px] text-white/50">{author.username}</p>
      </div>
    </div>
  )
}
