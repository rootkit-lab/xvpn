import { profileUsernameFromPath } from '@/lib/social-profile'

export type PageMeta = {
  kicker: string
  title: string
  description: string
}

const USER_PAGES: { prefix: string; exact?: boolean; meta: PageMeta }[] = [
  {
    prefix: '/my/login',
    meta: {
      kicker: 'ihuull',
      title: 'Entrar',
      description: 'SSO ihuull — cookie de sessão no domínio.',
    },
  },
  {
    prefix: '/login',
    exact: true,
    meta: {
      kicker: 'ihuull',
      title: 'Entrar',
      description: 'SSO ihuull — cookie de sessão no domínio.',
    },
  },
  {
    prefix: '/repositories',
    meta: {
      kicker: 'XGIT',
      title: 'Repositórios',
      description: 'Repositórios do usuário logado em xgit.corp.',
    },
  },
  {
    prefix: '/packages',
    exact: true,
    meta: {
      kicker: 'XGIT',
      title: 'Packages',
      description: 'Registry npm/PyPI/generic em xgit.corp no path <org>/<slug>. Maven/NuGet/RubyGems na Fase 59.',
    },
  },
  {
    prefix: '/stars',
    exact: true,
    meta: {
      kicker: 'XGIT',
      title: 'Stars',
      description: 'Repositórios marcados com estrela.',
    },
  },
  {
    prefix: '/my/profile',
    meta: {
      kicker: 'xvpn',
      title: 'Perfil',
      description: 'Como a conta aparece no XVPN — só leitura. Senha e chave SSH ficam em Editar conta.',
    },
  },
  {
    prefix: '/my/account',
    meta: {
      kicker: 'xvpn',
      title: 'Editar minha conta',
      description: 'Troque a senha do painel e a chave SSH extra. Nome e papel só um administrador altera.',
    },
  },
  {
    prefix: '/my/files',
    meta: {
      kicker: 'apps',
      title: 'XDRIVER',
      description: 'Drive nativo em xdriver.corp.ihuull.com — só na VPN. Sem host público.',
    },
  },
  {
    prefix: '/my/marketplace',
    meta: {
      kicker: 'xvpn',
      title: 'Marketplace',
      description: 'Loja em marketplace.ihuull.com. Confira o SHA-256 antes de instalar.',
    },
  },
  {
    prefix: '/my/devices',
    meta: {
      kicker: 'xvpn',
      title: 'Dispositivos',
      description: 'Seus dispositivos VPN. Para adicionar um novo, peça um convite a um administrador.',
    },
  },
  {
    prefix: '/my',
    exact: true,
    meta: {
      kicker: 'xvpn',
      title: 'Início',
      description: 'Portal do produto — status da VPN, download do cliente e atalhos.',
    },
  },
  {
    prefix: '/',
    exact: true,
    meta: {
      kicker: 'xvpn',
      title: 'Início',
      description: 'Portal do produto — status da VPN, download do cliente e atalhos.',
    },
  },
]

const ADMIN_PAGES: { prefix: string; exact?: boolean; meta: PageMeta }[] = [
  {
    prefix: '/admin/login',
    meta: {
      kicker: 'ihuull',
      title: 'Entrar',
      description: 'SSO ihuull — cookie de sessão no domínio.',
    },
  },
  {
    prefix: '/admin/users',
    meta: {
      kicker: 'IAM',
      title: 'Usuários',
      description: 'Contas, papéis e escopo de produtos (products: […]).',
    },
  },
  {
    prefix: '/admin/rbac',
    meta: {
      kicker: 'IAM',
      title: 'Papéis',
      description: 'Identidade de plataforma. ACL da loja e membros de repo são listas outras — ver as quatro camadas.',
    },
  },
  {
    prefix: '/admin/devices',
    meta: {
      kicker: 'Core VPN',
      title: 'Dispositivos',
      description: 'Peers WireGuard cadastrados no servidor.',
    },
  },
  {
    prefix: '/admin/shares',
    meta: {
      kicker: 'XDRIVER',
      title: 'Shares e Drive',
      description: 'Samba + Drive nativo (xdriver.corp) — só na VPN.',
    },
  },
  {
    prefix: '/admin/waitlist',
    meta: {
      kicker: 'Core VPN',
      title: 'Lista de espera',
      description: 'Pedidos de acesso aguardando aprovação.',
    },
  },
  {
    prefix: '/admin/xgit/settings',
    meta: {
      kicker: 'XGIT',
      title: 'Configurações',
      description: 'Visibility/network padrão, quem cria repositório e clone só em xgit.corp.',
    },
  },
  {
    prefix: '/admin/xgit',
    meta: {
      kicker: 'XGIT',
      title: 'Repositórios',
      description: 'Forge: no xadmin lista todos os repos; em xgit.corp só os do membro. ACL do app no Marketplace.',
    },
  },
  {
    prefix: '/admin/projects',
    meta: {
      kicker: 'XGIT',
      title: 'Repositórios',
      description: 'Redireciona para /admin/xgit. Um slug, membros, git em xgit.corp, MRs e branches protegidas.',
    },
  },
  {
    prefix: '/admin/compute/settings',
    meta: {
      kicker: 'Compute',
      title: 'Configurações',
      description: 'Contas BitLaunch (e-mail + API), saldo e recarga cripto. Token só no VPS.',
    },
  },
  {
    prefix: '/admin/services',
    meta: {
      kicker: 'Serviços',
      title: 'Instâncias',
      description: 'Mongo, Redis, Rabbit e LB no local ou na malha. Bind só wg0/loopback. Sem 27017 do control-plane.',
    },
  },
  {
    prefix: '/admin/servers',
    meta: {
      kicker: 'Compute',
      title: 'Servidores',
      description: 'Malha: BitLaunch + cadastro manual (nó data). Console xterm e observações no detalhe. Hosts externos só inventário.',
    },
  },
  {
    prefix: '/admin/marketplace/acl',
    meta: {
      kicker: 'Marketplace',
      title: 'ACL da loja',
      description: 'Quem vê/baixa um app restricted (AppAccess). Não concede clone nem papel de plataforma.',
    },
  },
  {
    prefix: '/admin/marketplace/catalog',
    meta: {
      kicker: 'Marketplace',
      title: 'Catálogo',
      description: 'Espelho de apps/*/marketplace.yaml — kind, network, versões e assets.',
    },
  },
  {
    prefix: '/admin/marketplace',
    meta: {
      kicker: 'Marketplace',
      title: 'Catálogo',
      description: 'Espelho de apps/*/marketplace.yaml — kind, network, versões e assets.',
    },
  },
  {
    prefix: '/admin/xgroup',
    meta: {
      kicker: 'XGROUP',
      title: 'Rede social',
      description: 'Operação do XGROUP. A rede em si vive em /social.',
    },
  },
  {
    prefix: '/admin/settings',
    meta: {
      kicker: 'Core VPN',
      title: 'Gerais',
      description: 'Rede WireGuard (somente leitura), assistente XCODESPACES e TTLs de convite/sessão.',
    },
  },
  {
    prefix: '/admin/backups',
    meta: {
      kicker: 'Core VPN',
      title: 'Backups',
      description: 'Destinos off-site (restic + rclone). Credenciais só no VPS. Dry-run e último job.',
    },
  },
  {
    prefix: '/admin/dns/settings',
    meta: {
      kicker: 'DNS',
      title: 'Configurações',
      description: 'Contas Cloudflare. Token só no VPS. Nameservers do stack saem daqui.',
    },
  },
  {
    prefix: '/admin/dns/public',
    meta: {
      kicker: 'DNS',
      title: 'Zonas públicas',
      description: 'Domínios do stack. NS no registrador. Visão interna no dnsmasq.',
    },
  },
  {
    prefix: '/admin/dns',
    meta: {
      kicker: 'DNS',
      title: 'DNS intranet',
      description: 'Zona corp.ihuull.com no dnsmasq (10.66.66.1:53, só wg0).',
    },
  },
  {
    prefix: '/admin/audit',
    meta: {
      kicker: 'IAM',
      title: 'Auditoria',
      description: 'Últimas ações administrativas registradas no servidor.',
    },
  },
  {
    prefix: '/admin',
    exact: true,
    meta: {
      kicker: 'Core VPN',
      title: 'Dashboard',
      description: 'Visão geral da VPN em tempo real.',
    },
  },
]

const SOCIAL_PAGES: { prefix: string; exact?: boolean; meta: PageMeta }[] = [
  {
    prefix: '/xgroup/messages',
    meta: { kicker: 'XCHAT', title: 'Mensagens', description: 'Messenger (XCHAT Client). A rede social é o XGROUP.' },
  },
  {
    prefix: '/xgroup/groups',
    meta: { kicker: 'XGROUP', title: 'Grupos', description: 'Espaços do XGROUP. O XCHAT abre no dock, sem sair desta página.' },
  },
  {
    prefix: '/xgroup/u',
    meta: { kicker: 'XGROUP', title: 'Perfil', description: 'Página do membro no XGROUP.' },
  },
  {
    prefix: '/xgroup/explore',
    meta: { kicker: 'XGROUP', title: 'Explorar', description: 'Encontre pessoas na VPN e siga.' },
  },
  {
    prefix: '/xgroup',
    exact: true,
    meta: { kicker: 'XGROUP', title: 'Início', description: 'O que está acontecendo na VPN.' },
  },
  {
    prefix: '/social/messages',
    meta: { kicker: 'XCHAT', title: 'Mensagens', description: 'Messenger (XCHAT Client). A rede social é o XGROUP.' },
  },
  {
    prefix: '/social/groups',
    meta: { kicker: 'XGROUP', title: 'Grupos', description: 'Espaços do XGROUP. O XCHAT abre no dock, sem sair desta página.' },
  },
  {
    prefix: '/social/u',
    meta: { kicker: 'XGROUP', title: 'Perfil', description: 'Página do membro no XGROUP.' },
  },
  {
    prefix: '/social/explore',
    meta: { kicker: 'XGROUP', title: 'Explorar', description: 'Encontre pessoas na VPN e siga.' },
  },
  {
    prefix: '/social',
    exact: true,
    meta: { kicker: 'XGROUP', title: 'Início', description: 'O que está acontecendo na VPN.' },
  },
]

function matchMeta(pathname: string, table: { prefix: string; exact?: boolean; meta: PageMeta }[]): PageMeta | null {
  for (const row of table) {
    if (row.exact) {
      if (pathname === row.prefix) return row.meta
      continue
    }
    if (pathname === row.prefix || pathname.startsWith(`${row.prefix}/`)) return row.meta
  }
  return null
}

export function pageMetaForPath(pathname: string, hostname?: string): PageMeta {
  if (pathname.includes('/actions/new')) {
    return {
      kicker: 'XGIT',
      title: 'New workflow',
      description: 'Galeria de templates de CI no estilo GitHub Actions (Fase 42.2).',
    }
  }
  if (/\/actions(\/|$)/.test(pathname) && !pathname.startsWith('/admin/audit')) {
    return {
      kicker: 'XGIT',
      title: 'Actions',
      description: 'Runs e workflows do repositório no XGIT.',
    }
  }
  const profile = profileUsernameFromPath(pathname, hostname)
  if (profile) {
    return { kicker: 'XGROUP', title: profile, description: 'Página do membro no XGROUP.' }
  }
  if (pathname.startsWith('/admin')) {
    return (
      matchMeta(pathname, ADMIN_PAGES) ?? {
        kicker: 'Administração',
        title: 'Painel',
        description: '',
      }
    )
  }
  if (pathname.startsWith('/social') || pathname.startsWith('/xgroup') || pathname.startsWith('/xchat')) {
    return (
      matchMeta(pathname, SOCIAL_PAGES) ?? {
        kicker: 'XGROUP',
        title: 'XGROUP',
        description: '',
      }
    )
  }
  return (
    matchMeta(pathname, USER_PAGES) ?? {
      kicker: 'xvpn',
      title: '',
      description: '',
    }
  )
}
