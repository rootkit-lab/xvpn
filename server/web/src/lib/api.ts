// Cliente HTTP único para a API do control-plane XVPN — ver
// .cursor/rules/frontend-react.mdc (nunca `fetch` espalhado por
// componentes, sempre passar por aqui para tratamento de erro/auth
// consistente).

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

export interface StatusResponse {
  api_version: number
  uptime_seconds: number
  connected_peers: number
  total_peers: number
}

export interface AuditLog {
  id: number
  actor: string
  action: string
  detail: string
  created_at: string
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
    request<{ token: string }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),

  status: () => request<StatusResponse>('/status'),

  listUsers: () => request<User[]>('/users'),
  createUser: (username: string, password: string) =>
    request<User>('/users', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  deleteUser: (id: number) => request<void>(`/users/${id}`, { method: 'DELETE' }),
  createInvite: (userId: number) => request<InviteResponse>(`/users/${userId}/invite`, { method: 'POST' }),

  listDevices: () => request<Device[]>('/devices'),
  deleteDevice: (id: number) => request<void>(`/devices/${id}`, { method: 'DELETE' }),

  listAudit: () => request<AuditLog[]>('/audit'),

  getConfig: () => request<ConfigResponse>('/config'),
}
