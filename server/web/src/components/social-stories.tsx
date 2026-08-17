import { ImagePlus, Plus, X } from 'lucide-react'
import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
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
    <>
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
                className={cn('rounded-full p-[3px]', s.unseen ? 'bg-[var(--profile-accent,var(--safe))]' : 'bg-white/20')}
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
    </>
  )
}

function StoryOverlay({
  onClose,
  children,
}: {
  onClose: () => void
  children: ReactNode
}) {
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose()
    }
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    document.addEventListener('keydown', onKey)
    return () => {
      document.body.style.overflow = prev
      document.removeEventListener('keydown', onKey)
    }
  }, [onClose])

  return createPortal(
    <div
      className="fixed inset-0 z-[80] flex items-center justify-center bg-black/80 p-3 sm:p-6"
      onClick={onClose}
    >
      {children}
    </div>,
    document.body,
  )
}

function StoryComposer({ onClose, onPublished }: { onClose: () => void; onPublished: () => void }) {
  const [text, setText] = useState('')
  const [photo, setPhoto] = useState<File | null>(null)
  const [preview, setPreview] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)

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
    <StoryOverlay onClose={onClose}>
      <div
        className="relative flex w-full max-w-sm flex-col gap-3 rounded-[22px] border border-white/10 bg-background p-5"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between">
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
        <div
          className="flex min-h-[14rem] items-center justify-center overflow-hidden rounded-[18px]"
          style={{
            background:
              'linear-gradient(165deg, var(--profile-accent, var(--primary)) 0%, color-mix(in oklch, var(--profile-accent, var(--primary)) 30%, black) 100%)',
          }}
        >
          {preview ? (
            <img src={preview} alt="" className="max-h-64 w-full object-contain" />
          ) : (
            <p className="px-4 text-center font-display text-xl font-semibold text-white">
              {text.trim() || 'O que você está pensando?'}
            </p>
          )}
        </div>
        <Textarea
          value={text}
          onChange={(e) => setText(e.target.value.slice(0, 280))}
          placeholder="Escreva um status…"
          rows={2}
        />
        <div className="flex gap-2">
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
    </StoryOverlay>
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
    if (!attachmentId) {
      setUrl(null)
      return
    }
    let alive = true
    let objectUrl = ''
    void api.fetchSocialAttachment(attachmentId).then((b) => {
      const next = URL.createObjectURL(b)
      if (!alive) {
        URL.revokeObjectURL(next)
        return
      }
      objectUrl = next
      setUrl(next)
    })
    return () => {
      alive = false
      if (objectUrl) URL.revokeObjectURL(objectUrl)
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
    <StoryOverlay onClose={onClose}>
      <article
        className="relative flex h-[min(40rem,88dvh)] w-full max-w-[22rem] flex-col overflow-hidden rounded-[22px] shadow-2xl"
        style={{
          background:
            'linear-gradient(165deg, var(--profile-accent, var(--primary)) 0%, color-mix(in oklch, var(--profile-accent, var(--primary)) 28%, black) 100%)',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex gap-1 px-3 pt-3">
          {author.items.map((s, i) => (
            <span
              key={s.id}
              className={cn('h-0.5 flex-1 rounded-full', i <= idx ? 'bg-white' : 'bg-white/25')}
            />
          ))}
        </div>
        <div className="flex items-center gap-2 px-3 py-2">
          <SocialAvatar name={author.username} src={author.avatar_url} className="size-8 text-xs" />
          <p className="min-w-0 flex-1 truncate font-display text-sm font-semibold text-white">
            @{author.username}
          </p>
          <button
            type="button"
            className="inline-flex size-8 items-center justify-center rounded-full bg-black/35 text-white/90"
            aria-label="Fechar status"
            onClick={onClose}
          >
            <X className="size-4" />
          </button>
        </div>
        <div className="relative flex min-h-0 flex-1 items-center justify-center px-6 py-8">
          <button
            type="button"
            className="absolute inset-y-0 left-0 z-10 w-1/3"
            aria-label="Anterior"
            onClick={() => idx > 0 && setIdx((i) => i - 1)}
          />
          <button
            type="button"
            className="absolute inset-y-0 right-0 z-10 w-2/3"
            aria-label="Próximo"
            onClick={() => void next()}
          />
          {item.kind === 'image' && url ? (
            <img src={url} alt="" className="pointer-events-none max-h-full max-w-full object-contain" />
          ) : (
            <p className="pointer-events-none text-center font-display text-2xl font-semibold leading-snug text-white">
              {item.body}
            </p>
          )}
        </div>
      </article>
    </StoryOverlay>
  )
}
