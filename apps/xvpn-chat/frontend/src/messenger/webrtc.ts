/** WebKitGTK do Wails no Linux não expõe WebRTC. Windows/WebView2 em geral sim. */

export const CALL_BROWSER_URL = 'https://xchat.corp.ihuull.com/social/messages'

export function hasRTCPeerConnection(): boolean {
  return typeof globalThis.RTCPeerConnection === 'function'
}

export function openCallInBrowser(): void {
  const url = CALL_BROWSER_URL
  if (typeof window !== 'undefined') {
    window.open(url, '_blank', 'noopener,noreferrer')
  }
}
