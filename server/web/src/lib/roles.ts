// Espelha server/internal/store/models.go (Role, roleRank, CanManage) —
// mantenha os dois em sincronia manualmente, já que o frontend não importa
// tipos Go diretamente. Ver PLAN.md §6.7 pela tabela de papéis da Fase 10.
import { isProductAppHost, productKind, storeLoginPath } from '@/lib/product-host'

export type Role = 'super_admin' | 'admin' | 'viewer' | 'member'

/** Escopos de produto de um admin — espelha store.Product (PLAN.md §6.13). */
export type Product = 'core' | 'marketplace' | 'xgroup' | 'xdriver' | 'forge' | 'compute' | 'dns' | 'managed'

export const ALL_PRODUCTS: Product[] = [
  'core',
  'marketplace',
  'xgroup',
  'xdriver',
  'forge',
  'compute',
  'dns',
  'managed',
]

export const PRODUCT_LABELS: Record<Product, string> = {
  core: 'Core VPN',
  marketplace: 'Marketplace',
  xgroup: 'XGROUP',
  xdriver: 'XDRIVER',
  forge: 'XGIT',
  compute: 'Compute',
  dns: 'DNS',
  managed: 'Serviços',
}

export const PRODUCT_DESCRIPTIONS: Record<Product, string> = {
  core: 'Peers WireGuard, lista de espera e TTLs do painel.',
  marketplace: 'ACL da loja (quem vê app restrito).',
  xgroup: 'Operação da rede social.',
  xdriver: 'Shares Samba, SFTP e cota de disco.',
  forge: 'XGIT: repositórios, membros, git em xgit.corp, MRs, CI e branches protegidas.',
  compute: 'VPS da malha, contas BitLaunch, saldo, console e token do runner CI.',
  dns: 'Zona corp (dnsmasq) e DNS público (Fase 39).',
  managed: 'Mongo, Redis, Rabbit e LB na malha (Fase 43).',
}

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
  admin: 'Gerencia IAM e os produtos do escopo (lista vazia = todos). Sem promover a super admin.',
  viewer: 'Só leitura no painel de administração (dashboard, listas, auditoria).',
  member: 'Acesso ao próprio espaço: VPN, arquivos, apps, conta e repositórios XGIT em que participa.',
}

export type RoleCapability = {
  id: string
  label: string
  roles: Role[]
}

/** Matriz resumida do que cada papel pode no painel — espelha PLAN.md §6.7. */
export const ROLE_CAPABILITIES: RoleCapability[] = [
  { id: 'user-space', label: 'Painel do usuário (xvpn)', roles: ['super_admin', 'admin', 'viewer', 'member'] },
  { id: 'own-devices', label: 'Gerenciar próprios dispositivos VPN', roles: ['super_admin', 'admin', 'viewer', 'member'] },
  { id: 'own-account', label: 'Editar a própria conta (senha, chave SSH)', roles: ['super_admin', 'admin', 'viewer', 'member'] },
  { id: 'marketplace-dl', label: 'Baixar apps do catálogo (se a ACL permitir)', roles: ['super_admin', 'admin', 'viewer', 'member'] },
  { id: 'admin-read', label: 'Ler dashboard, usuários, dispositivos e auditoria', roles: ['super_admin', 'admin', 'viewer'] },
  { id: 'admin-write', label: 'Criar usuários, convites e resetar senhas', roles: ['super_admin', 'admin'] },
  { id: 'file-access', label: 'Ligar SFTP/Samba e cota (escopo xdriver)', roles: ['super_admin', 'admin'] },
  { id: 'marketplace-acl', label: 'Gerenciar ACL da loja (escopo marketplace)', roles: ['super_admin', 'admin'] },
  { id: 'forge-write', label: 'Criar repositórios XGIT e membros (escopo forge)', roles: ['super_admin', 'admin'] },
  { id: 'product-scope', label: 'Restringir admin a products: [core, marketplace, xgroup, xdriver, forge, compute, dns, managed]', roles: ['super_admin', 'admin'] },
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

/** Home pós-login: app de produto → `/`; portal XVPN → `/`; console → `/admin`. */
export function defaultRouteForRole(role: Role): string {
  if (productKind() === 'xadmin-corp') return role === 'member' ? '/admin/xgit' : '/admin'
  if (isProductAppHost()) return '/'
  if (productKind() === 'xvpn') return '/'
  return role === 'member' ? '/my' : '/admin'
}

/** Login alinhado ao namespace que o usuário tentou abrir. */
export function loginPathForLocation(pathname: string): string {
  if (isProductAppHost()) return storeLoginPath()
  if (pathname.startsWith('/admin')) return '/admin/login'
  if (pathname.startsWith('/social')) return '/my/login'
  return '/my/login'
}

export function loginPathForRole(role: Role): string {
  return role === 'member' ? '/my/login' : '/admin/login'
}

/** Lista vazia = admin irrestrito (Fase 10). super_admin e viewer veem tudo. */
export function hasAdminProduct(
  role: Role | undefined,
  products: Product[] | undefined,
  want: Product,
): boolean {
  if (!role) return false
  if (role === 'super_admin' || role === 'viewer') return true
  if (role !== 'admin') return false
  if (!products || products.length === 0) return true
  return products.includes(want)
}

export function canWriteAdminProduct(
  role: Role | undefined,
  products: Product[] | undefined,
  want: Product,
): boolean {
  if (!role) return false
  if (role === 'super_admin') return true
  if (role !== 'admin') return false
  if (!products || products.length === 0) return true
  return products.includes(want)
}
