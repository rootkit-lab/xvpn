import { useCallback, useMemo } from 'react'
import { Activity, MessageCircle, Shield } from 'lucide-react'
import { api } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { formatDuration } from '@/lib/format'
import { useAuth } from '@/lib/auth-context'
import { ROLE_LABELS } from '@/lib/roles'
import { cn } from '@/lib/utils'
import { useChatPanel } from '@/components/layout/use-chat-panel'

/** Barra de status fixa — saúde da API, chat e sessão. GET /api/status é público. */
export function PanelStatusBar({ variant }: { variant: 'user' | 'admin' }) {
  const { user } = useAuth()
  const fetchStatus = useCallback(() => api.status(), [])
  const { data: status, error } = usePollingData(fetchStatus, 10_000)
  const online = Boolean(status) && !error
  const { hidden, open, setDockOpen, unread, session } = useChatPanel()
  const totalUnread = useMemo(() => Object.values(unread).reduce((a, b) => a + b, 0), [unread])
  const showChat = Boolean(session?.loggedIn && !hidden)

  return (
    <footer
      data-variant={variant}
      className="chrome-bar flex h-8 shrink-0 items-center gap-3 border-t border-white/8 px-4 font-display text-[11px] text-muted-foreground"
    >
      <span className="flex items-center gap-1.5">
        <span
          className={cn(
            'size-1.5 rounded-full',
            online ? 'status-safe-dot' : 'bg-destructive',
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
      {showChat && (
        <>
          <span className="text-white/15">│</span>
          <button
            type="button"
            aria-pressed={open}
            aria-controls="xvpn-chat-sidebar"
            aria-label={open ? 'Fechar chat' : 'Abrir chat'}
            onClick={() => setDockOpen(!open)}
            className={cn(
              'relative flex items-center gap-1.5 rounded-[10px] px-2 py-0.5 font-display transition-colors',
              open ? 'bg-safe/15 text-safe' : 'hover:bg-white/8 hover:text-foreground',
            )}
          >
            <MessageCircle className="size-3.5" />
            Chat
            {totalUnread > 0 && (
              <span className="flex h-4 min-w-4 items-center justify-center rounded-full bg-safe px-1 text-[10px] font-bold text-safe-foreground">
                {totalUnread > 9 ? '9+' : totalUnread}
              </span>
            )}
          </button>
        </>
      )}
    </footer>
  )
}
