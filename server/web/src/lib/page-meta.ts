export type PageMeta = {
  kicker: string
  title: string
  description: string
}

const USER_PAGES: { prefix: string; exact?: boolean; meta: PageMeta }[] = [
  {
    prefix: '/my/profile',
    meta: {
      kicker: 'Meu espaço',
      title: 'Perfil',
      description: 'Como a conta aparece no XVPN — só leitura. Senha e chave SSH ficam em Editar conta.',
    },
  },
  {
    prefix: '/my/account',
    meta: {
      kicker: 'Meu espaço',
      title: 'Editar minha conta',
      description: 'Troque a senha do painel e a chave SSH extra. Nome e papel só um administrador altera.',
    },
  },
  {
    prefix: '/my/files',
    meta: {
      kicker: 'Meu espaço',
      title: 'Arquivos',
      description: 'Samba, SFTP e FileBrowser só respondem dentro da VPN.',
    },
  },
  {
    prefix: '/my/download',
    meta: {
      kicker: 'Meu espaço',
      title: 'Downloads',
      description: 'Cliente desktop do XVPN — .deb, AppImage e instalador Windows.',
    },
  },
  {
    prefix: '/my/marketplace',
    meta: {
      kicker: 'Meu espaço',
      title: 'Apps',
      description: 'Programas liberados para a sua conta. Confira o SHA-256 antes de instalar.',
    },
  },
  {
    prefix: '/my',
    exact: true,
    meta: {
      kicker: 'Meu espaço',
      title: 'Início',
      description: 'Seus dispositivos VPN. Para adicionar um novo, peça um convite a um administrador.',
    },
  },
]

const ADMIN_PAGES: { prefix: string; exact?: boolean; meta: PageMeta }[] = [
  {
    prefix: '/admin/users',
    meta: {
      kicker: 'Administração',
      title: 'Usuários',
      description: 'Contas com acesso à VPN e ao painel.',
    },
  },
  {
    prefix: '/admin/rbac',
    meta: {
      kicker: 'Administração',
      title: 'Papéis',
      description: 'Hierarquia RBAC: um ator só gerencia contas no próprio nível ou abaixo.',
    },
  },
  {
    prefix: '/admin/devices',
    meta: {
      kicker: 'Administração',
      title: 'Dispositivos',
      description: 'Peers WireGuard cadastrados no servidor.',
    },
  },
  {
    prefix: '/admin/shares',
    meta: {
      kicker: 'Administração',
      title: 'Compartilhamentos',
      description: 'Diretórios do VPS acessíveis só pela VPN.',
    },
  },
  {
    prefix: '/admin/waitlist',
    meta: {
      kicker: 'Administração',
      title: 'Lista de espera',
      description: 'Pedidos de acesso aguardando aprovação.',
    },
  },
  {
    prefix: '/admin/download',
    meta: {
      kicker: 'Administração',
      title: 'Downloads',
      description: 'Cliente desktop do XVPN — .deb, AppImage e instalador Windows.',
    },
  },
  {
    prefix: '/admin/marketplace',
    meta: {
      kicker: 'Administração',
      title: 'Marketplace',
      description: 'Catálogo espelhado de apps/*/marketplace.yaml — ACL e download.',
    },
  },
  {
    prefix: '/admin/settings',
    meta: {
      kicker: 'Administração',
      title: 'Configurações',
      description: 'Rede WireGuard (somente leitura) e TTLs de convite/sessão.',
    },
  },
  {
    prefix: '/admin/audit',
    meta: {
      kicker: 'Administração',
      title: 'Auditoria',
      description: 'Últimas ações administrativas registradas no servidor.',
    },
  },
  {
    prefix: '/admin',
    exact: true,
    meta: {
      kicker: 'Administração',
      title: 'Dashboard',
      description: 'Visão geral da VPN em tempo real.',
    },
  },
]

const SOCIAL_PAGES: { prefix: string; exact?: boolean; meta: PageMeta }[] = [
  {
    prefix: '/social/messages',
    meta: { kicker: 'Social', title: 'Mensagens', description: 'Messenger da organização — a rede continua em Pessoas e Grupos.' },
  },
  {
    prefix: '/social/groups',
    meta: { kicker: 'Social', title: 'Grupos', description: 'Espaços da organização. O chat abre no dock, sem sair desta página.' },
  },
  {
    prefix: '/social/u',
    meta: { kicker: 'Social', title: 'Perfil', description: 'Página pública do membro na organização.' },
  },
  {
    prefix: '/social',
    exact: true,
    meta: { kicker: 'Social', title: 'Pessoas', description: 'Membros da VPN. Siga e abra o perfil.' },
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
  if (pathname.startsWith('/social')) {
    return (
      matchMeta(pathname, SOCIAL_PAGES) ?? {
        kicker: 'Social',
        title: 'XVPN Social',
        description: '',
      }
    )
  }
  return (
    matchMeta(pathname, USER_PAGES) ?? {
      kicker: 'Meu espaço',
      title: 'Painel',
      description: '',
    }
  )
}
