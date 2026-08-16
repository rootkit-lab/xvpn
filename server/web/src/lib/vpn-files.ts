// Host dos shares de arquivo — invariante AGENTS.md #2: Samba e
// XDriver escutam só em wg0, nunca no IP público. Member não chama
// GET /api/config (viewerUp), então o endereço é constante, não lido da API.
export const VPN_FILE_HOST = '10.66.66.1'
export const FILEBROWSER_URL = 'https://xdriver.corp.ihuull.com'

export function sambaUnc(share: string): string {
  return `\\\\${VPN_FILE_HOST}\\${share}`
}

export function sambaUri(share: string): string {
  return `smb://${VPN_FILE_HOST}/${share}`
}

export function personalShareName(username: string): string {
  return `home-${username}`
}
