// Espelha server/internal/store/models.go (Role, roleRank, CanManage) —
// mantenha os dois em sincronia manualmente, já que o frontend não importa
// tipos Go diretamente. Ver PLAN.md §6.7 pela tabela de papéis da Fase 10.
export type Role = 'super_admin' | 'admin' | 'viewer' | 'member'

export const ROLE_RANK: Record<Role, number> = {
  super_admin: 3,
  admin: 2,
  viewer: 1,
  member: 0,
}

export const ROLE_LABELS: Record<Role, string> = {
  super_admin: 'Super admin',
  admin: 'Admin',
  viewer: 'Leitura',
  member: 'Membro',
}

export const ROLE_DESCRIPTIONS: Record<Role, string> = {
  super_admin: 'Controle total: altera papéis, apaga outros admins e gerencia toda a operação.',
  admin: 'Gerencia usuários, dispositivos, waitlist e marketplace — sem promover a super admin.',
  viewer: 'Só leitura no painel de administração (dashboard, listas, auditoria).',
  member: 'Acesso ao próprio espaço: VPN, arquivos, apps e conta. Sem telas de admin.',
}

export type RoleCapability = {
  id: string
  label: string
  roles: Role[]
}

/** Matriz resumida do que cada papel pode no painel — espelha PLAN.md §6.7. */
export const ROLE_CAPABILITIES: RoleCapability[] = [
  { id: 'user-space', label: 'Painel do usuário (Meu espaço)', roles: ['super_admin', 'admin', 'viewer', 'member'] },
  { id: 'own-devices', label: 'Gerenciar próprios dispositivos VPN', roles: ['super_admin', 'admin', 'viewer', 'member'] },
  { id: 'own-account', label: 'Editar a própria conta (senha, chave SSH)', roles: ['super_admin', 'admin', 'viewer', 'member'] },
  { id: 'marketplace-dl', label: 'Baixar apps do catálogo (se a ACL permitir)', roles: ['super_admin', 'admin', 'viewer', 'member'] },
  { id: 'admin-read', label: 'Ler dashboard, usuários, dispositivos e auditoria', roles: ['super_admin', 'admin', 'viewer'] },
  { id: 'admin-write', label: 'Criar usuários, convites e resetar senhas', roles: ['super_admin', 'admin'] },
  { id: 'file-access', label: 'Ligar SFTP/Samba e cota de disco de outros', roles: ['super_admin', 'admin'] },
  { id: 'marketplace-acl', label: 'Gerenciar ACL do marketplace', roles: ['super_admin', 'admin'] },
  { id: 'super', label: 'Promover ou rebaixar super admin', roles: ['super_admin'] },
]

export const ROLE_BADGE_VARIANT: Record<Role, 'default' | 'secondary' | 'outline'> = {
  super_admin: 'default',
  admin: 'secondary',
  viewer: 'outline',
  member: 'outline',
}

// ALL_ROLES define a ordem de exibição em qualquer seletor de papel.
export const ALL_ROLES: Role[] = ['super_admin', 'admin', 'viewer', 'member']

// VIEWER_UP_ROLES/ADMIN_ROLES espelham store.ViewerUpRoles/store.AdminRoles
// — usados tanto para decidir o que a navegação mostra quanto para os
// ProtectedRoute de cada grupo de telas.
export const VIEWER_UP_ROLES: Role[] = ['super_admin', 'admin', 'viewer']
export const ADMIN_ROLES: Role[] = ['super_admin', 'admin']

export function isAdminRole(role: Role | undefined): boolean {
  return role !== undefined && ADMIN_ROLES.includes(role)
}

export function isViewerUpRole(role: Role | undefined): boolean {
  return role !== undefined && VIEWER_UP_ROLES.includes(role)
}

// canManageRole reproduz store.Role.CanManage: um ator só gerencia (edita
// papel, reseta senha, exclui) alvos no próprio nível ou abaixo.
export function canManageRole(actor: Role | undefined, target: Role): boolean {
  if (!actor) return false
  return ROLE_RANK[target] <= ROLE_RANK[actor]
}

// assignableRoles são os papéis que `actor` pode conceder a outro usuário
// (criar ou promover) — mesma regra de canManageRole, mas como lista para
// popular um <Select>.
export function assignableRoles(actor: Role | undefined): Role[] {
  if (!actor) return []
  return ALL_ROLES.filter((r) => canManageRole(actor, r))
}

/** Home pós-login: member → painel do usuário; viewer+ → administração. */
export function defaultRouteForRole(role: Role): string {
  return role === 'member' ? '/my' : '/admin'
}

/** Login alinhado ao namespace que o usuário tentou abrir. */
export function loginPathForLocation(pathname: string): string {
  if (pathname.startsWith('/admin')) return '/admin/login'
  if (pathname.startsWith('/social')) return '/my/login'
  return '/my/login'
}

export function loginPathForRole(role: Role): string {
  return role === 'member' ? '/my/login' : '/admin/login'
}
