import { FileText, Pause, Play } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useChat } from '@chat/messenger/ChatProvider'
import { typedBlob } from '@chat/messenger/media'
import type { Message } from '@chat/chatapi/types'

const cache = new Map<number, string>()

export function MediaBubble({ message }: { message: Message }) {
  const { api } = useChat()
  const [url, setUrl] = useState<string | null>(() =>
    message.attachment_id ? (cache.get(message.attachment_id) ?? null) : null,
  )
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    const id = message.attachment_id
    if (!id) return
    const hit = cache.get(id)
    if (hit) {
      setUrl(hit)
      return
    }
    let alive = true
    setFailed(false)
    void api
      .fetchAttachment(id)
      .then((blob) => {
        const typed = typedBlob(blob, message.mime)
        if (typed.size === 0) throw new Error('vazio')
        const next = URL.createObjectURL(typed)
        cache.set(id, next)
        if (alive) setUrl(next)
      })
      .catch(() => {
        if (alive) setFailed(true)
      })
    return () => {
      alive = false
    }
  }, [api, message.attachment_id, message.mime])

  const kind = message.kind ?? 'text'
  if (kind === 'image') {
    if (failed) return <p className="text-xs opacity-80">Não foi possível carregar a imagem.</p>
    return url ? (
      <img src={url} alt={message.filename || 'imagem'} className="max-h-56 max-w-full rounded-[14px] object-cover" />
    ) : (
      <p className="text-xs opacity-80">Carregando imagem…</p>
    )
  }
  if (kind === 'audio') {
    if (failed) return <p className="text-xs opacity-80">Não foi possível carregar o áudio.</p>
    if (!url) return <p className="text-xs opacity-80">Carregando áudio…</p>
    return <AudioPlayer src={url} />
  }
  if (kind === 'file') {
    return (
      <a
        href={url ?? '#'}
        download={message.filename}
        className="flex items-center gap-2 text-sm underline-offset-2 hover:underline"
      >
        <FileText className="size-4 shrink-0" />
        <span className="truncate">{message.filename || 'arquivo'}</span>
      </a>
    )
  }
  return <span>{message.body}</span>
}

function AudioPlayer({ src }: { src: string }) {
  const ref = useRef<HTMLAudioElement>(null)
  const [playing, setPlaying] = useState(false)
  const [err, setErr] = useState(false)
  const [progress, setProgress] = useState(0)

  useEffect(() => {
    const el = ref.current
    if (!el) return
    const onTime = () => {
      if (el.duration) setProgress(el.currentTime / el.duration)
    }
    const onEnd = () => {
      setPlaying(false)
      setProgress(0)
    }
    el.addEventListener('timeupdate', onTime)
    el.addEventListener('ended', onEnd)
    return () => {
      el.removeEventListener('timeupdate', onTime)
      el.removeEventListener('ended', onEnd)
    }
  }, [src])

  async function toggle() {
    const el = ref.current
    if (!el) return
    if (playing) {
      el.pause()
      setPlaying(false)
      return
    }
    try {
      await el.play()
      setPlaying(true)
      setErr(false)
    } catch {
      setErr(true)
    }
  }

  return (
    <div className="flex w-52 items-center gap-2">
      <audio ref={ref} src={src} preload="metadata" onError={() => setErr(true)} />
      <button
        type="button"
        className="flex size-8 shrink-0 items-center justify-center rounded-full bg-black/20"
        aria-label={playing ? 'Pausar' : 'Reproduzir'}
        onClick={() => void toggle()}
      >
        {playing ? <Pause className="size-3.5" /> : <Play className="size-3.5" />}
      </button>
      <div className="min-w-0 flex-1">
        <div className="h-1 overflow-hidden rounded-full bg-black/20">
          <div className="h-full bg-current" style={{ width: `${Math.round(progress * 100)}%` }} />
        </div>
        {err && <p className="mt-1 text-[10px] opacity-80">Não foi possível reproduzir</p>}
      </div>
    </div>
  )
}
