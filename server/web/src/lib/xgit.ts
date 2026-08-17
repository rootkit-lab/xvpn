import { productKind } from '@/lib/product-host'

/** Prefixo das rotas do forge: app em xgit.corp ou console no xadmin. */
export function xgitPath(suffix = ''): string {
  const rest = suffix.replace(/^\//, '')
  if (productKind() === 'xgit-corp') {
    return rest ? `/${rest}` : '/'
  }
  return rest ? `/admin/xgit/${rest}` : '/admin/xgit'
}

export function isXgitAdminHost(): boolean {
  return productKind() === 'xadmin-corp'
}
