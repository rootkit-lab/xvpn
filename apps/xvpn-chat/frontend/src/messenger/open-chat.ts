export const OPEN_CHAT_EVENT = 'xvpn-chat:open'

export type OpenChatDetail = {
  username?: string
  groupId?: number
  dmId?: number
  title?: string
}

export function openChat(detail: OpenChatDetail): void {
  window.dispatchEvent(new CustomEvent<OpenChatDetail>(OPEN_CHAT_EVENT, { detail }))
}
