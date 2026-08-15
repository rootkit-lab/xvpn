import { useLocation } from 'react-router-dom'
import { useChat } from '@chat/messenger/ChatProvider'

/** Chat no chrome: escondido na landing, nos logins e na página cheia. */
export function isChatChromeHidden(pathname: string): boolean {
  return pathname === '/' || pathname === '/my/login' || pathname === '/admin/login' || pathname.startsWith('/social/messages')
}

export function useChatPanel() {
  const { pathname } = useLocation()
  const chat = useChat()
  const hidden = isChatChromeHidden(pathname)
  const open = Boolean(chat.session?.loggedIn && chat.dockOpen && !hidden)
  return { ...chat, hidden, open }
}
