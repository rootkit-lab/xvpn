# ROADMAP — XVPN

Checklist de execução do projeto, fase a fase. Baseado nas decisões arquiteturais de [`PLAN.md`](./PLAN.md). Marque os itens conforme forem concluídos — este arquivo é a fonte da verdade sobre "o que já foi feito" no projeto.

Convenção: `[ ]` pendente · `[x]` concluído · `[~]` em andamento/parcial.

---

## Preparação do repositório e tooling

- [x] `PLAN.md` criado com arquitetura completa e decisões justificadas
- [x] `README.md` criado
- [x] `ROADMAP.md` criado (este arquivo)
- [x] `AGENTS.md` criado (instruções para agentes de IA)
- [x] `CONTRIBUTING.md` criado
- [x] `SECURITY.md` criado
- [x] `CHANGELOG.md` criado
- [x] Regras do Cursor configuradas (`.cursor/rules/*.mdc`)
- [x] Hooks do Cursor configurados (`.cursor/hooks.json`)
- [x] Skills do Cursor configuradas (`.cursor/skills/*`)
- [x] `.gitignore` completo criado (segredos, artefatos de build, DB, SO/IDE)
- [x] Convenção de build documentada em [`PLAN.md` §11.1](./PLAN.md#111-convenção-de-build-e-artefatos-o-que-é-gerado-onde-fica-é-commitado)
- [x] Hook real de pre-commit criado (`.githooks/pre-commit`)
- [x] Repositório Git inicializado (`git init`) e primeiro commit
- [x] `core.hooksPath` configurado localmente (`.githooks`)
- [x] Repositório remoto criado no GitHub (`rootkit-lab/xvpn`, público — ver `SECURITY.md`) e push inicial
- [x] Repositório configurado para squash merge apenas (merge commit e rebase merge desabilitados)
- [x] Fluxo de trabalho GitHub Flow documentado e obrigatório (`CONTRIBUTING.md`)
- [x] Hook `.githooks/pre-commit` bloqueando commit direto em `main`/`master` (exceto merge)
- [x] Branch protection real aplicada em `main` no GitHub (PR obrigatório, sem push direto, sem force-push, sem deleção, histórico linear, `enforce_admins` ativo) — validado com teste de push direto rejeitado
- [ ] Definir e adicionar `LICENSE` (pendente — ver README.md)

---

## Fase 0 — Hardening e provisionamento base do VPS

- [ ] Verificar efetivo do SSH: `sshd -T | grep -i passwordauthentication`
- [ ] Criar `/etc/ssh/sshd_config.d/99-xvpn-hardening.conf` (`PasswordAuthentication no`, `PermitRootLogin prohibit-password`, `KbdInteractiveAuthentication no`)
- [ ] Recarregar sshd e confirmar (`systemctl reload sshd` + reconectar para validar antes de fechar a sessão atual)
- [ ] Criar usuário de sistema `xvpn` (sem shell de login interativo, home dedicada em `/opt/xvpn`)
- [ ] Instalar pacotes base: `nginx`, `certbot`, `python3-certbot-nginx`, `samba`, `fail2ban`, `unattended-upgrades`
- [ ] Configurar `ufw`: política padrão `deny incoming` / `allow outgoing`
- [ ] `ufw allow 22/tcp`, `ufw allow 80/tcp`, `ufw allow 443/tcp`, `ufw allow 51820/udp`
- [ ] Ativar `ufw` (`ufw enable`) e confirmar com `ufw status verbose`
- [ ] Configurar `fail2ban` para o serviço SSH
- [ ] Habilitar e configurar `unattended-upgrades` (patches de segurança automáticos)
- [ ] Criar server block Nginx para `vpn.officeempresa.com` (proxy para `127.0.0.1:8080`, ainda sem backend — pode retornar 502 temporariamente)
- [ ] Emitir certificado: `certbot --nginx -d vpn.officeempresa.com`
- [ ] Confirmar renovação automática do certificado (`systemctl list-timers | grep certbot`)
- [ ] Coordenar com o setup do `landpages-ops` para não haver conflito de *server block* em `ldpops.appapisip.com`
- [ ] Registrar em [`PLAN.md` §5](./PLAN.md#5-alocação-de-rede-portas-e-domínios-registro-para-não-colidir-com-landpages-ops) qualquer porta/domínio novo definido nesta fase

## Fase 1 — Validação manual do túnel WireGuard

- [ ] Confirmar módulo carregado: `modprobe wireguard` + `lsmod | grep wireguard`
- [ ] Criar interface: `ip link add dev wg0 type wireguard`
- [ ] Gerar par de chaves do servidor (`wg genkey | tee server.key | wg pubkey > server.pub`), salvar chave privada em `/etc/wireguard/server.key` com permissão `600`
- [ ] Atribuir IP à interface: `ip addr add 10.66.66.1/24 dev wg0`
- [ ] Configurar a interface com a chave privada e porta de escuta (`wg set wg0 listen-port 51820 private-key /etc/wireguard/server.key`)
- [ ] Subir a interface: `ip link set wg0 up`
- [ ] Habilitar `net.ipv4.ip_forward=1` (persistente via `/etc/sysctl.d/99-xvpn.conf`)
- [ ] Configurar regra de NAT/MASQUERADE (`nftables` ou `iptables`) para `10.66.66.0/24` saindo por `eth0`
- [ ] Gerar par de chaves de um peer de teste (no seu notebook/desktop)
- [ ] Adicionar peer de teste no servidor: `wg set wg0 peer <pubkey_cliente> allowed-ips 10.66.66.2/32`
- [ ] Configurar peer de teste localmente (`Endpoint = 206.189.224.72:51820`, `AllowedIPs = 0.0.0.0/0, ::/0`, `PersistentKeepalive = 25`)
- [ ] Subir túnel no peer de teste e validar handshake (`wg show` nos dois lados)
- [ ] Validar exit: `curl ifconfig.me` de dentro do túnel deve retornar `206.189.224.72`
- [ ] Validar latência/throughput básico (`ping`, opcionalmente `iperf3`)
- [ ] Validar que o peer de teste consegue alcançar `10.66.66.1` (o próprio servidor) — confirma que "estar na mesma rede" funciona, não só o exit
- [ ] Documentar quaisquer ajustes de MTU/roteamento encontrados

## Fase 2 — Control-plane API (Go)

- [ ] Criar `server/` e inicializar módulo Go (`go mod init`)
- [ ] Modelagem de dados via GORM: `User`, `Device`/`Peer`, `InviteToken`, `AuditLog`
- [ ] Camada de autenticação: hash de senha com Argon2id, emissão/validação de JWT
- [ ] Pacote `internal/wireguard/`: wrapper sobre `wgctrl-go` (`CreateInterface`, `AddPeer`, `RemovePeer`, `ListPeers` com estatísticas rx/tx/handshake)
- [ ] Endpoint `POST /api/auth/login`
- [ ] Endpoints CRUD `GET/POST/DELETE /api/users`
- [ ] Endpoint `POST /api/users/:id/invite` (gera token de convite, expira em 15 min)
- [ ] Endpoint `POST /api/devices/enroll` (recebe chave pública + token, aloca IP livre em `10.66.66.0/24`, registra peer via `wgctrl`)
- [ ] Endpoint `GET /api/devices` (lista peers + estatísticas ao vivo)
- [ ] Endpoint `DELETE /api/devices/:id` (revoga peer imediatamente)
- [ ] Endpoint `GET /api/status` (saúde do servidor, nº de peers conectados)
- [ ] Testes unitários dos handlers principais e da camada `wireguard/`
- [ ] `systemd` unit `xvpn-server.service` com `AmbientCapabilities=CAP_NET_ADMIN` (rodando como usuário `xvpn`, não root)
- [ ] Apontar o server block Nginx de `vpn.officeempresa.com` para `127.0.0.1:8080` (backend real)
- [ ] Configurar backup automático do `xvpn.db` (cron + `sqlite3 .backup`, rotação de 7 dias)

## Fase 3 — Painel Web (React + Tailwind + shadcn/ui)

- [ ] Scaffold Vite + React + TypeScript em `server/web/`
- [ ] Configurar TailwindCSS + shadcn/ui
- [ ] Tela de Login
- [ ] Dashboard (peers ativos, throughput agregado, status geral)
- [ ] Tela Usuários (CRUD + gerar convite / QR code)
- [ ] Tela Dispositivos (status, último handshake, revogar)
- [ ] Tela Compartilhamentos (gerenciar shares Samba/FileBrowser e permissões)
- [ ] Tela Configurações (rede, DNS, firewall)
- [ ] Tela Auditoria (log de conexões/ações administrativas)
- [ ] Build do painel embutido no binário Go via `embed.FS`
- [ ] Teste end-to-end manual: criar usuário → gerar convite → dispositivo aparece conectado no painel

## Fase 4 — Cliente Desktop MVP (Wails3)

- [ ] Criar `client/` e inicializar com `wails3 init` (Go + React)
- [ ] Configurar TailwindCSS + shadcn/ui no frontend do cliente
- [ ] Implementar helper privilegiado (`internal/tunnel/`: `wireguard-go` + `wgctrl-go`)
- [ ] Implementar TUN no Linux (dispositivo `wg` nativo do kernel)
- [ ] Implementar TUN no Windows (`wintun` embutido via `go:embed`)
- [ ] Implementar IPC GUI ↔ Helper (JSON-RPC via Unix Socket no Linux / Named Pipe no Windows)
- [ ] Tela de enrollment (inserir código de convite gerado no painel)
- [ ] Tela principal: Conectar/Desconectar, status (IP, latência, throughput, tempo conectado)
- [ ] Ícone de bandeja (tray) básico
- [ ] Instalação do serviço/helper (systemd unit no Linux / Windows Service no instalador)
- [ ] Testar enrollment e conexão ponta a ponta no Linux
- [ ] Testar enrollment e conexão ponta a ponta no Windows

## Fase 5 — Compartilhamento de arquivos

- [ ] Instalar e configurar Samba (`bind interfaces only = yes`, `interfaces = wg0 lo`)
- [ ] Criar shares iniciais (ex.: `[shared]`, `[home-<usuario>]`)
- [ ] (Opcional) Sincronizar criação de usuário Samba com criação de usuário XVPN via painel
- [ ] Instalar FileBrowser, `systemd` unit `xvpn-filebrowser.service`, bind exclusivo em `10.66.66.1:8081`
- [ ] Botão no cliente desktop: "Abrir arquivos do servidor" (unidade de rede e/ou FileBrowser)
- [ ] Validar externamente (fora da VPN) que Samba e FileBrowser são **inacessíveis** via `eth0`/IP público

## Fase 6 — Recursos avançados do cliente

- [ ] Kill switch (`nftables` no Linux / Windows Filtering Platform no Windows)
- [ ] Reconexão automática com backoff exponencial
- [ ] Ícone de bandeja completo (status visual, atalhos rápidos)
- [ ] Auto-start no boot do sistema operacional (opcional, configurável)
- [ ] Split-tunnel opcional (só `10.66.66.0/24` vs. full-tunnel `0.0.0.0/0`)
- [ ] Página de diagnóstico no cliente (logs, teste de conectividade, exportar relatório)

## Fase 7 — Empacotamento e distribuição

- [ ] Instalador Windows via NSIS (`.exe`)
- [ ] Empacotamento `.deb` para Linux
- [ ] Empacotamento AppImage para Linux
- [ ] Versionamento semântico + changelog automatizado no build
- [ ] Testar instalação limpa em VM nova (Windows)
- [ ] Testar instalação limpa em VM nova (Linux)
- [ ] (Futuro/opcional) Avaliar certificado de assinatura de código para reduzir alertas do SmartScreen

## Fase 8 — Observabilidade e documentação final

- [ ] Logs estruturados no servidor e no cliente
- [ ] Métricas básicas (nº de peers conectados, throughput agregado, uptime)
- [ ] Rodar a skill `vps-security-audit` novamente e revisar todos os achados
- [ ] Atualizar `README.md` com instruções finais de build/uso
- [ ] Revisão final do `PLAN.md` (marcar decisões que mudaram durante a implementação)

---

## Como usar este arquivo

- Ao concluir uma tarefa, marque o checkbox correspondente nesta mesma sessão de trabalho (não deixe para depois).
- Se uma decisão do `PLAN.md` mudar durante a implementação, atualize o `PLAN.md` **e** ajuste os itens correspondentes aqui.
- Itens novos descobertos durante o trabalho (ex.: um passo de hardening adicional) devem ser adicionados na fase correta, não só resolvidos "silenciosamente".
