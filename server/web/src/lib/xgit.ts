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

/** Lista de repositórios (aba Repositories no corp; console no xadmin). */
export function xgitReposPath(): string {
  return productKind() === 'xgit-corp' ? '/repositories' : '/admin/xgit'
}

export function xgitRepoPath(org: string, slug: string, rest = ''): string {
  const suffix = rest.replace(/^\//, '')
  return xgitPath(suffix ? `${org}/${slug}/${suffix}` : `${org}/${slug}`)
}

export function repoRef(org: string, slug: string): string {
  return `${org}/${slug}`
}

const LANG_TOKEN: Record<string, string> = {
  Go: 'var(--primary)',
  TypeScript: 'var(--product-xgit)',
  JavaScript: 'var(--product-xdriver)',
  Shell: 'var(--safe)',
  Python: 'var(--product-xchat)',
  SCSS: 'var(--product-xgroup)',
  CSS: 'var(--product-marketplace)',
  Markdown: 'var(--muted-foreground)',
  YAML: 'var(--muted-foreground)',
}

export function languageColor(name?: string): string {
  if (!name) return 'var(--muted-foreground)'
  return LANG_TOKEN[name] ?? 'var(--foreground)'
}
