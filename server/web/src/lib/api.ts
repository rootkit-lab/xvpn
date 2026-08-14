// Cliente HTTP único para a API do control-plane XVPN — ver
// .cursor/rules/frontend-react.mdc (nunca `fetch` espalhado por
// componentes, sempre passar por aqui para tratamento de erro/auth
// consistente).
import type { Role } from '@/lib/roles'

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
// JSON puro (ver comentário em uploadMarketplaceAsset).
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
  if (path.startsWith('/auth/login')) return
  clearToken()
  if (window.location.pathname !== '/login') {
    window.location.href = '/login'
  }
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers)
  headers.set('Content-Type', 'application/json')
  const token = getToken()
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }

  const res = await fetch(`/api${path}`, { ...options, headers })

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

// uploadMarketplaceAsset não passa por request(): o endpoint espera
// multipart/form-data (ver marketplace_handler.go), e request() sempre
// força Content-Type: application/json. O boundary do multipart precisa
// ser gerado pelo próprio browser — nunca setamos Content-Type manualmente
// aqui, senão o FormData vai sem boundary e o Gin não consegue parsear.
async function uploadMarketplaceAsset(
  versionId: number,
  file: File,
  platform: MarketplacePlatform,
  arch: string,
): Promise<MarketplaceAsset> {
  const form = new FormData()
  form.append('platform', platform)
  if (arch.trim()) form.append('arch', arch.trim())
  form.append('file', file)

  const headers = new Headers()
  const token = getToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)

  const path = `/marketplace/versions/${versionId}/assets`
  const res = await fetch(`/api${path}`, { method: 'POST', headers, body: form })

  if (res.status === 401) {
    handleUnauthorized(path)
  }
  if (!res.ok) {
    throw new ApiError(res.status, await parseErrorMessage(res))
  }
  return (await res.json()) as MarketplaceAsset
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
  const res = await fetch(`/api${path}`, { headers })

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

export interface User {
  id: number
  username: string
  role: Role
  created_at: string
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

// Espelha store.Platform/store.AppVisibility (server/internal/store/models.go)
// — Fase 11, ver PLAN.md §6.8.
export type MarketplacePlatform = 'linux' | 'windows' | 'android'
export type MarketplaceVisibility = 'global' | 'restricted'
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
  name: string
  description: string
  icon_url?: string
  visibility: MarketplaceVisibility
  created_at: string
  versions: MarketplaceVersion[]
  // access_user_ids só vem preenchido quando quem pediu administra o
  // marketplace (ver handleListMarketplaceApps no servidor) — nunca
  // presente na resposta pra member/viewer, mesmo em apps que eles
  // enxergam.
  access_user_ids?: number[]
}

export const api = {
  login: (username: string, password: string) =>
    request<{ token: string; user: User }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  // me restaura {id, username, role} depois de um refresh de página — o
  // token em localStorage sozinho não é decodificado no cliente.
  me: () => request<User>('/auth/me'),

  status: () => request<StatusResponse>('/status'),

  listUsers: () => request<User[]>('/users'),
  createUser: (username: string, password: string, role: Role) =>
    request<User>('/users', {
      method: 'POST',
      body: JSON.stringify({ username, password, role }),
    }),
  updateUser: (id: number, changes: { username?: string; role?: Role }) =>
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

  listDevices: () => request<Device[]>('/devices'),
  deleteDevice: (id: number) => request<void>(`/devices/${id}`, { method: 'DELETE' }),

  // listMyDevices/deleteMyDevice são o autosserviço da Fase 10 (ver
  // PLAN.md §6.7): qualquer papel autenticado gerencia os próprios
  // dispositivos, sem precisar das telas administrativas.
  listMyDevices: () => request<Device[]>('/me/devices'),
  deleteMyDevice: (id: number) => request<void>(`/me/devices/${id}`, { method: 'DELETE' }),

  listAudit: () => request<AuditLog[]>('/audit'),

  getConfig: () => request<ConfigResponse>('/config'),

  // joinWaitlist é o único endpoint de escrita público (sem
  // autenticação) de toda a API — chamado da landing page em "/". Ver
  // PLAN.md pela decisão de design e o rate limit aplicado no backend.
  joinWaitlist: (name: string, email: string, message: string) =>
    request<WaitlistEntry>('/waitlist', {
      method: 'POST',
      body: JSON.stringify({ name, email, message }),
    }),
  listWaitlist: () => request<WaitlistEntry[]>('/waitlist'),
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

  // Catálogo do marketplace (Fase 11, PLAN.md §6.8): listMarketplaceApps já
  // vem filtrado por ACL pelo servidor — o front nunca decide sozinho o
  // que esconder, só reflete o que a API devolveu.
  listMarketplaceApps: () => request<MarketplaceApp[]>('/marketplace/apps'),
  createMarketplaceApp: (input: {
    name: string
    description?: string
    icon_url?: string
    visibility?: MarketplaceVisibility
  }) => request<MarketplaceApp>('/marketplace/apps', { method: 'POST', body: JSON.stringify(input) }),
  updateMarketplaceApp: (
    id: number,
    changes: { name?: string; description?: string; icon_url?: string; visibility?: MarketplaceVisibility },
  ) => request<MarketplaceApp>(`/marketplace/apps/${id}`, { method: 'PATCH', body: JSON.stringify(changes) }),
  deleteMarketplaceApp: (id: number) => request<void>(`/marketplace/apps/${id}`, { method: 'DELETE' }),
  setMarketplaceAppAccess: (id: number, userIds: number[]) =>
    request<{ user_ids: number[] }>(`/marketplace/apps/${id}/access`, {
      method: 'PUT',
      body: JSON.stringify({ user_ids: userIds }),
    }),
  createMarketplaceVersion: (appId: number, input: { version: string; channel?: MarketplaceChannel; changelog?: string }) =>
    request<MarketplaceVersion>(`/marketplace/apps/${appId}/versions`, { method: 'POST', body: JSON.stringify(input) }),
  deleteMarketplaceVersion: (id: number) => request<void>(`/marketplace/versions/${id}`, { method: 'DELETE' }),
  deleteMarketplaceAsset: (id: number) => request<void>(`/marketplace/assets/${id}`, { method: 'DELETE' }),
  uploadMarketplaceAsset,
  downloadMarketplaceAsset,
}
