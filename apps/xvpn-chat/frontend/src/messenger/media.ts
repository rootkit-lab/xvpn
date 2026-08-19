const AUDIO_MIMES = ['audio/ogg;codecs=opus', 'audio/ogg', 'audio/mp4', 'audio/webm;codecs=opus', 'audio/webm'] as const

const ALLOWED: Record<string, string> = {
  'audio/webm': 'audio.webm',
  'audio/ogg': 'audio.ogg',
  'audio/mp4': 'audio.m4a',
  'audio/mpeg': 'audio.mp3',
  'audio/wav': 'audio.wav',
}

export function pickRecorderMime(): string {
  if (typeof MediaRecorder === 'undefined') return ''
  for (const m of AUDIO_MIMES) {
    if (MediaRecorder.isTypeSupported(m)) return m
  }
  return ''
}

export function normalizeMediaMime(raw: string | undefined): string {
  const base = (raw ?? '').split(';')[0].trim().toLowerCase()
  if (base in ALLOWED) return base
  if (base.startsWith('audio/webm')) return 'audio/webm'
  if (base.startsWith('audio/ogg')) return 'audio/ogg'
  if (base.startsWith('audio/mp4') || base === 'audio/x-m4a') return 'audio/mp4'
  return base
}

export function audioFileFromChunks(chunks: Blob[], recorderMime: string): File {
  const mime = normalizeMediaMime(recorderMime) || 'audio/webm'
  const blob = new Blob(chunks, { type: mime })
  return new File([blob], ALLOWED[mime] ?? 'audio.webm', { type: mime })
}

export function typedBlob(blob: Blob, fallbackMime?: string): Blob {
  const want = normalizeMediaMime(blob.type) || normalizeMediaMime(fallbackMime)
  if (!want) return blob
  if (blob.type && normalizeMediaMime(blob.type) === want) return blob
  return new Blob([blob], { type: want })
}

export function coerceToArrayBuffer(raw: unknown): ArrayBuffer {
  if (typeof raw === 'string' && raw) {
    const bin = atob(raw)
    const out = new Uint8Array(bin.length)
    for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i)
    return out.buffer
  }
  if (raw instanceof ArrayBuffer) return raw
  if (Array.isArray(raw)) return Uint8Array.from(raw).buffer
  if (raw && typeof raw === 'object') {
    const rec = raw as { data?: unknown }
    if (typeof rec.data === 'string' || Array.isArray(rec.data)) return coerceToArrayBuffer(rec.data)
  }
  return new ArrayBuffer(0)
}

export async function filesFromClipboard(e: ClipboardEvent): Promise<File[]> {
  const out: File[] = []
  const seen = new Set<string>()
  const add = (f: File | null | undefined) => {
    if (!f || f.size === 0) return
    const key = `${f.name}:${f.size}:${f.type}`
    if (seen.has(key)) return
    seen.add(key)
    out.push(f)
  }
  const data = e.clipboardData
  if (data) {
    for (const f of Array.from(data.files ?? [])) add(f)
    for (const item of Array.from(data.items ?? [])) {
      if (item.kind === 'file' || item.type.startsWith('image/')) add(item.getAsFile())
    }
  }
  if (out.length) return out
  if (!navigator.clipboard?.read) return out
  try {
    const items = await navigator.clipboard.read()
    for (const item of items) {
      const type = item.types.find((t) => t.startsWith('image/'))
      if (!type) continue
      const blob = await item.getType(type)
      const ext = type.includes('jpeg') ? 'jpg' : type.split('/')[1] || 'png'
      add(new File([blob], `print.${ext}`, { type }))
    }
  } catch {
    // permissão / clipboard vazio
  }
  return out
}

export function clipboardLooksLikeImage(e: ClipboardEvent): boolean {
  const data = e.clipboardData
  if (!data) return false
  if (data.files && data.files.length > 0) return Array.from(data.files).some((f) => f.type.startsWith('image/') || f.size > 0)
  return Array.from(data.items ?? []).some((i) => i.kind === 'file' || i.type.startsWith('image/'))
}

export function videoConstraints(cameraId: string): MediaTrackConstraints {
  const base: MediaTrackConstraints = {
    width: { ideal: 1280 },
    height: { ideal: 720 },
    facingMode: 'user',
  }
  return cameraId ? { ...base, deviceId: { exact: cameraId } } : base
}
