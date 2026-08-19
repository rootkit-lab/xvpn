import type { ReactNode } from 'react'
import { Outlet, useLocation } from 'react-router-dom'
import { AnimatePresence, motion } from 'framer-motion'
import { ChatPopouts } from '@chat/messenger/ChatPopouts'
import { ChatSidebar } from '@chat/messenger/ChatSidebar'
import { cn } from '@/lib/utils'
import { headerProduct } from '@/lib/product-host'
import { isSocialProfilePath } from '@/lib/social-profile'
import { PanelHeader } from '@/components/layout/panel-header'
import { PageHeading } from '@/components/layout/page-heading'
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
  const product = headerProduct()
  const fillMain = /\/(social|xgroup)\/messages/.test(location.pathname)
  const hideNav = variant === 'social' && isSocialProfilePath(location.pathname)

  return (
    <div
      data-product={product}
      className={cn('watch-face relative flex h-svh w-full flex-col overflow-hidden', className)}
    >
      <div className="watch-vignette pointer-events-none absolute inset-0" aria-hidden="true" />
      <PanelHeader variant={variant} />
      <div className="relative z-10 flex min-h-0 flex-1">
        {!hideNav && (
          <aside
            className={cn(
              'relative z-20 flex h-full w-60 shrink-0 flex-col border-r border-white/8',
              variant === 'admin' ? 'w-64' : '',
              'watch-complication',
              asideClassName,
            )}
          >
            <div className={cn('px-5 pt-4', variant === 'admin' && 'px-6')}>
              <span className="hud-label text-muted-foreground/70">{subtitle}</span>
            </div>
            {variant === 'admin' && <div className="scanline mx-3 mt-3" />}
            {nav}
          </aside>
        )}
        <div className="relative z-10 flex min-w-0 flex-1 flex-col">
          <div className="flex min-h-0 flex-1">
            <main
              className={cn(
                'min-h-0 min-w-0 flex-1',
                fillMain
                  ? 'flex flex-col overflow-hidden p-0'
                  : cn('overflow-y-auto', variant === 'admin' ? 'p-8' : 'p-6 md:p-8'),
                mainClassName,
              )}
            >
              {!fillMain && !hideNav && <PageHeading variant={variant} />}
              <AnimatePresence mode="wait">
                <motion.div
                  key={location.pathname}
                  className={cn(fillMain && 'flex h-full min-h-0 min-w-0 flex-1 flex-col overflow-hidden')}
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
      </div>
      {showPopouts && <ChatPopouts railOpen={chatOpen} />}
    </div>
  )
}
