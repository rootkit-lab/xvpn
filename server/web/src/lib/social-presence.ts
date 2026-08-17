import type { PresenceStatus } from '@/lib/api'

export function normalizePresence(raw?: string | null): PresenceStatus {
  if (raw === 'online' || raw === 'away' || raw === 'dnd') return raw
  return 'offline'
}

export function presenceLabel(status: PresenceStatus): string {
  if (status === 'online') return 'Online'
  if (status === 'away') return 'Ausente'
  if (status === 'dnd') return 'Ocupado'
  return 'Offline'
}

export function livePresence(
  userId: number,
  rest?: string | null,
  live?: Record<number, PresenceStatus> | null,
): PresenceStatus {
  const fromLive = live?.[userId]
  if (fromLive) return normalizePresence(fromLive)
  return normalizePresence(rest)
}
