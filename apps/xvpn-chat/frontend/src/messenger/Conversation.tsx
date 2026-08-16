import { ArrowUp, Check, CheckCheck, Mic, Paperclip, Phone, Video, MessageCircle, Minus, X } from 'lucide-react'
import { useEffect, useRef, useState, type DragEvent, type FormEvent, type KeyboardEvent } from 'react'
import { cn } from '@chat/lib/utils'
import { useChat } from '@chat/messenger/ChatProvider'
import { MediaBubble } from '@chat/messenger/MediaBubble'
import { initials, StatusDot } from '@chat/messenger/StatusDot'
import { ChatButton, ChatInput } from '@chat/messenger/ui'
import { ChatIconButton } from '@chat/messenger/chrome'
import { audioConstraints, useChatSettings } from '@chat/messenger/ChatSettings'
import { audioFileFromChunks, clipboardLooksLikeImage, filesFromClipboard, pickRecorderMime } from '@chat/messenger/media'
import { hasRTCPeerConnection, openCallInBrowser } from '@chat/messenger/webrtc'

function ReceiptTicks({
  delivered,
  read,
  showRead,
}: {
  delivered: boolean
  read: boolean
  showRead: boolean
}) {
  const seen = showRead && read
  const Icon = delivered || seen ? CheckCheck : Check
  return <Icon className={cn('size-3.5', seen ? 'text-[var(--safe-foreground)]' : 'opacity-70')} strokeWidth={2.4} />
}

function dayLabel(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleDateString('pt-BR', { day: '2-digit', month: 'short' })
}

export function Conversation({
  threadKey,
  onClose,
  onMinimize,
  alignEnd,
  variant = 'page',
}: {
  threadKey: string
  onClose?: () => void
  onMinimize?: () => void
  alignEnd?: boolean
  variant?: 'page' | 'popout'
}) {
  const { messages, typing, send, sendFile, session, contactByKey, api, presence, startCall } = useChat()
  const { settings } = useChatSettings()
  const contact = contactByKey(threadKey)
  const list = messages[threadKey] ?? []
  const [body, setBody] = useState('')
  const [dragOver, setDragOver] = useState(false)
  const [recording, setRecording] = useState(false)
  const bottom = useRef<HTMLDivElement>(null)
  const rootRef = useRef<HTMLDivElement>(null)
  const fileRef = useRef<HTMLInputElement>(null)
  const recorder = useRef<MediaRecorder | null>(null)
  const chunks = useRef<Blob[]>([])
  const takeFilesRef = useRef<(files: FileList | File[] | null) => Promise<void>>(async () => {})

  useEffect(() => {
    bottom.current?.scrollIntoView({ block: 'end' })
  }, [list.length, threadKey])

  useEffect(() => {
    takeFilesRef.current = async (files) => {
      if (!files) return
      for (const f of Array.from(files)) {
        await sendFile(f, threadKey)
      }
    }
  }, [sendFile, threadKey])

  useEffect(() => {
    async function onPaste(e: ClipboardEvent) {
      const root = rootRef.current
      if (!root) return
      const target = e.target as Node | null
      const active = document.activeElement
      const inside = Boolean((target && root.contains(target)) || (active && root.contains(active)))
      if (!inside) return
      const text = e.clipboardData?.getData('text/plain')?.trim() ?? ''
      if (text && !clipboardLooksLikeImage(e)) return
      e.preventDefault()
      const files = await filesFromClipboard(e)
      if (files.length) await takeFilesRef.current(files)
    }
    document.addEventListener('paste', onPaste)
    return () => document.removeEventListener('paste', onPaste)
  }, [])

  if (!contact) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-2 px-4 text-muted-foreground">
        <MessageCircle className="size-10 opacity-35" strokeWidth={1.5} />
        <p className="font-display text-sm">Selecione um contato.</p>
      </div>
    )
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

  async function takeFiles(files: FileList | File[] | null) {
    await takeFilesRef.current(files)
  }

  function onDrop(e: DragEvent) {
    e.preventDefault()
    setDragOver(false)
    void takeFiles(e.dataTransfer.files)
  }

  async function toggleRec() {
    if (recording) {
      recorder.current?.stop()
      setRecording(false)
      return
    }
    const stream = await navigator.mediaDevices.getUserMedia({ audio: audioConstraints(settings.micId) })
    const mime = pickRecorderMime()
    const rec = mime ? new MediaRecorder(stream, { mimeType: mime }) : new MediaRecorder(stream)
    chunks.current = []
    rec.ondataavailable = (ev) => {
      if (ev.data.size) chunks.current.push(ev.data)
    }
    rec.onstop = () => {
      stream.getTracks().forEach((t) => t.stop())
      void sendFile(audioFileFromChunks(chunks.current, rec.mimeType), threadKey)
    }
    recorder.current = rec
    rec.start()
    setRecording(true)
  }

  let lastDay = ''
  const peerStatus =
    contact.kind === 'dm' && contact.peerUserId ? (presence[contact.peerUserId] ?? 'offline') : undefined
  const popout = variant === 'popout'

  return (
    <div
      ref={rootRef}
      className="relative flex h-full min-h-0 flex-col"
      onDragOver={(e) => {
        e.preventDefault()
        setDragOver(true)
      }}
      onDragLeave={() => setDragOver(false)}
      onDrop={onDrop}
    >
      {dragOver && (
        <div className="pointer-events-none absolute inset-0 z-20 flex items-center justify-center rounded-[18px] border-2 border-dashed border-[var(--safe)] bg-black/40 font-display text-sm text-[var(--safe)]">
          Solte para enviar
        </div>
      )}
      <header
        className={cn(
          'flex items-center gap-2',
          popout ? 'border-b border-white/8 bg-secondary/40 px-2 py-1.5' : 'px-3 py-2.5',
          !popout && (alignEnd ? 'flex-row-reverse' : 'justify-between'),
        )}
      >
        {popout && (
          <span className="relative shrink-0">
            <span className="flex size-8 items-center justify-center rounded-full bg-primary/20 text-[10px] font-semibold text-primary">
              {initials(contact.title)}
            </span>
            {peerStatus && <StatusDot status={peerStatus} className="absolute -bottom-0.5 -right-0.5 size-2 ring-1" />}
          </span>
        )}
        {popout ? (
          <button type="button" className="min-w-0 flex-1 cursor-pointer text-left" onClick={onMinimize}>
            <p className="truncate font-display text-sm font-semibold">{contact.title}</p>
            {typing[threadKey] ? (
              <p className="text-[11px] text-muted-foreground">digitando…</p>
            ) : (
              peerStatus && <p className="text-[11px] capitalize text-muted-foreground">{peerStatus}</p>
            )}
          </button>
        ) : (
          <div className={cn('min-w-0 flex-1', alignEnd && 'text-right')}>
            <p className="truncate font-display text-sm font-semibold">{contact.title}</p>
            {typing[threadKey] && <p className="text-[11px] text-muted-foreground">digitando…</p>}
          </div>
        )}
        <div className="flex shrink-0 items-center gap-1">
          {contact.kind === 'dm' && contact.peerUserId && (
            <>
              <ChatIconButton
                label={hasRTCPeerConnection() ? 'Chamada de voz' : 'Abrir chamada no navegador'}
                onClick={() => (hasRTCPeerConnection() ? startCall(contact.peerUserId!, false) : openCallInBrowser())}
              >
                <Phone className="h-4 w-4" />
              </ChatIconButton>
              <ChatIconButton
                label={hasRTCPeerConnection() ? 'Chamada de vídeo' : 'Abrir chamada no navegador'}
                onClick={() => (hasRTCPeerConnection() ? startCall(contact.peerUserId!, true) : openCallInBrowser())}
              >
                <Video className="h-4 w-4" />
              </ChatIconButton>
            </>
          )}
          {popout && onMinimize && (
            <button
              type="button"
              className="inline-flex size-7 items-center justify-center rounded-full text-muted-foreground hover:bg-white/10 hover:text-foreground"
              aria-label="Minimizar conversa"
              onClick={onMinimize}
            >
              <Minus className="size-4" />
            </button>
          )}
          {popout && onClose && (
            <button
              type="button"
              className="inline-flex size-7 items-center justify-center rounded-full text-muted-foreground hover:bg-white/10 hover:text-foreground"
              aria-label="Fechar conversa"
              onClick={onClose}
            >
              <X className="size-4" />
            </button>
          )}
          {!popout && onClose && (
            <ChatButton variant="ghost" className="size-8 shrink-0 px-0" aria-label="Fechar conversa" onClick={onClose}>
              ×
            </ChatButton>
          )}
        </div>
      </header>
      <div className="min-h-0 flex-1 overflow-y-auto px-3 py-2">
        {list.map((m) => {
          const day = dayLabel(m.created_at)
          const showDay = day !== lastDay
          lastDay = day
          const mine = Number(m.author_id) === Number(session?.userId)
          const kind = m.kind ?? 'text'
          return (
            <div key={m.id}>
              {showDay && (
                <p className="my-2 text-center font-display text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground/75">
                  {day}
                </p>
              )}
              <div className={cn('mb-1.5 flex', mine ? 'justify-end' : 'justify-start')}>
                <div
                  className={cn(
                    'max-w-[80%] break-words px-3 py-1.5 text-sm leading-snug',
                    mine
                      ? 'rounded-[18px] rounded-br-md bg-[var(--safe)] text-[var(--safe-foreground)] shadow-[0_0_18px_-6px_var(--glow-safe)]'
                      : 'watch-complication rounded-[18px] rounded-bl-md text-foreground',
                  )}
                >
                  {kind === 'text' ? m.body : <MediaBubble message={m} />}
                  {kind !== 'text' && m.body ? <p className="mt-1 text-xs opacity-80">{m.body}</p> : null}
                  <p
                    className={cn(
                      'mt-0.5 flex items-center justify-end gap-0.5 text-[10px] leading-none',
                      mine ? 'text-[var(--safe-foreground)]/80' : 'text-muted-foreground',
                    )}
                  >
                    {new Date(m.created_at).toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' })}
                    {mine && (
                      <ReceiptTicks delivered={Boolean(m.delivered)} read={Boolean(m.read)} showRead={settings.readReceipts} />
                    )}
                  </p>
                </div>
              </div>
            </div>
          )
        })}
        <div ref={bottom} />
      </div>
      <form className="flex items-center gap-1.5 p-2.5" onSubmit={onSubmit}>
        <input
          ref={fileRef}
          type="file"
          className="hidden"
          multiple
          onChange={(e) => {
            void takeFiles(e.target.files)
            e.target.value = ''
          }}
        />
        <ChatIconButton label="Anexar" filled onClick={() => fileRef.current?.click()}>
          <Paperclip className="h-4 w-4" strokeWidth={2} />
        </ChatIconButton>
        <ChatIconButton label={recording ? 'Parar áudio' : 'Gravar áudio'} filled onClick={() => void toggleRec()}>
          <Mic className={cn('h-4 w-4', recording && 'text-destructive')} strokeWidth={2} />
        </ChatIconButton>
        <ChatInput
          value={body}
          onChange={(e) => {
            setBody(e.target.value)
            if (contact && settings.sendTyping) api.sendTyping(contact.kind, contact.id)
          }}
          onKeyDown={onKey}
          placeholder={recording ? 'Gravando…' : 'Mensagem'}
          aria-label="Mensagem"
          autoComplete="off"
        />
        <button
          type="submit"
          aria-label="Enviar"
          className="flex size-10 shrink-0 items-center justify-center rounded-full bg-[var(--safe)] text-[var(--safe-foreground)] shadow-[0_0_18px_-4px_var(--glow-safe)] transition-transform hover:scale-105 active:scale-95 disabled:opacity-40"
        >
          <ArrowUp className="size-4" strokeWidth={2.25} />
        </button>
      </form>
    </div>
  )
}
