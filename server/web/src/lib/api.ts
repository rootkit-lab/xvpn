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

async function requestText(path: string): Promise<string> {
  const headers = new Headers()
  const token = getToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)
  const res = await fetch(`/api${path}`, { headers, credentials: 'include' })
  if (res.status === 401) handleUnauthorized(path)
  if (!res.ok) throw new ApiError(res.status, await parseErrorMessage(res))
  return res.text()
}

async function downloadBinary(path: string, filename: string): Promise<void> {
  const headers = new Headers()
  const token = getToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)
  const res = await fetch(`/api${path}`, { headers, credentials: 'include' })
  if (res.status === 401) handleUnauthorized(path)
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
  xgit_enabled?: boolean
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

export interface CloudflareAccount {
  id: number
  name: string
  email: string
  token_hint: string
}

export interface PublicZone {
  id: number
  account_id: number
  name: string
  status: string
  name_servers: string[]
  intranet: boolean
}

export interface PublicRecord {
  id: number
  type: string
  name: string
  content: string
  ttl: number
  proxied: boolean
  intranet_ipv4?: string
  comment?: string
}

// Espelha store.Platform/store.AppVisibility (server/internal/store/models.go)
// — Fase 11, ver PLAN.md §6.8.
export type MarketplacePlatform = 'linux' | 'windows' | 'android'
export type MarketplaceVisibility = 'global' | 'restricted'
export type MarketplaceNetwork = 'public' | 'vpn'
export type MarketplaceKind = 'desktop' | 'web' | 'service' | 'library' | 'infra' | 'docs' | 'container'
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
  kind: MarketplaceKind
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
  project_slug?: string
  project_name?: string
  social_group_id?: number
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

export type DriverRoot = 'home' | 'shared' | `project:${string}`

export type ProjectRole = 'guest' | 'reporter' | 'developer' | 'maintainer' | 'owner'

export interface ProjectMember {
  user_id: number
  username: string
  role: ProjectRole
}

export type MeshServerRole = 'control' | 'mesh' | 'runner' | 'external'

export interface MeshServer {
  id: number
  bitlaunch_id: string
  name: string
  hostname: string
  role: MeshServerRole | string
  ipv4: string
  wg_ip: string
  region: string
  size: string
  status: string
  labels: string[]
  group_id?: number
  device_id?: number
  access_user_ids?: number[]
  account_id?: number
  notes?: string
  protected?: boolean
  has_runner_token?: boolean
  has_agent_token?: boolean
  created_at: string
  enroll_token?: string
}

export type ServiceKind = 'mongo' | 'redis' | 'rabbitmq' | 'lb'
export type ServiceBind = 'wg0' | 'loopback'
export type ServiceHost = 'local' | 'mesh'
export type ServiceStatus = 'pending' | 'ready' | 'error' | 'stopped'

export interface ManagedService {
  id: number
  slug: string
  kind: ServiceKind
  project_slug?: string
  host: ServiceHost
  mesh_server_id?: number
  mesh_hostname?: string
  bind: ServiceBind
  listen: string
  port: number
  hostname?: string
  endpoint: string
  status: ServiceStatus
  error?: string
  created_at: string
  password?: string
}

export type CiJobStatus =
  | 'awaiting_approval'
  | 'pending'
  | 'running'
  | 'success'
  | 'failed'
  | 'canceled'

export interface CiJobStep {
  name: string
  status: CiJobStatus
}

export interface CiWorkflow {
  name: string
  path: string
}

export interface CiJob {
  number: number
  workflow: string
  title: string
  event: string
  trigger: string
  ref: string
  branch: string
  sha: string
  actor?: string
  merge_request_number?: number
  status: CiJobStatus
  runner?: string
  has_log: boolean
  has_artifact: boolean
  error?: string
  jobs: CiJobStep[]
  duration_ms?: number
  can_approve?: boolean
  can_rerun?: boolean
  can_cancel?: boolean
  started_at?: string
  finished_at?: string
  created_at: string
}

export interface CiRunner {
  hostname: string
  name: string
  status: string
  labels?: string[]
  wg_ip?: string
}

export interface BitLaunchAccount {
  id: number
  name: string
  email: string
  token_hint: string
  balance_usd?: number
  used?: number
  limit?: number
  cost_per_hr?: number
  billing_alert_days?: number
}

export interface BitLaunchTopUp {
  id: string
  address: string
  crypto_symbol: string
  amount_usd: number
  amount_crypto: string
  status: string
  status_url: string
}

export interface ServerGroup {
  id: number
  name: string
  description: string
  created_at: string
}

export interface Project {
  slug: string
  name: string
  description: string
  app_id?: number
  social_group_id: number
  files_enabled: boolean
  visibility: MarketplaceVisibility
  network: MarketplaceNetwork
  runners: string[]
  member_count: number
  members?: ProjectMember[]
  created_at: string
  updated_at: string
  language?: string
  last_commit_at?: string
  starred?: boolean
  star_count?: number
  spark?: number[]
}

export interface XgitOverview {
  profile: SocialProfile
  repo_count: number
  star_count: number
  popular: Project[]
  contributions: { total: number; days: { date: string; count: number }[] }
  activity: XgitActivityItem[]
}

export interface XgitActivityItem {
  kind: 'commits' | 'repos_created' | 'repo_created' | 'merge_request' | string
  month?: string
  count?: number
  repo_count?: number
  repos?: string[]
  slug?: string
  number?: number
  title?: string
  description?: string
  comments?: number
  thread_id?: number
  language?: string
  created_at: string
}

export interface ProtectedBranch {
  pattern: string
  min_push_role: ProjectRole
}

export interface ProjectGit {
  clone_url: string
  exists: boolean
  protected_branches: ProtectedBranch[]
}

export interface GitLangStat {
  name: string
  bytes: number
  pct: number
}

export interface GitTreeEntry {
  name: string
  path: string
  type: 'blob' | 'tree' | string
  mode: string
  size: number
  sha: string
  last_commit?: GitCommit
}

export interface GitCommit {
  sha: string
  subject: string
  author: string
  date: string
}

export type MergeRequestStatus = 'open' | 'merged' | 'closed'

export interface MergeRequest {
  number: number
  title: string
  description: string
  source_branch: string
  target_branch: string
  author_id: number
  author: string
  status: MergeRequestStatus
  thread_id: number
  social_post_id?: number
  merged_at?: string
  merged_by?: string
  created_at: string
  updated_at: string
}

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
  getPublicDNSSettings: () =>
    request<{ accounts: CloudflareAccount[]; cloudflare: boolean }>('/dns/public/settings'),
  createCloudflareAccount: (body: { name: string; email: string; token: string }) =>
    request<CloudflareAccount>('/dns/public/settings/accounts', { method: 'POST', body: JSON.stringify(body) }),
  deleteCloudflareAccount: (id: number) =>
    request<void>(`/dns/public/settings/accounts/${id}`, { method: 'DELETE' }),
  listPublicZones: () => request<{ items: PublicZone[]; cloudflare: boolean }>('/dns/public/zones'),
  importPublicZones: () => request<{ items: PublicZone[]; cloudflare: boolean }>('/dns/public/zones/import', { method: 'POST' }),
  createPublicZone: (body: { name: string; account_id?: number; intranet?: boolean }) =>
    request<PublicZone>('/dns/public/zones', { method: 'POST', body: JSON.stringify(body) }),
  getPublicZone: (id: number) => request<PublicZone>(`/dns/public/zones/${id}`),
  listPublicRecords: (zoneId: number) =>
    request<{ items: PublicRecord[]; zone: PublicZone }>(`/dns/public/zones/${zoneId}/records`),
  createPublicRecord: (
    zoneId: number,
    body: {
      type: string
      name: string
      content: string
      ttl?: number
      proxied?: boolean
      intranet_ipv4?: string
      comment?: string
    },
  ) => request<PublicRecord>(`/dns/public/zones/${zoneId}/records`, { method: 'POST', body: JSON.stringify(body) }),
  deletePublicRecord: (zoneId: number, id: number) =>
    request<void>(`/dns/public/zones/${zoneId}/records/${id}`, { method: 'DELETE' }),

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

  listProjects: (scope?: 'all' | 'mine', cards?: boolean) => {
    const q = new URLSearchParams()
    if (scope) q.set('scope', scope)
    if (cards) q.set('cards', '1')
    const qs = q.toString()
    return request<{ items: Project[] }>(qs ? `/projects?${qs}` : '/projects')
  },
  getXgitOverview: () => request<XgitOverview>('/xgit/overview'),
  listXgitStars: () => request<{ items: Project[] }>('/xgit/stars'),
  toggleProjectStar: (slug: string) => request<Project>(`/projects/${encodeURIComponent(slug)}/star`, { method: 'POST' }),
  createXgitRepo: (body: { slug: string; name: string; description?: string; network?: MarketplaceNetwork }) =>
    request<Project>('/xgit/repos', { method: 'POST', body: JSON.stringify(body) }),
  getXgitSettings: () =>
    request<{
      default_visibility: MarketplaceVisibility
      default_network: MarketplaceNetwork
      allow_member_create: boolean
      clone_host: string
    }>('/xgit/settings'),
  updateXgitSettings: (body: {
    default_visibility?: MarketplaceVisibility
    default_network?: MarketplaceNetwork
    allow_member_create?: boolean
  }) =>
    request<{
      default_visibility: MarketplaceVisibility
      default_network: MarketplaceNetwork
      allow_member_create: boolean
      clone_host: string
    }>('/xgit/settings', { method: 'PATCH', body: JSON.stringify(body) }),
  listProjectTree: (slug: string, ref?: string, path?: string) => {
    const q = new URLSearchParams()
    if (ref) q.set('ref', ref)
    if (path) q.set('path', path)
    const qs = q.toString()
    return request<{
      items: GitTreeEntry[]
      ref: string
      path: string
      commit_count?: number
      tags?: string[]
      languages?: GitLangStat[]
    }>(`/projects/${encodeURIComponent(slug)}/tree${qs ? `?${qs}` : ''}`)
  },
  getProjectBlob: (slug: string, path: string, ref?: string) => {
    const q = new URLSearchParams({ path })
    if (ref) q.set('ref', ref)
    return request<{ path: string; ref: string; binary: boolean; content: string }>(
      `/projects/${encodeURIComponent(slug)}/blob?${q}`,
    )
  },
  listProjectCommits: (slug: string, ref?: string, path?: string) => {
    const q = new URLSearchParams()
    if (ref) q.set('ref', ref)
    if (path) q.set('path', path)
    const qs = q.toString()
    return request<{ items: GitCommit[] }>(`/projects/${encodeURIComponent(slug)}/commits${qs ? `?${qs}` : ''}`)
  },
  getProject: (slug: string) => request<Project>(`/projects/${encodeURIComponent(slug)}`),
  createProject: (body: {
    slug: string
    name: string
    description?: string
    files_enabled?: boolean
    visibility?: MarketplaceVisibility
    network?: MarketplaceNetwork
    runners?: string[]
  }) => request<Project>('/projects', { method: 'POST', body: JSON.stringify(body) }),
  updateProject: (
    slug: string,
    body: {
      name?: string
      description?: string
      files_enabled?: boolean
      visibility?: MarketplaceVisibility
      network?: MarketplaceNetwork
      runners?: string[]
    },
  ) => request<Project>(`/projects/${encodeURIComponent(slug)}`, { method: 'PATCH', body: JSON.stringify(body) }),
  setProjectMembers: (slug: string, members: { user_id: number; role: ProjectRole }[]) =>
    request<Project>(`/projects/${encodeURIComponent(slug)}/members`, {
      method: 'PUT',
      body: JSON.stringify({ members }),
    }),
  getProjectGit: (slug: string) => request<ProjectGit>(`/projects/${encodeURIComponent(slug)}/git`),
  initProjectGit: (slug: string) =>
    request<ProjectGit>(`/projects/${encodeURIComponent(slug)}/git`, { method: 'POST' }),
  setProtectedBranches: (slug: string, branches: ProtectedBranch[]) =>
    request<ProjectGit>(`/projects/${encodeURIComponent(slug)}/protected-branches`, {
      method: 'PUT',
      body: JSON.stringify({ branches }),
    }),
  listProjectBranches: (slug: string) =>
    request<{ items: string[] }>(`/projects/${encodeURIComponent(slug)}/branches`),
  listMergeRequests: (slug: string, status?: MergeRequestStatus) => {
    const q = status ? `?status=${encodeURIComponent(status)}` : ''
    return request<{ items: MergeRequest[] }>(`/projects/${encodeURIComponent(slug)}/merge-requests${q}`)
  },
  getMergeRequest: (slug: string, iid: number) =>
    request<MergeRequest>(`/projects/${encodeURIComponent(slug)}/merge-requests/${iid}`),
  createMergeRequest: (
    slug: string,
    body: { title: string; description?: string; source_branch: string; target_branch: string },
  ) =>
    request<MergeRequest>(`/projects/${encodeURIComponent(slug)}/merge-requests`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  mergeMergeRequest: (slug: string, iid: number) =>
    request<MergeRequest>(`/projects/${encodeURIComponent(slug)}/merge-requests/${iid}/merge`, { method: 'POST' }),
  closeMergeRequest: (slug: string, iid: number) =>
    request<MergeRequest>(`/projects/${encodeURIComponent(slug)}/merge-requests/${iid}/close`, { method: 'POST' }),
  listCiJobs: (slug: string, workflow?: string) =>
    request<{ items: CiJob[]; workflows: CiWorkflow[] }>(
      `/projects/${encodeURIComponent(slug)}/jobs${workflow ? `?workflow=${encodeURIComponent(workflow)}` : ''}`,
    ),
  getCiJob: (slug: string, n: number) =>
    request<CiJob>(`/projects/${encodeURIComponent(slug)}/jobs/${n}`),
  getCiJobLog: (slug: string, n: number) => requestText(`/projects/${encodeURIComponent(slug)}/jobs/${n}/log`),
  cancelCiJob: (slug: string, n: number) =>
    request<CiJob>(`/projects/${encodeURIComponent(slug)}/jobs/${n}/cancel`, { method: 'POST' }),
  approveCiJob: (slug: string, n: number) =>
    request<CiJob>(`/projects/${encodeURIComponent(slug)}/jobs/${n}/approve`, { method: 'POST' }),
  rerunCiJob: (slug: string, n: number) =>
    request<CiJob>(`/projects/${encodeURIComponent(slug)}/jobs/${n}/rerun`, { method: 'POST' }),
  listProjectRunners: (slug: string) =>
    request<{ items: CiRunner[] }>(`/projects/${encodeURIComponent(slug)}/runners`),
  downloadCiArtifact: (slug: string, n: number) =>
    downloadBinary(`/projects/${encodeURIComponent(slug)}/jobs/${n}/artifact`, `job-${n}-artifact`),

  listServers: () => request<{ items: MeshServer[]; bitlaunch: boolean; accounts: BitLaunchAccount[] }>('/servers'),
  getServer: (id: number) => request<MeshServer>(`/servers/${id}`),
  importServers: () => request<{ items: MeshServer[]; bitlaunch: boolean }>('/servers/import', { method: 'POST' }),
  createServer: (body: {
    name?: string
    hostname: string
    host_id: number
    host_image_id: string
    size_id: string
    region_id: string
    ssh_keys?: string[]
    labels?: string[]
    role?: MeshServerRole
    account_id?: number
  }) => request<MeshServer>('/servers', { method: 'POST', body: JSON.stringify(body) }),
  updateServer: (
    id: number,
    body: { name?: string; labels?: string[]; role?: MeshServerRole; group_id?: number; notes?: string },
  ) => request<MeshServer>(`/servers/${id}`, { method: 'PATCH', body: JSON.stringify(body) }),
  destroyServer: (id: number) => request<void>(`/servers/${id}`, { method: 'DELETE' }),
  rebuildServer: (id: number, hostImageId: string, imageDescription?: string) =>
    request<MeshServer>(`/servers/${id}/rebuild`, {
      method: 'POST',
      body: JSON.stringify({ host_image_id: hostImageId, image_description: imageDescription }),
    }),
  setServerAccess: (id: number, userIds: number[]) =>
    request<{ ok: boolean }>(`/servers/${id}/access`, {
      method: 'PUT',
      body: JSON.stringify({ user_ids: userIds }),
    }),
  issueRunnerToken: (id: number) =>
    request<{ runner_token: string; ci_url: string }>(`/servers/${id}/runner-token`, { method: 'POST' }),
  issueAgentToken: (id: number) =>
    request<{ agent_token: string; svc_url: string }>(`/servers/${id}/agent-token`, { method: 'POST' }),
  listServices: (project?: string) =>
    request<{ items: ManagedService[] }>(project ? `/services?project=${encodeURIComponent(project)}` : '/services'),
  getService: (slug: string) => request<ManagedService>(`/services/${encodeURIComponent(slug)}`),
  createService: (body: {
    slug: string
    kind: ServiceKind
    project_slug?: string
    host: ServiceHost
    mesh_server_id?: number
    bind: ServiceBind
    port?: number
    backends?: string[]
  }) => request<ManagedService>('/services', { method: 'POST', body: JSON.stringify(body) }),
  applyService: (slug: string) =>
    request<ManagedService>(`/services/${encodeURIComponent(slug)}/apply`, { method: 'POST' }),
  stopService: (slug: string) =>
    request<ManagedService>(`/services/${encodeURIComponent(slug)}/stop`, { method: 'POST' }),
  rotateService: (slug: string) =>
    request<ManagedService>(`/services/${encodeURIComponent(slug)}/rotate`, { method: 'POST' }),
  deleteService: (slug: string) => request<{ ok: boolean }>(`/services/${encodeURIComponent(slug)}`, { method: 'DELETE' }),
  listProjectServices: (slug: string) =>
    request<{ items: ManagedService[] }>(`/projects/${encodeURIComponent(slug)}/services`),
  listServerGroups: () => request<{ items: ServerGroup[] }>('/server-groups'),
  createServerGroup: (name: string, description?: string) =>
    request<ServerGroup>('/server-groups', {
      method: 'POST',
      body: JSON.stringify({ name, description }),
    }),
  setServerGroupAccess: (id: number, userIds: number[]) =>
    request<{ ok: boolean; access_user_ids: number[] }>(`/server-groups/${id}/access`, {
      method: 'PUT',
      body: JSON.stringify({ user_ids: userIds }),
    }),
  getComputeSettings: () => request<{ accounts: BitLaunchAccount[]; bitlaunch: boolean }>('/compute/settings'),
  createBitLaunchAccount: (body: { name: string; email: string; token: string }) =>
    request<BitLaunchAccount>('/compute/settings/accounts', { method: 'POST', body: JSON.stringify(body) }),
  updateBitLaunchAccount: (id: number, body: { name: string; email: string; token?: string }) =>
    request<BitLaunchAccount>(`/compute/settings/accounts/${id}`, { method: 'PATCH', body: JSON.stringify(body) }),
  deleteBitLaunchAccount: (id: number) => request<void>(`/compute/settings/accounts/${id}`, { method: 'DELETE' }),
  topUpBitLaunchAccount: (id: number, body: { amount_usd: number; crypto_symbol: 'BTC' | 'LTC' | 'ETH' }) =>
    request<BitLaunchTopUp>(`/compute/settings/accounts/${id}/topup`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),

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
  createSocialPost: (body: string, projectSlug?: string) =>
    request<SocialPost>('/social/posts', {
      method: 'POST',
      body: JSON.stringify({ body, project_slug: projectSlug || undefined }),
    }),
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
