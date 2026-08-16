export type SoundKind = 'in' | 'out' | 'call'

function clampVolume(v: number): number {
  if (Number.isNaN(v)) return 0
  return Math.min(1, Math.max(0, v))
}

function playSweep(startHz: number, endHz: number, dur: number, volume: number): void {
  const gainLevel = clampVolume(volume) * 0.1
  if (gainLevel <= 0) return
  try {
    const ctx = new AudioContext()
    const now = ctx.currentTime
    const osc = ctx.createOscillator()
    const gain = ctx.createGain()
    osc.type = 'sine'
    osc.frequency.setValueAtTime(startHz, now)
    osc.frequency.setValueAtTime(endHz, now + dur * 0.35)
    gain.gain.setValueAtTime(0.0001, now)
    gain.gain.exponentialRampToValueAtTime(gainLevel, now + 0.02)
    gain.gain.exponentialRampToValueAtTime(0.0001, now + dur)
    osc.connect(gain)
    gain.connect(ctx.destination)
    osc.start(now)
    osc.stop(now + dur + 0.02)
    osc.onended = () => void ctx.close()
  } catch {
    // autoplay / AudioContext indisponível
  }
}

export function playTone(kind: SoundKind, volume: number): void {
  if (kind === 'in') playSweep(880, 1320, 0.22, volume)
  else if (kind === 'out') playSweep(660, 990, 0.14, volume)
  else playSweep(440, 660, 0.35, volume)
}

/** Som legado — entrada, volume médio. */
export function playMessageSound(): void {
  playTone('in', 0.8)
}

export function startRingtone(volume: number): () => void {
  playTone('call', volume)
  const id = window.setInterval(() => playTone('call', volume), 1800)
  return () => window.clearInterval(id)
}
