import type { ReactNode } from 'react'
import { Outlet, useLocation } from 'react-router-dom'
import { AnimatePresence, motion } from 'framer-motion'
import { ChatPopouts } from '@chat/messenger/ChatPopouts'
import { ChatSidebar } from '@chat/messenger/ChatSidebar'
import { cn } from '@/lib/utils'
import { PanelHeader } from '@/components/layout/panel-header'
import { PanelStatusBar } from '@/components/layout/panel-status-bar'
import { useChatPanel } from '@/components/layout/use-chat-panel'

/** Chrome de sistema compartilhado — nav esquerda, chat no rail direito. */
export function SystemChrome({
  variant,
  subtitle,
  nav,
  className,
  asideClassName,
  mainClassName,
}: {
  variant: 'user' | 'admin' | 'social'
  subtitle: string
  nav: ReactNode
  className?: string
  asideClassName?: string
  mainClassName?: string
}) {
  const location = useLocation()
  const { open: chatOpen, hidden, session } = useChatPanel()
  const showPopouts = Boolean(session?.loggedIn && !hidden)

  return (
    <div className={cn('watch-face relative flex h-svh w-full overflow-hidden', className)}>
      <div className="watch-vignette pointer-events-none absolute inset-0" aria-hidden="true" />
      <aside
        className={cn(
          'relative z-20 flex h-full w-60 shrink-0 flex-col border-r border-white/8',
          variant === 'admin' ? 'w-64' : '',
          'watch-complication',
          asideClassName,
        )}
      >
        <div className={cn('flex items-center gap-2.5 px-5 py-4', variant === 'admin' && 'px-6')}>
          <img
            src="/logo-192.png"
            alt="XVPN"
            className={cn(
              'size-8',
              variant === 'admin' && 'drop-shadow-[0_0_12px_var(--color-glow)]',
              variant !== 'admin' && 'rounded-[10px]',
            )}
          />
          <div className="min-w-0">
            <span className={cn('font-display block font-semibold tracking-tight', variant === 'admin' ? 'text-lg' : 'text-base')}>
              XVPN
            </span>
            <span className="hud-label text-muted-foreground/70">{subtitle}</span>
          </div>
        </div>
        {variant === 'admin' && <div className="scanline mx-3" />}
        {nav}
      </aside>
      <div className="relative z-10 flex min-w-0 flex-1 flex-col">
        <PanelHeader variant={variant} />
        <div className="flex min-h-0 flex-1">
          <main className={cn('min-h-0 flex-1 overflow-y-auto', variant === 'admin' ? 'p-8' : 'p-6 md:p-8', mainClassName)}>
            <AnimatePresence mode="wait">
              <motion.div
                key={location.pathname}
                initial={{ opacity: 0, y: 8 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: -6 }}
                transition={{ duration: 0.22, ease: 'easeOut' }}
              >
                <Outlet />
              </motion.div>
            </AnimatePresence>
          </main>
          {chatOpen && (
            <aside
              id="xvpn-chat-sidebar"
              className="watch-complication relative z-20 flex h-full w-80 shrink-0 flex-col border-l border-white/8 max-md:absolute max-md:inset-y-0 max-md:right-0 max-md:z-30 max-md:w-[min(100%,20rem)] max-md:shadow-2xl"
            >
              <ChatSidebar />
            </aside>
          )}
        </div>
        <PanelStatusBar variant={variant === 'admin' ? 'admin' : 'user'} />
      </div>
      {showPopouts && <ChatPopouts railOpen={chatOpen} />}
    </div>
  )
}
