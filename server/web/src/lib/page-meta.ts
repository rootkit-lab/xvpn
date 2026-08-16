export type PageMeta = {
  kicker: string
  title: string
  description: string
}

const USER_PAGES: { prefix: string; exact?: boolean; meta: PageMeta }[] = [
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
]

const ADMIN_PAGES: { prefix: string; exact?: boolean; meta: PageMeta }[] = [
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
      description: 'Hierarquia RBAC e escopo de produto. Um ator só gerencia contas no próprio nível ou abaixo.',
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
    prefix: '/admin/marketplace',
    meta: {
      kicker: 'Marketplace',
      title: 'Catálogo e ACL',
      description: 'Catálogo espelhado de apps/*/marketplace.yaml — ACL, network e download.',
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
      description: 'Rede WireGuard (somente leitura) e TTLs de convite/sessão.',
    },
  },
  {
    prefix: '/admin/dns',
    meta: {
      kicker: 'Core VPN',
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

export function pageMetaForPath(pathname: string): PageMeta {
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
      title: 'Painel',
      description: '',
    }
  )
}
