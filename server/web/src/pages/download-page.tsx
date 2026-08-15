import { Download, ExternalLink, Monitor, Terminal } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

const CLIENT_README_URL =
  'https://github.com/rootkit-lab/xvpn/blob/main/apps/xvpn-client/README.md'

const PACKAGES = [
  {
    id: 'deb',
    icon: Terminal,
    title: 'Linux (.deb)',
    description:
      'Recomendado em Ubuntu, Debian e Pop!_OS. Instala a GUI e o helper privilegiado (systemd) automaticamente.',
    install: 'sudo apt install ./xvpn-client_*.deb',
    note: 'Depois do install, faça logout/login (ou newgrp xvpn) para o grupo do helper entrar em vigor.',
  },
  {
    id: 'appimage',
    icon: Monitor,
    title: 'Linux (AppImage)',
    description:
      'Binário portátil da GUI. Não sobe o helper sozinho — use o .deb se quiser conexão VPN completa sem passos manuais.',
    install: 'chmod +x xvpn-client-*.AppImage && ./xvpn-client-*.AppImage',
    note: null,
  },
  {
    id: 'windows',
    icon: Download,
    title: 'Windows (instalador NSIS)',
    description:
      'Instalador .exe com atalhos no menu Iniciar. Requer WebView2 (o instalador oferece o bootstrapper se faltar).',
    install: 'Execute xvpn-client-*-installer.exe como administrador.',
    note: 'O registro do helper como Windows Service ainda depende de validação em máquina real — ver ROADMAP Fase 7.',
  },
] as const

export function DownloadPage() {
  return (
    <div className="flex flex-col gap-6">
      <div className="grid gap-4 lg:grid-cols-3">
        {PACKAGES.map(({ id, icon: Icon, title, description, install, note }) => (
          <Card key={id} className="border-white/5 bg-card/60">
            <CardHeader>
              <div className="mb-2 flex size-10 items-center justify-center rounded-full bg-primary/15 text-primary shadow-[0_0_20px_-6px_var(--color-glow)]">
                <Icon className="size-5" />
              </div>
              <CardTitle className="text-base">{title}</CardTitle>
              <CardDescription>{description}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <pre className="overflow-x-auto rounded-xl border border-white/5 bg-background/60 p-3 text-xs text-muted-foreground">
                {install}
              </pre>
              {note && <p className="text-xs text-muted-foreground">{note}</p>}
            </CardContent>
          </Card>
        ))}
      </div>

      <Card className="border-white/5 bg-card/40">
        <CardHeader>
          <CardTitle className="text-base">Depois de instalar</CardTitle>
          <CardDescription>
            Peça um convite ao administrador (tela Usuários), abra o cliente, faça o enrollment com o código e
            conecte. Detalhes de build e desenvolvimento estão no README do cliente.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Button asChild variant="outline" className="rounded-full">
            <a href={CLIENT_README_URL} target="_blank" rel="noreferrer">
              Ver apps/xvpn-client/README.md
              <ExternalLink className="size-4" />
            </a>
          </Button>
        </CardContent>
      </Card>
    </div>
  )
}
