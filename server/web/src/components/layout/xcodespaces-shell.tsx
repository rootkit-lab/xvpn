import { Outlet } from 'react-router-dom'
import { ChatPopouts } from '@chat/messenger/ChatPopouts'
import { ChatSidebar } from '@chat/messenger/ChatSidebar'
import { ProductHeader } from '@xvpn/ui/react/product-header'
import { useAuth } from '@/lib/auth-context'
import { AccountMenu } from '@/components/layout/account-menu'
import { AppLauncher } from '@/components/layout/app-launcher'
import { PanelStatusBar } from '@/components/layout/panel-status-bar'
import { useChatPanel } from '@/components/layout/use-chat-panel'

export function XcodespacesShell() {
  const { user } = useAuth()
  const { open: chatOpen, hidden, session } = useChatPanel()
  const showPopouts = Boolean(session?.loggedIn && !hidden)

  return (
    <div data-product="xcodespaces" className="watch-face relative flex h-svh flex-col overflow-hidden">
      <div className="watch-vignette pointer-events-none absolute inset-0" aria-hidden="true" />
      <ProductHeader
        product="xcodespaces"
        href="/"
        trailing={
          user ? (
            <>
              <AppLauncher variant="xgit" />
              <AccountMenu variant="user" />
            </>
          ) : null
        }
      />
      <div className="relative z-10 flex min-h-0 flex-1">
        <div className="flex min-w-0 flex-1 flex-col">
          <div className="flex min-h-0 flex-1">
            <main className="min-h-0 min-w-0 flex-1 overflow-hidden">
              <Outlet />
            </main>
            {chatOpen ? (
              <aside
                id="xvpn-chat-sidebar"
                className="watch-complication relative z-20 flex h-full w-80 shrink-0 flex-col border-l border-white/8 max-md:absolute max-md:inset-y-0 max-md:right-0 max-md:z-30 max-md:w-[min(100%,20rem)] max-md:shadow-2xl"
              >
                <ChatSidebar />
              </aside>
            ) : null}
          </div>
          <PanelStatusBar variant="user" />
        </div>
      </div>
      {showPopouts ? <ChatPopouts railOpen={chatOpen} /> : null}
    </div>
  )
}
