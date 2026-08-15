import { useCallback } from 'react'
import { Activity, Shield } from 'lucide-react'
import { api } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { formatDuration } from '@/lib/format'
import { useAuth } from '@/lib/auth-context'
import { ROLE_LABELS } from '@/lib/roles'
import { cn } from '@/lib/utils'

/** Barra de status fixa — saúde da API e sessão. GET /api/status é público. */
export function PanelStatusBar({ variant }: { variant: 'user' | 'admin' }) {
  const { user } = useAuth()
  const fetchStatus = useCallback(() => api.status(), [])
  const { data: status, error } = usePollingData(fetchStatus, 10_000)
  const online = Boolean(status) && !error

  return (
    <footer
      className={cn(
        'flex h-8 shrink-0 items-center gap-3 border-t px-4 text-[11px] text-muted-foreground',
        variant === 'admin'
          ? 'border-white/5 bg-card/80 font-mono backdrop-blur'
          : 'border-white/8 bg-card/70 backdrop-blur-xl',
      )}
    >
      <span className="flex items-center gap-1.5">
        <span
          className={cn(
            'size-1.5 rounded-full',
            online ? 'bg-primary shadow-[0_0_8px_var(--color-glow)]' : 'bg-destructive',
          )}
        />
        {online ? 'API' : 'API offline'}
      </span>
      {status && (
        <>
          <span className="text-white/15">│</span>
          <span className="flex items-center gap-1">
            <Shield className="size-3" />
            {status.connected_peers}/{status.total_peers} peers
          </span>
          <span className="text-white/15">│</span>
          <span className="flex items-center gap-1">
            <Activity className="size-3" />
            {formatDuration(status.uptime_seconds)}
          </span>
          <span className="hidden text-white/15 sm:inline">│</span>
          <span className="hidden sm:inline">api v{status.api_version}</span>
        </>
      )}
      <span className="ml-auto truncate">
        {user ? `${user.username} · ${ROLE_LABELS[user.role]}` : ''}
      </span>
    </footer>
  )
}
