import { ImagePlus, Plus, X } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import {
  api,
  ApiError,
  type SocialStoryAuthor,
  type SocialStoryItem,
} from '@/lib/api'
import { useAuth } from '@/lib/auth-context'
import { usePollingData } from '@/hooks/use-polling-data'
import { SocialAvatar } from '@/components/social-avatar'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'

export function SocialStoriesRail({
  filterAuthorId,
  allowCompose = true,
  className,
}: {
  filterAuthorId?: number
  allowCompose?: boolean
  className?: string
}) {
  const { user } = useAuth()
  const fetchStories = useCallback(() => api.listSocialStories(), [])
  const { data, reload } = usePollingData(fetchStories, 20_000)
  const [composer, setComposer] = useState(false)
  const [viewer, setViewer] = useState<SocialStoryAuthor | null>(null)

  const authors = (data?.items ?? []).filter((a) =>
    filterAuthorId == null ? true : a.author_id === filterAuthorId,
  )
  const empty = authors.length === 0
  if (empty && !allowCompose) return null

  return (
    <div className={cn('watch-complication rounded-[22px] px-4 py-3', className)}>
      <div className="mb-2.5 flex items-baseline justify-between gap-3">
        <p className="hud-label text-muted-foreground/70">Status</p>
        {empty && (
          <p className="text-[11px] text-muted-foreground">
            {filterAuthorId != null ? 'Nenhum status nas últimas 24 h' : 'Ninguém publicou nas últimas 24 h'}
          </p>
        )}
      </div>
      <div className="flex gap-3 overflow-x-auto pb-0.5">
        {allowCompose && (
          <button
            type="button"
            onClick={() => setComposer(true)}
            className="flex w-16 shrink-0 flex-col items-center gap-1.5"
          >
            <span className="flex size-14 items-center justify-center rounded-full bg-gradient-to-b from-white/14 to-white/6 ring-2 ring-white/12">
              <Plus className="size-5" strokeWidth={2.25} />
            </span>
            <span className="w-full truncate text-center text-[11px] text-muted-foreground">Novo</span>
          </button>
        )}
        {authors.map((s) => (
          <button
            key={s.author_id}
            type="button"
            onClick={() => setViewer(s)}
            className="flex w-16 shrink-0 flex-col items-center gap-1.5"
          >
            <span
              className={cn(
                'rounded-full p-[3px]',
                s.unseen ? 'bg-[var(--safe)]' : 'bg-white/20',
              )}
            >
              <SocialAvatar
                name={s.username}
                src={s.avatar_url}
                className="size-12 bg-background text-sm"
              />
            </span>
            <span className="w-full truncate text-center text-[11px] text-muted-foreground">
              {user && s.username === user.username ? 'Você' : s.username}
            </span>
          </button>
        ))}
      </div>
      {composer && (
        <StoryComposer
          onClose={() => setComposer(false)}
          onPublished={() => {
            setComposer(false)
            reload()
          }}
        />
      )}
      {viewer && (
        <StoryViewer
          author={viewer}
          onClose={() => setViewer(null)}
          onViewed={() => reload()}
        />
      )}
    </div>
  )
}

function StoryComposer({ onClose, onPublished }: { onClose: () => void; onPublished: () => void }) {
  const [text, setText] = useState('')
  const [photo, setPhoto] = useState<File | null>(null)
  const [preview, setPreview] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
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
      toast.error('Escreva um texto ou escolha uma foto.')
      return
    }
    setBusy(true)
    try {
      if (photo) {
        const att = await api.uploadSocialAttachment(photo)
        await api.createSocialStory(text.trim(), { kind: 'image', attachment_id: att.id })
      } else {
        await api.createSocialStory(text.trim(), { kind: 'text' })
      }
      toast.success('Status publicado')
      onPublished()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao publicar status')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="fixed inset-0 z-[55] flex items-center justify-center bg-black/75 p-4" onClick={onClose}>
      <div className="relative w-full max-w-sm rounded-[22px] p-5 watch-complication" onClick={(e) => e.stopPropagation()}>
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
        <Textarea value={text} onChange={(e) => setText(e.target.value.slice(0, 280))} placeholder="Escreva um status…" rows={2} />
        <div className="mt-3 flex gap-2">
          <Button type="button" variant="outline" className="flex-1 rounded-full" onClick={() => fileRef.current?.click()}>
            <ImagePlus className="size-4" />
            Foto
          </Button>
          <Button type="button" className="flex-1 rounded-full" disabled={busy} onClick={() => void publish()}>
            {busy ? 'Publicando…' : 'Publicar'}
          </Button>
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

function StoryViewer({
  author,
  onClose,
  onViewed,
}: {
  author: SocialStoryAuthor
  onClose: () => void
  onViewed: () => void
}) {
  const [idx, setIdx] = useState(0)
  const [url, setUrl] = useState<string | null>(null)
  const item: SocialStoryItem | undefined = author.items[idx]
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
    void api.fetchSocialAttachment(attachmentId).then((b) => {
      if (alive) setUrl(URL.createObjectURL(b))
    })
    return () => {
      alive = false
    }
  }, [attachmentId])

  if (!item) return null
  const current = item

  async function next() {
    try {
      await api.viewSocialStory(current.id)
      onViewed()
    } catch {
      // visualização é best-effort
    }
    if (idx + 1 >= author.items.length) onClose()
    else setIdx((i) => i + 1)
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
          <button
            type="button"
            className="absolute inset-y-0 left-0 w-1/3"
            aria-label="Anterior"
            onClick={() => idx > 0 && setIdx((i) => i - 1)}
          />
          <button type="button" className="absolute inset-y-0 right-0 w-2/3" aria-label="Próximo" onClick={() => void next()} />
          {item.kind === 'image' && url ? (
            <img src={url} alt="" className="pointer-events-none max-h-[22rem] w-full object-contain" />
          ) : (
            <p className="pointer-events-none font-display text-xl font-semibold text-white">{item.body}</p>
          )}
        </div>
        <p className="pb-4 text-center font-display text-[11px] text-white/50">@{author.username}</p>
      </div>
    </div>
  )
}
