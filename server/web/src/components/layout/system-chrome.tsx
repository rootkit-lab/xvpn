import type { ReactNode } from 'react'
import { Outlet, useLocation } from 'react-router-dom'
import { AnimatePresence, motion } from 'framer-motion'
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
  const { open: chatOpen, activeKey } = useChatPanel()

  return (
    <div className={cn('relative flex h-svh w-full overflow-hidden bg-background', className)}>
      {variant === 'admin' ? (
        <div className="dot-grid pointer-events-none fixed inset-0 opacity-60" />
      ) : (
        <div
          className="pointer-events-none fixed inset-0 opacity-80"
          style={{
            background:
              'radial-gradient(90% 60% at 10% 0%, color-mix(in oklch, var(--primary) 18%, transparent), transparent 55%), radial-gradient(70% 50% at 90% 100%, color-mix(in oklch, var(--primary) 10%, transparent), transparent 50%)',
          }}
        />
      )}
      <aside
        className={cn(
          'relative z-20 flex h-full w-60 shrink-0 flex-col border-r backdrop-blur-xl',
          variant === 'admin' ? 'cyber-frame w-64 border-white/5 bg-card/70 backdrop-blur' : 'border-white/8 bg-card/50',
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
            <span className={cn('block font-semibold tracking-tight', variant === 'admin' ? 'text-lg' : 'text-base')}>
              XVPN
            </span>
            {variant === 'admin' ? (
              <span className="hud-label text-muted-foreground/70">{subtitle}</span>
            ) : (
              <span className="text-[11px] text-muted-foreground">{subtitle}</span>
            )}
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
              className={cn(
                'relative z-20 flex h-full shrink-0 flex-col border-l border-white/8 bg-card max-md:absolute max-md:inset-y-0 max-md:right-0 max-md:z-30 max-md:shadow-2xl',
                activeKey ? 'w-96 max-md:w-[min(100%,24rem)]' : 'w-80 max-md:w-[min(100%,20rem)]',
              )}
            >
              <ChatSidebar />
            </aside>
          )}
        </div>
        <PanelStatusBar variant={variant === 'admin' ? 'admin' : 'user'} />
      </div>
    </div>
  )
}
