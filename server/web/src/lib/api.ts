// Cliente HTTP único para a API do control-plane XVPN — ver
// .cursor/rules/frontend-react.mdc (nunca `fetch` espalhado por
// componentes, sempre passar por aqui para tratamento de erro/auth
// consistente).
import type { Product, Role } from '@/lib/roles'
import { productKind, ssoLoginURL } from '@/lib/product-host'

const TOKEN_KEY = 'xvpn_token'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY)
}

export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

// parseErrorMessage extrai `{"error": "..."}` do corpo de uma resposta não-OK
// — compartilhado por request() e pelos dois helpers de marketplace abaixo
// (upload/download), que não podem passar por request() porque não usam
// downloadMarketplaceAsset baixa o blob autenticado (JWT) e dispara o
// save-as do browser — não passa por request() porque a resposta é
// binária, não JSON.
async function parseErrorMessage(res: Response): Promise<string> {
  let message = `Erro ${res.status}`
  try {
    const body = (await res.json()) as { error?: string }
    if (body?.error) message = body.error
  } catch {
    // corpo não é JSON (ex.: 502 do Nginx) — mantém mensagem genérica
  }
  return message
}

// handleUnauthorized centraliza o que request()/upload/download fazem
// quando o token expirou ou é inválido — mesmo efeito colateral (limpar
// token e mandar pro /login) nos três casos.
function handleUnauthorized(path: string) {
  clearToken()
  if (path.startsWith('/auth/login') || path === '/auth/me' || path === '/auth/logout') return
  if (productKind() === 'xauth') {
    if (window.location.pathname !== '/login' && window.location.pathname !== '/') {
      window.location.href = '/login'
    }
    return
  }
  window.location.href = ssoLoginURL()
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers)
  headers.set('Content-Type', 'application/json')
  const token = getToken()
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }

  const res = await fetch(`/api${path}`, { ...options, headers, credentials: 'include' })

  if (res.status === 401) {
    handleUnauthorized(path)
  }

  if (!res.ok) {
    throw new ApiError(res.status, await parseErrorMessage(res))
  }

  if (res.status === 204) {
    return undefined as T
  }
  return (await res.json()) as T
}

// downloadMarketplaceAsset também não passa por request(): a resposta é o
// binário do asset, não JSON, e um <a href> simples não anexaria o header
// Authorization (o token vive em localStorage, não em cookie) — o
// download autenticado (PLAN.md §6.8) precisa desse fetch manual. Simula o
// clique num link temporário apontando pro blob já baixado em memória.
async function downloadMarketplaceAsset(assetId: number, filename: string): Promise<void> {
  const headers = new Headers()
  const token = getToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)

  const path = `/marketplace/assets/${assetId}/download`
  const res = await fetch(`/api${path}`, { headers, credentials: 'include' })

  if (res.status === 401) {
    handleUnauthorized(path)
  }
  if (!res.ok) {
    throw new ApiError(res.status, await parseErrorMessage(res))
  }

  const blob = await res.blob()
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}

async function uploadDriverFile(root: DriverRoot, path: string, file: File): Promise<{ ok: boolean; name: string }> {
  const headers = new Headers()
  const token = getToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)
  const fd = new FormData()
  fd.append('root', root)
  fd.append('path', path)
  fd.append('file', file)
  const res = await fetch('/api/driver/upload', { method: 'POST', headers, body: fd, credentials: 'include' })
  if (res.status === 401) handleUnauthorized('/driver/upload')
  if (!res.ok) throw new ApiError(res.status, await parseErrorMessage(res))
  return (await res.json()) as { ok: boolean; name: string }
}

async function uploadSocialAttachment(file: File): Promise<SocialAttachment> {
  const headers = new Headers()
  const token = getToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)
  const fd = new FormData()
  fd.append('file', file)
  const res = await fetch('/api/social/attachments', { method: 'POST', headers, body: fd, credentials: 'include' })
  if (res.status === 401) handleUnauthorized('/social/attachments')
  if (!res.ok) throw new ApiError(res.status, await parseErrorMessage(res))
  return (await res.json()) as SocialAttachment
}

async function fetchSocialAttachment(id: number): Promise<Blob> {
  const headers = new Headers()
  const token = getToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)
  const path = `/social/attachments/${id}`
  const res = await fetch(`/api${path}`, { headers, credentials: 'include' })
  if (res.status === 401) handleUnauthorized(path)
  if (!res.ok) throw new ApiError(res.status, await parseErrorMessage(res))
  return res.blob()
}

async function downloadDriverFile(root: DriverRoot, path: string, filename: string): Promise<void> {
  const headers = new Headers()
  const token = getToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)
  const sp = new URLSearchParams({ root, path })
  const apiPath = `/driver/download?${sp}`
  const res = await fetch(`/api${apiPath}`, { headers, credentials: 'include' })
  if (res.status === 401) handleUnauthorized(apiPath)
  if (!res.ok) throw new ApiError(res.status, await parseErrorMessage(res))
  const blob = await res.blob()
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}

export interface User {
  id: number
  username: string
  role: Role
  created_at: string
  // Escopo de produto (Fase 33). Lista vazia = admin irrestrito.
  products?: Product[]
  // Acesso a arquivos (Fase 13, PLAN.md §6.9): toggles de SFTP/Samba
  // + chave pública SSH do usuário. Omitidos em respostas antigas
  // (back-end pré-Fase 13) — trate como false/"" se ausente.
  sftp_enabled?: boolean
  samba_enabled?: boolean
  ssh_public_key?: string
  disk_quota_mb?: number
}

export interface FileAccessResponse {
  sftp_enabled: boolean
  samba_enabled: boolean
  ssh_public_key: string
  disk_quota_mb: number
}

export interface DeviceSSHKey {
  device_id: number
  device_name: string
  fingerprint: string
  updated_at?: string
}

export interface Device {
  id: number
  user_id: number
  name: string
  public_key: string
  allowed_ip: string
  created_at: string
  last_handshake?: string
  receive_bytes: number
  transmit_bytes: number
  endpoint?: string
}

export interface InviteResponse {
  token: string
  expires_at: string
}

export interface ResetPasswordResponse {
  // Só preenchido quando nenhuma senha foi informada no pedido — o
  // servidor gera uma e devolve nesta única resposta (ver
  // PLAN.md/AGENTS.md: nunca recuperável depois).
  password?: string
}

export interface ProvisionWaitlistResponse {
  user: User
  password: string
  invite: InviteResponse
}

export interface StatusResponse {
  api_version: number
  uptime_seconds: number
  connected_peers: number
  total_peers: number
  receive_bytes_total: number
  transmit_bytes_total: number
}

export interface AuditLog {
  id: number
  actor: string
  action: string
  detail: string
  created_at: string
}

export interface WaitlistEntry {
  id: number
  name: string
  email: string
  message: string
  status: 'pending' | 'approved' | 'rejected'
  created_at: string
  reviewed_at?: string
}

export interface ConfigResponse {
  wireguard_interface: string
  wireguard_address: string
  wireguard_allowed_subnet: string
  wireguard_listen_port: number
  wireguard_endpoint: string
  server_public_key: string
  invite_token_ttl_minutes: number
  jwt_token_ttl_minutes: number
}

export interface DNSRecord {
  id: number
  hostname: string
  ipv4: string
  system: boolean
  enabled: boolean
  comment: string
}

export interface DNSResponse {
  listen: string
  listening: boolean
  query_ok: boolean
  query_detail?: string
  forwarders: string[]
  cache_size: number
  catch_all: boolean
  last_applied_at?: string
  last_apply_error?: string
  records: DNSRecord[]
}

// Espelha store.Platform/store.AppVisibility (server/internal/store/models.go)
// — Fase 11, ver PLAN.md §6.8.
export type MarketplacePlatform = 'linux' | 'windows' | 'android'
export type MarketplaceVisibility = 'global' | 'restricted'
export type MarketplaceNetwork = 'public' | 'vpn'
export type MarketplaceChannel = 'stable' | 'beta'

export interface MarketplaceAsset {
  id: number
  version_id: number
  platform: MarketplacePlatform
  arch: string
  filename: string
  sha256: string
  size_bytes: number
  download_count: number
  created_at: string
}

export interface MarketplaceVersion {
  id: number
  app_id: number
  version: string
  channel: string
  changelog: string
  created_at: string
  assets: MarketplaceAsset[]
}

export interface MarketplaceApp {
  id: number
  slug: string
  name: string
  description: string
  icon_url?: string
  visibility: MarketplaceVisibility
  network: MarketplaceNetwork
  source?: string
  source_path?: string
  created_at: string
  versions: MarketplaceVersion[]
  // access_user_ids só vem preenchido quando quem pediu administra o
  // marketplace (ver handleListMarketplaceApps no servidor) — nunca
  // presente na resposta pra member/viewer, mesmo em apps que eles
  // enxergam.
  access_user_ids?: number[]
}

// Espelha marketplaceAssetStat/marketplaceStatsResponse
// (server/internal/api/marketplace_handler.go) — Fase 12, estatísticas
// agregadas do catálogo pro dashboard admin (ver ROADMAP.md).
export interface MarketplaceAssetStat {
  asset_id: number
  app_id: number
  app_name: string
  version: string
  platform: MarketplacePlatform
  arch: string
  filename: string
  download_count: number
}

export interface MarketplaceStats {
  total_apps: number
  total_versions: number
  total_assets: number
  total_downloads: number
  total_storage_bytes: number
  top_assets: MarketplaceAssetStat[]
}

export interface PageParams {
  page?: number
  per_page?: number
  q?: string
  role?: string
  status?: string
  sftp?: string
  samba?: string
}

export type PresenceStatus = 'online' | 'away' | 'dnd' | 'invisible' | 'offline'

export interface SocialProfile {
  user_id: number
  username: string
  display_name: string
  bio: string
  avatar_url: string
  banner_url: string
  theme: string
  following: boolean
  followers: number
  following_count: number
  presence?: PresenceStatus
}

export interface SocialPostOriginal {
  id: number
  username: string
  display_name: string
  avatar_url: string
  body: string
  created_at: string
}

export interface SocialPost {
  id: number
  author_id: number
  username: string
  display_name: string
  avatar_url: string
  body: string
  kind: 'text' | 'repost' | string
  presence?: PresenceStatus
  starred: boolean
  stars: number
  comments: number
  reposts: number
  reposted: boolean
  original?: SocialPostOriginal
  created_at: string
}

export interface SocialPostComment {
  id: number
  post_id: number
  author_id: number
  username: string
  display_name: string
  avatar_url: string
  body: string
  created_at: string
}

export interface SocialStoryItem {
  id: number
  author_id: number
  username: string
  kind: 'text' | 'image' | string
  body: string
  attachment_id?: number
  mime?: string
  viewed: boolean
  expires_at: string
  created_at: string
}

export interface SocialStoryAuthor {
  author_id: number
  username: string
  avatar_url?: string
  unseen: boolean
  items: SocialStoryItem[]
}

export interface SocialAttachment {
  id: number
  filename: string
  mime: string
  size_bytes: number
  kind: string
}

export type DriverRoot = 'home' | 'shared'

export interface DriverEntry {
  name: string
  path: string
  is_dir: boolean
  size: number
  mod_time: number
}

export interface DriverList {
  root: DriverRoot
  path: string
  items: DriverEntry[]
}

export interface SocialGroup {
  id: number
  name: string
  description: string
  owner_user_id: number
  member_count: number
}

export interface SocialThread {
  id: number
  kind: 'dm' | 'group'
  title: string
  peer_user_id?: number
  last_body?: string
  last_at?: string
}

export interface SocialMessage {
  id: number
  thread_kind: string
  thread_id: number
  author_id: number
  body: string
  created_at: string
}

export interface PageEnvelope<T> {
  items: T[]
  total: number
  page: number
  per_page: number
}

function withQuery(path: string, params?: PageParams): string {
  if (!params) return path
  const sp = new URLSearchParams()
  if (params.page != null) sp.set('page', String(params.page))
  if (params.per_page != null) sp.set('per_page', String(params.per_page))
  if (params.q) sp.set('q', params.q)
  if (params.role) sp.set('role', params.role)
  if (params.status) sp.set('status', params.status)
  if (params.sftp) sp.set('sftp', params.sftp)
  if (params.samba) sp.set('samba', params.samba)
  const qs = sp.toString()
  return qs ? `${path}?${qs}` : path
}

export const api = {
  login: (username: string, password: string, aud?: string) =>
    request<{ token: string; user: User }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password, ...(aud ? { aud } : {}) }),
    }),
  logout: () => request<void>('/auth/logout', { method: 'POST' }),
  // me restaura {id, username, role} depois de um refresh — cookie SSO
  // ou Bearer. 401 aqui não redireciona (sonda de sessão).
  me: () => request<User>('/auth/me'),

  status: () => request<StatusResponse>('/status'),

  listUsers: (params?: PageParams) => request<PageEnvelope<User>>(withQuery('/users', params)),
  getUser: (id: number) => request<User>(`/users/${id}`),
  createUser: (username: string, password: string, role: Role, products?: Product[]) =>
    request<User>('/users', {
      method: 'POST',
      body: JSON.stringify({ username, password, role, products }),
    }),
  updateUser: (id: number, changes: { username?: string; role?: Role; products?: Product[] }) =>
    request<User>(`/users/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(changes),
    }),
  resetPassword: (id: number, password?: string) =>
    request<ResetPasswordResponse>(`/users/${id}/reset-password`, {
      method: 'POST',
      body: JSON.stringify({ password: password || undefined }),
    }),
  deleteUser: (id: number) => request<void>(`/users/${id}`, { method: 'DELETE' }),
  createInvite: (userId: number) => request<InviteResponse>(`/users/${userId}/invite`, { method: 'POST' }),
  // Acesso a arquivos (Fase 13, PLAN.md §6.9): aplica o estado desejado
  // de SFTP/Samba + chave pública SSH. O back-end calcula o diff e só
  // chama o provisionador pro que mudou (idempotente).
  setFileAccess: (id: number, body: {
    sftp_enabled: boolean
    samba_enabled: boolean
    ssh_public_key: string
    disk_quota_mb: number
  }) =>
    request<FileAccessResponse>(`/users/${id}/file-access`, {
      method: 'PUT',
      body: JSON.stringify(body),
    }),
  // Chaves SSH auto-registradas pelos dispositivos do usuário (Fase 14).
  // Só leitura — para revogar, revoga-se o dispositivo.
  listUserSSHKeys: (id: number) =>
    request<{ device_keys: DeviceSSHKey[] }>(`/users/${id}/ssh-keys`),

  listDevices: (params?: PageParams) => request<PageEnvelope<Device>>(withQuery('/devices', params)),
  deleteDevice: (id: number) => request<void>(`/devices/${id}`, { method: 'DELETE' }),

  // listMyDevices/deleteMyDevice são o autosserviço da Fase 10 (ver
  // PLAN.md §6.7): qualquer papel autenticado gerencia os próprios
  // dispositivos, sem precisar das telas administrativas.
  listMyDevices: () => request<Device[]>('/me/devices'),
  deleteMyDevice: (id: number) => request<void>(`/me/devices/${id}`, { method: 'DELETE' }),
  // Chave SSH manual no portal (Fase 15) — distinta das chaves automáticas
  // dos dispositivos (POST /me/ssh-key via túnel).
  updateMySSHPublicKey: (sshPublicKey: string) =>
    request<User>('/me/ssh-public-key', {
      method: 'PUT',
      body: JSON.stringify({ ssh_public_key: sshPublicKey }),
    }),
  // Autosserviço de senha (Fase 18). 204 sem corpo — a senha nova nunca
  // volta na resposta (diferente do reset administrativo, que devolve a
  // gerada uma única vez).
  changeMyPassword: (currentPassword: string, newPassword: string) =>
    request<void>('/me/password', {
      method: 'PATCH',
      body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
    }),

  listAudit: (params?: PageParams) => request<PageEnvelope<AuditLog>>(withQuery('/audit', params)),

  getConfig: () => request<ConfigResponse>('/config'),
  updateConfig: (body: { invite_token_ttl_minutes?: number; jwt_token_ttl_minutes?: number }) =>
    request<ConfigResponse>('/config', { method: 'PATCH', body: JSON.stringify(body) }),

  getDNS: () => request<DNSResponse>('/dns'),
  updateDNS: (body: { forwarders?: string; cache_size?: number; catch_all?: boolean }) =>
    request<DNSResponse>('/dns', { method: 'PATCH', body: JSON.stringify(body) }),
  createDNSRecord: (body: { hostname: string; ipv4: string; comment?: string; enabled?: boolean }) =>
    request<DNSResponse>('/dns/records', { method: 'POST', body: JSON.stringify(body) }),
  updateDNSRecord: (
    id: number,
    body: { hostname: string; ipv4: string; comment?: string; enabled?: boolean },
  ) => request<DNSResponse>(`/dns/records/${id}`, { method: 'PATCH', body: JSON.stringify(body) }),
  deleteDNSRecord: (id: number) => request<DNSResponse>(`/dns/records/${id}`, { method: 'DELETE' }),
  applyDNS: () => request<DNSResponse>('/dns/apply', { method: 'POST' }),

  // joinWaitlist é o único endpoint de escrita público (sem
  // autenticação) de toda a API — chamado da landing page em "/". Ver
  // PLAN.md pela decisão de design e o rate limit aplicado no backend.
  joinWaitlist: (name: string, email: string, message: string) =>
    request<WaitlistEntry>('/waitlist', {
      method: 'POST',
      body: JSON.stringify({ name, email, message }),
    }),
  listWaitlist: (params?: PageParams) => request<PageEnvelope<WaitlistEntry>>(withQuery('/waitlist', params)),
  approveWaitlist: (id: number) => request<WaitlistEntry>(`/waitlist/${id}/approve`, { method: 'POST' }),
  rejectWaitlist: (id: number) => request<WaitlistEntry>(`/waitlist/${id}/reject`, { method: 'POST' }),
  // provisionWaitlist orquestra "aprovar e provisionar": cria o User +
  // InviteToken num só passo e marca o cadastro como aprovado (ver
  // handleProvisionWaitlist no servidor).
  provisionWaitlist: (id: number, username: string, role: Role) =>
    request<ProvisionWaitlistResponse>(`/waitlist/${id}/provision`, {
      method: 'POST',
      body: JSON.stringify({ username, role }),
    }),

  // Catálogo do marketplace (Fases 11/16): listagem já filtrada por ACL;
  // publicação só via POST /marketplace/sync (CI). No painel resta ACL + download.
  listMarketplaceApps: () => request<MarketplaceApp[]>('/marketplace/apps'),
  setMarketplaceAppAccess: (id: number, userIds: number[]) =>
    request<{ user_ids: number[] }>(`/marketplace/apps/${id}/access`, {
      method: 'PUT',
      body: JSON.stringify({ user_ids: userIds }),
    }),
  downloadMarketplaceAsset,
  marketplaceStats: () => request<MarketplaceStats>('/marketplace/stats'),

  listSocialPeople: (params?: PageParams) =>
    request<PageEnvelope<SocialProfile>>(withQuery('/social/people', params)),
  getSocialMe: () => request<SocialProfile>('/social/profile'),
  patchSocialMe: (body: {
    display_name?: string
    bio?: string
    avatar_url?: string
    banner_url?: string
    theme?: string
  }) => request<SocialProfile>('/social/profile', { method: 'PATCH', body: JSON.stringify(body) }),
  getSocialProfile: (username: string) => request<SocialProfile>(`/social/u/${encodeURIComponent(username)}`),
  followUser: (username: string) =>
    request<{ ok: boolean }>(`/social/follow/${encodeURIComponent(username)}`, { method: 'POST' }),
  unfollowUser: (username: string) =>
    request<{ ok: boolean }>(`/social/follow/${encodeURIComponent(username)}`, { method: 'DELETE' }),
  listSocialGroups: (params?: PageParams) =>
    request<PageEnvelope<SocialGroup>>(withQuery('/social/groups', params)),
  createSocialGroup: (name: string, description: string) =>
    request<SocialGroup>('/social/groups', {
      method: 'POST',
      body: JSON.stringify({ name, description }),
    }),
  inviteToSocialGroup: (id: number, username: string) =>
    request<{ ok: boolean }>(`/social/groups/${id}/invite`, {
      method: 'POST',
      body: JSON.stringify({ username }),
    }),
  listSocialThreads: (params?: PageParams) =>
    request<PageEnvelope<SocialThread>>(withQuery('/social/threads', params)),
  openSocialThread: (username: string) =>
    request<SocialThread>('/social/threads', {
      method: 'POST',
      body: JSON.stringify({ username }),
    }),
  listSocialMessages: (kind: string, id: number, params?: PageParams) =>
    request<PageEnvelope<SocialMessage>>(withQuery(`/social/threads/${kind}/${id}/messages`, params)),
  postSocialMessage: (kind: string, id: number, body: string) =>
    request<SocialMessage>(`/social/threads/${kind}/${id}/messages`, {
      method: 'POST',
      body: JSON.stringify({ body }),
    }),
  listSocialFeed: (params?: PageParams) => request<PageEnvelope<SocialPost>>(withQuery('/social/feed', params)),
  listSocialUserPosts: (username: string, params?: PageParams) =>
    request<PageEnvelope<SocialPost>>(withQuery(`/social/u/${encodeURIComponent(username)}/posts`, params)),
  createSocialPost: (body: string) =>
    request<SocialPost>('/social/posts', { method: 'POST', body: JSON.stringify({ body }) }),
  deleteSocialPost: (id: number) => request<void>(`/social/posts/${id}`, { method: 'DELETE' }),
  starSocialPost: (id: number) =>
    request<{ starred: boolean; stars: number; post_id: number }>(`/social/posts/${id}/star`, {
      method: 'POST',
      body: JSON.stringify({}),
    }),
  listSocialComments: (id: number, params?: PageParams) =>
    request<PageEnvelope<SocialPostComment>>(withQuery(`/social/posts/${id}/comments`, params)),
  createSocialComment: (id: number, body: string) =>
    request<SocialPostComment>(`/social/posts/${id}/comments`, {
      method: 'POST',
      body: JSON.stringify({ body }),
    }),
  repostSocialPost: (id: number) =>
    request<{ reposted: boolean; reposts: number; post_id: number }>(`/social/posts/${id}/repost`, {
      method: 'POST',
      body: JSON.stringify({}),
    }),
  listSocialStories: () => request<{ items: SocialStoryAuthor[] }>('/social/stories'),
  createSocialStory: (body: string, extra?: { kind?: string; attachment_id?: number }) =>
    request<SocialStoryItem>('/social/stories', {
      method: 'POST',
      body: JSON.stringify({ body, kind: extra?.kind ?? 'text', attachment_id: extra?.attachment_id }),
    }),
  viewSocialStory: (id: number) =>
    request<{ ok: boolean }>(`/social/stories/${id}/view`, { method: 'POST', body: JSON.stringify({}) }),
  uploadSocialAttachment: (file: File) => uploadSocialAttachment(file),
  fetchSocialAttachment: (id: number) => fetchSocialAttachment(id),

  listDriver: (root: DriverRoot, path = '') => {
    const sp = new URLSearchParams({ root })
    if (path) sp.set('path', path)
    return request<DriverList>(`/driver/ls?${sp}`)
  },
  mkdirDriver: (root: DriverRoot, path: string, name: string) =>
    request<{ ok: boolean }>('/driver/mkdir', {
      method: 'POST',
      body: JSON.stringify({ root, path, name }),
    }),
  uploadDriver: (root: DriverRoot, path: string, file: File) => uploadDriverFile(root, path, file),
  downloadDriver: (root: DriverRoot, path: string, filename: string) => downloadDriverFile(root, path, filename),
  rmDriver: (root: DriverRoot, path: string) =>
    request<void>('/driver/rm', { method: 'DELETE', body: JSON.stringify({ root, path }) }),
}
