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
      title: 'XDriver',
      description: 'Portal em xdriver.ihuull.com. FileBrowser (xdriver.corp) só responde dentro da VPN.',
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
    prefix: '/my',
    exact: true,
    meta: {
      kicker: 'xvpn',
      title: 'Início',
      description: 'Seus dispositivos VPN. Para adicionar um novo, peça um convite a um administrador.',
    },
  },
]

const ADMIN_PAGES: { prefix: string; exact?: boolean; meta: PageMeta }[] = [
  {
    prefix: '/admin/users',
    meta: {
      kicker: 'Core',
      title: 'Usuários',
      description: 'Contas com acesso à VPN e ao painel.',
    },
  },
  {
    prefix: '/admin/rbac',
    meta: {
      kicker: 'Core',
      title: 'Papéis',
      description: 'Hierarquia RBAC: um ator só gerencia contas no próprio nível ou abaixo.',
    },
  },
  {
    prefix: '/admin/devices',
    meta: {
      kicker: 'Core',
      title: 'Dispositivos',
      description: 'Peers WireGuard cadastrados no servidor.',
    },
  },
  {
    prefix: '/admin/shares',
    meta: {
      kicker: 'Apps',
      title: 'XDriver',
      description: 'Shares Samba + FileBrowser (xdriver.corp) — só na VPN.',
    },
  },
  {
    prefix: '/admin/waitlist',
    meta: {
      kicker: 'Core',
      title: 'Lista de espera',
      description: 'Pedidos de acesso aguardando aprovação.',
    },
  },
  {
    prefix: '/admin/marketplace',
    meta: {
      kicker: 'Apps',
      title: 'Marketplace',
      description: 'Catálogo espelhado de apps/*/marketplace.yaml — ACL e download.',
    },
  },
  {
    prefix: '/admin/settings',
    meta: {
      kicker: 'Settings',
      title: 'Gerais',
      description: 'Rede WireGuard (somente leitura) e TTLs de convite/sessão.',
    },
  },
  {
    prefix: '/admin/audit',
    meta: {
      kicker: 'Settings',
      title: 'Auditoria',
      description: 'Últimas ações administrativas registradas no servidor.',
    },
  },
  {
    prefix: '/admin',
    exact: true,
    meta: {
      kicker: 'Core',
      title: 'Dashboard',
      description: 'Visão geral da VPN em tempo real.',
    },
  },
]

const SOCIAL_PAGES: { prefix: string; exact?: boolean; meta: PageMeta }[] = [
  {
    prefix: '/xgroup/messages',
    meta: { kicker: 'xchat', title: 'Mensagens', description: 'Messenger (xchat). A rede social é o xgroup.' },
  },
  {
    prefix: '/xgroup/groups',
    meta: { kicker: 'xgroup', title: 'Grupos', description: 'Espaços do xgroup. O xchat abre no dock, sem sair desta página.' },
  },
  {
    prefix: '/xgroup/u',
    meta: { kicker: 'xgroup', title: 'Perfil', description: 'Página do membro no xgroup.' },
  },
  {
    prefix: '/xgroup',
    exact: true,
    meta: { kicker: 'xgroup', title: 'Pessoas', description: 'Membros da VPN. Siga e abra o perfil.' },
  },
  {
    prefix: '/social/messages',
    meta: { kicker: 'xchat', title: 'Mensagens', description: 'Messenger (xchat). A rede social é o xgroup.' },
  },
  {
    prefix: '/social/groups',
    meta: { kicker: 'xgroup', title: 'Grupos', description: 'Espaços do xgroup. O xchat abre no dock, sem sair desta página.' },
  },
  {
    prefix: '/social/u',
    meta: { kicker: 'xgroup', title: 'Perfil', description: 'Página do membro no xgroup.' },
  },
  {
    prefix: '/social',
    exact: true,
    meta: { kicker: 'xgroup', title: 'Pessoas', description: 'Membros da VPN. Siga e abra o perfil.' },
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
        kicker: 'xgroup',
        title: 'xgroup',
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
