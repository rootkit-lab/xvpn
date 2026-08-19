import { useLocation } from 'react-router-dom'
import { useChat } from '@chat/messenger/ChatProvider'
import { productKind } from '@/lib/product-host'

/** Chat no chrome: escondido na landing, nos logins e na página cheia. */
export function isChatChromeHidden(pathname: string, kind = productKind()): boolean {
  if (kind === 'xgit-corp') {
    return pathname === '/login'
  }
  return (
    pathname === '/' ||
    pathname === '/my/login' ||
    pathname === '/admin/login' ||
    pathname.startsWith('/social/messages') ||
    pathname.startsWith('/xgroup/messages')
  )
}

export function useChatPanel() {
  const { pathname } = useLocation()
  const chat = useChat()
  const hidden = isChatChromeHidden(pathname)
  const open = Boolean(chat.session?.loggedIn && chat.dockOpen && !hidden)
  return { ...chat, hidden, open }
}
