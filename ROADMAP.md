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
- [x] Skills de Git/GitHub criadas (`start-task`, `ship-pr`, `release-status` — ver [`PLAN.md` §13](./PLAN.md#13-versionamento-e-releases))
- [x] Estratégia de versionamento independente por componente documentada (`PLAN.md` §13, `CONTRIBUTING.md`)
- [ ] Definir e adicionar `LICENSE` (pendente — ver README.md)

---

## Fase 0 — Hardening e provisionamento base do VPS

- [x] Verificar efetivo do SSH: `sshd -T | grep -i passwordauthentication`
- [x] Criar `/etc/ssh/sshd_config.d/00-xvpn-hardening.conf` (`PasswordAuthentication no`, `PermitRootLogin prohibit-password`, `KbdInteractiveAuthentication no`) — **nota**: usamos `00-` em vez do `99-` originalmente planejado; ver gotcha de ordenação documentado em [`PLAN.md` §9](./PLAN.md#9-correção-de-segurança-imediata-recomendada-independente-do-resto-do-projeto)
- [x] Recarregar sshd e confirmar (`systemctl reload ssh` + segunda sessão SSH independente validada antes de prosseguir)
- [x] Criar usuário de sistema `xvpn` (sem shell de login interativo, home dedicada em `/opt/xvpn`)
- [x] Instalar pacotes base: `nginx`, `certbot`, `python3-certbot-nginx`, `samba`, `fail2ban`, `unattended-upgrades` (`nginx`/`certbot`/`unattended-upgrades` já vinham instalados na imagem; `samba` e `fail2ban` instalados nesta fase)
- [x] Configurar `ufw`: política padrão `deny incoming` / `allow outgoing`
- [x] `ufw allow 22/tcp`, `ufw allow 80/tcp`, `ufw allow 443/tcp`, `ufw allow 51820/udp`
- [x] Ativar `ufw` (`ufw enable`) e confirmar com `ufw status verbose`
- [x] Configurar `fail2ban` para o serviço SSH (jail `sshd` ativa, já baniu tentativas reais de força bruta ao ser habilitada)
- [x] Habilitar e configurar `unattended-upgrades` (já vinha habilitado por padrão na imagem cloud do Ubuntu 26.04 — validado, nenhuma mudança necessária)
- [x] Criar server block Nginx para `vpn.officeempresa.com` (proxy para `127.0.0.1:8080`, retorna 502 temporariamente — validado, sem backend ainda até a Fase 2)
- [x] Emitir certificado: `certbot --nginx -d vpn.officeempresa.com`
- [x] Confirmar renovação automática do certificado (`systemctl list-timers | grep certbot` → `certbot.timer` ativo)
- [x] Coordenar com o setup do `landpages-ops` para não haver conflito de *server block* em `ldpops.appapisip.com` (validado: `ldpops.appapisip.com` continua respondendo normalmente após as mudanças)
- [x] Registrar em [`PLAN.md` §5](./PLAN.md#5-alocação-de-rede-portas-e-domínios-registro-para-não-colidir-com-landpages-ops) qualquer porta/domínio novo definido nesta fase (nenhuma porta/domínio novo além do já registrado)

**Achado de segurança durante a instalação**: o pacote `samba` sobe `smbd`/`nmbd` com o `smb.conf` padrão, que escuta em `0.0.0.0:139`/`0.0.0.0:445` (todas as interfaces) imediatamente após a instalação — violaria o invariante do `AGENTS.md` de que Samba nunca pode ser exposto publicamente. Como a interface `wg0` só é criada na Fase 1 e o `smb.conf` só é restrito a `wg0`/`lo` na Fase 5, paramos e desabilitamos `smbd`/`nmbd` (`systemctl stop/disable`) logo após a instalação, para não deixar a porta exposta entre esta fase e a Fase 5. Serão reabilitados só quando o `smb.conf` estiver com `bind interfaces only = yes` e `interfaces = wg0 lo`.

## Fase 1 — Validação manual do túnel WireGuard

- [x] Confirmar módulo carregado: `modprobe wireguard` + `lsmod | grep wireguard`
- [x] Criar interface: `ip link add dev wg0 type wireguard`
- [x] Gerar par de chaves do servidor (`wg genkey | tee server.key | wg pubkey > server.pub`), salvar chave privada em `/etc/wireguard/server.key` com permissão `600` — **nota**: o pacote `wireguard-tools` (comando `wg`) não vinha instalado por padrão, precisou ser instalado antes de gerar as chaves
- [x] Atribuir IP à interface: `ip addr add 10.66.66.1/24 dev wg0`
- [x] Configurar a interface com a chave privada e porta de escuta (`wg set wg0 listen-port 51820 private-key /etc/wireguard/server.key`)
- [x] Subir a interface: `ip link set wg0 up`
- [x] Habilitar `net.ipv4.ip_forward=1` (persistente via `/etc/sysctl.d/99-xvpn.conf`)
- [x] Configurar regra de NAT/MASQUERADE (`nftables`) para `10.66.66.0/24` saindo por `eth0` — implementada via `*nat`/`POSTROUTING`/`MASQUERADE` em `/etc/ufw/before.rules` (mecanismo nativo do `ufw` para isso), **mais** uma regra explícita `ufw route allow in on wg0 out on eth0`, já que a chain `FORWARD` do `ufw` tem política padrão `deny` — sem essa regra extra o NAT nunca seria alcançado (achado não previsto no checklist original)
- [x] Gerar par de chaves de um peer de teste (no seu notebook/desktop) — gerado num **container Docker isolado** na sua máquina (`--cap-add=NET_ADMIN --device=/dev/net/tun`), em vez de instalar `wireguard-tools` diretamente no SO principal: evita mexer na rede/rotas da máquina de uso diário e não exige sudo interativo. A chave privada nunca saiu do container/máquina local, nunca tocou o servidor.
- [x] Adicionar peer de teste no servidor: `wg set wg0 peer <pubkey_cliente> allowed-ips 10.66.66.2/32`
- [x] Configurar peer de teste localmente (`Endpoint = 206.189.224.72:51820`, `AllowedIPs = 0.0.0.0/0`, `PersistentKeepalive = 25`) — `::/0` omitido porque o container Docker de teste não tinha rota IPv6 configurada (irrelevante para o resultado do teste)
- [x] Subir túnel no peer de teste e validar handshake (`wg show` nos dois lados) — handshake confirmado nos dois lados
- [x] Validar exit: `curl ifconfig.me` de dentro do túnel deve retornar `206.189.224.72` — confirmado, após o ajuste de MTU abaixo
- [x] Validar latência/throughput básico (`ping`, opcionalmente `iperf3`) — `ping` ~135ms (esperado, tráfego passa por um túnel adicional pré-existente na máquina de teste — ver nota de MTU); download de 10 MB via `curl` em 1.77s (~45 Mbit/s)
- [x] Validar que o peer de teste consegue alcançar `10.66.66.1` (o próprio servidor) — confirma que "estar na mesma rede" funciona, não só o exit — confirmado via `ping 10.66.66.1`, 0% de perda
- [x] Documentar quaisquer ajustes de MTU/roteamento encontrados — **achado**: a máquina de teste já tinha uma VPN própria ativa (Cloudflare WARP, MTU 1280) como rota padrão. Com o MTU padrão do WireGuard (1420) para o túnel de teste, pacotes pequenos (ICMP) passavam mas requisições HTTP/TLS "sumiam" (black hole de PMTU, comum quando ICMP "packet too big" é descartado por túneis intermediários). Corrigido definindo `MTU = 1200` no `[Interface]` do peer de teste. **Relevante para o cliente real** (Fase 4/6): o cliente desktop deve permitir configurar/ajustar o MTU manualmente para usuários atrás de outra VPN, CGNAT restritivo ou redes móveis — considerar adicionar isso à tela de Configurações/Diagnóstico.
- [x] Peer de teste e container Docker removidos ao final (nenhum resíduo no servidor ou na máquina local); auditoria (`vps-security-audit`) confirmando `wg0` limpo, sem peers residuais

## Fase 2 — Control-plane API (Go)

- [x] Criar `server/` e inicializar módulo Go (`go mod init`)
- [x] Modelagem de dados via GORM: `User`, `Device`/`Peer`, `InviteToken`, `AuditLog`
- [x] Camada de autenticação: hash de senha com Argon2id, emissão/validação de JWT
- [x] Pacote `internal/wireguard/`: wrapper sobre `wgctrl-go` (`EnsureInterface`, `AddPeer`, `RemovePeer`, `ListPeers` com estatísticas rx/tx/handshake, `ReconcilePeers`) — **nota**: além do `wgctrl` (que só configura chave/porta/peers), a *criação* da interface (`ip link add ... type wireguard`, atribuição de IP, "link up") usa a lib `github.com/vishvananda/netlink` diretamente (nunca `exec.Command("ip", ...)`), mantendo o espírito de "nunca faça shell-out" do `go-backend.mdc` mesmo para essa parte que o `wgctrl` não cobre
- [x] Endpoint `POST /api/auth/login`
- [x] Endpoints CRUD `GET/POST/DELETE /api/users`
- [x] Endpoint `POST /api/users/:id/invite` (gera token de convite, expira em 15 min)
- [x] Endpoint `POST /api/devices/enroll` (recebe chave pública + token, aloca IP livre em `10.66.66.0/24`, registra peer via `wgctrl`)
- [x] Endpoint `GET /api/devices` (lista peers + estatísticas ao vivo)
- [x] Endpoint `DELETE /api/devices/:id` (revoga peer imediatamente)
- [x] Endpoint `GET /api/status` (saúde do servidor, nº de peers conectados, `api_version`)
- [x] Testes unitários dos handlers principais e da camada `wireguard/` — a camada `api/` é testada com um `wireguard.PeerManager` fake (interface extraída especificamente para isso) + SQLite em memória, sem precisar de `CAP_NET_ADMIN`/kernel real; a camada `wireguard/` em si só tem testes unitários da lógica pura de alocação de IP (`AllocateIP`), já que manipular a interface de fato exige privilégio e um kernel com suporte a WireGuard — não reproduzível de forma confiável em CI/sandbox. A validação de ponta a ponta real (API → `wgctrl` → `wg show`) foi feita manualmente em produção (ver abaixo).
- [x] `systemd` unit `xvpn-server.service` com `AmbientCapabilities=CAP_NET_ADMIN` (rodando como usuário `xvpn`, não root) — inclui hardening adicional (`ProtectSystem=strict`, `ProtectHome`, `PrivateTmp`, `NoNewPrivileges`)
- [x] Apontar o server block Nginx de `vpn.officeempresa.com` para `127.0.0.1:8080` (backend real) — o server block já apontava para lá desde a Fase 0 (retornava 502); nenhuma mudança de config necessária, só o backend passou a existir
- [x] Configurar backup automático do `xvpn.db` (cron + `sqlite3 .backup`, rotação de 7 dias) — `sqlite3` (CLI) precisou ser instalado à parte no VPS (não vem por padrão); testado manualmente com sucesso (`sudo -u xvpn /opt/xvpn/bin/backup.sh`)
- [x] Adicionar componente `server` (e `shared`, se já criado) ao `release-please-config.json` + `.release-please-manifest.json` + criar workflow `.github/workflows/release-please.yml` (ver [`PLAN.md` §13.4](./PLAN.md#134-implantação-faseada-não-criar-workflow-ainda)) — `shared/` ainda não existe, só `server` foi adicionado por enquanto

**Achados durante o deploy em produção:**

1. **Permissão da chave privada**: `/etc/wireguard/server.key` era `600 root:root` (criada manualmente na Fase 1) — o processo `xvpn-server`, rodando como usuário `xvpn`, não conseguia lê-la. Corrigido com `chgrp xvpn` + `chmod 640` (nunca `chmod o+r` — mantém a chave ilegível para qualquer usuário fora do grupo `xvpn`/root).
2. **`XVPN_DB_PATH` relativo quebra com `ProtectSystem=strict`**: o valor padrão do caminho do banco (`xvpn.db`, relativo ao `WorkingDirectory=/opt/xvpn`) tentava gravar em um diretório que o hardening do systemd (`ProtectSystem=strict`, só libera escrita em `ReadWritePaths=/opt/xvpn/data`) torna somente leitura. Corrigido definindo `XVPN_DB_PATH=/opt/xvpn/data/xvpn.db` explicitamente no `.env` de produção; o `.env.example` no repo já reflete isso como obrigatório.
3. **`sqlite3` (CLI) não vem instalado por padrão** no VPS — só a biblioteca é usada pelo Go via `mattn/go-sqlite3` (cgo), o binário `sqlite3` usado pelo script de backup precisou ser instalado à parte (`apt-get install sqlite3`).

**Validação de ponta a ponta em produção** (critério de "pronto" da Fase 2, `PLAN.md` §12): criado um convite via API, gerado um par de chaves de teste, feito o enrollment via `https://vpn.officeempresa.com/api/devices/enroll` — o IP `10.66.66.2/32` foi alocado corretamente e o peer apareceu **imediatamente** em `wg show wg0` no servidor. Revogado o mesmo device via `DELETE /api/devices/:id` — o peer sumiu do `wg show wg0` na mesma hora. Testado também um `systemctl restart xvpn-server`: a interface e a chave pública do servidor permaneceram intactas, e a reconciliação de peers a partir do banco rodou sem erro. Auditoria de segurança pós-deploy confirma `xvpn-server` escutando **só** em `127.0.0.1:8080` (nunca exposto direto).

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
- [ ] Adicionar componente `client` ao `release-please-config.json` + `.release-please-manifest.json` (ver [`PLAN.md` §13.4](./PLAN.md#134-implantação-faseada-não-criar-workflow-ainda))

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
