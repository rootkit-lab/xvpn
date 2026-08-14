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

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers)
  headers.set('Content-Type', 'application/json')
  const token = getToken()
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }

  const res = await fetch(`/api${path}`, { ...options, headers })

  if (res.status === 401 && !path.startsWith('/auth/login')) {
    clearToken()
    if (window.location.pathname !== '/login') {
      window.location.href = '/login'
    }
  }

  if (!res.ok) {
    let message = `Erro ${res.status}`
    try {
      const body = (await res.json()) as { error?: string }
      if (body?.error) message = body.error
    } catch {
      // corpo não é JSON (ex.: 502 do Nginx) — mantém mensagem genérica
    }
    throw new ApiError(res.status, message)
  }

  if (res.status === 204) {
    return undefined as T
  }
  return (await res.json()) as T
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
}
