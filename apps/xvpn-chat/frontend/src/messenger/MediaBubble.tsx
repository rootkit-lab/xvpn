import { FileText } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useChat } from '@chat/messenger/ChatProvider'
import type { Message } from '@chat/chatapi/types'

const cache = new Map<number, string>()

export function MediaBubble({ message }: { message: Message }) {
  const { api } = useChat()
  const [url, setUrl] = useState<string | null>(() =>
    message.attachment_id ? (cache.get(message.attachment_id) ?? null) : null,
  )

  useEffect(() => {
    const id = message.attachment_id
    if (!id) return
    const hit = cache.get(id)
    if (hit) {
      setUrl(hit)
      return
    }
    let alive = true
    void api.fetchAttachment(id).then((blob) => {
      const next = URL.createObjectURL(blob)
      cache.set(id, next)
      if (alive) setUrl(next)
    })
    return () => {
      alive = false
    }
  }, [api, message.attachment_id])

  const kind = message.kind ?? 'text'
  if (kind === 'image') {
    return url ? (
      <img src={url} alt={message.filename || 'imagem'} className="max-h-56 max-w-full rounded-[14px] object-cover" />
    ) : (
      <p className="text-xs text-muted-foreground">Carregando imagem…</p>
    )
  }
  if (kind === 'audio') {
    return url ? (
      <audio controls src={url} className="h-9 w-52" />
    ) : (
      <p className="text-xs text-muted-foreground">Carregando áudio…</p>
    )
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
