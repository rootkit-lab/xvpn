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

- [x] Scaffold Vite + React + TypeScript em `server/web/`
- [x] Configurar TailwindCSS + shadcn/ui (Tailwind v4, `components.json` com alias `@/`, estilo `new-york`)
- [x] Tela de Login
- [x] Dashboard (peers ativos, throughput agregado, status geral)
- [x] Tela Usuários (CRUD + gerar convite / QR code)
- [x] Tela Dispositivos (status, último handshake, revogar)
- [x] Tela Compartilhamentos (placeholder explícito — implementação real chega na Fase 5)
- [x] Tela Configurações (rede, DNS, firewall) — somente leitura por ora; edição via painel fica para uma fase futura (exigiria desenho de validação/segurança próprio)
- [x] Tela Auditoria (log de ações administrativas)
- [x] Build do painel embutido no binário Go via `embed.FS`
- [x] Teste end-to-end manual: criar usuário → gerar convite → dispositivo aparece conectado no painel

**Notas de implementação:**

- `server/web/` usa Vite + React 19 + TypeScript + Tailwind v4 (`@tailwindcss/vite`, CSS-first, sem `tailwind.config.js`) + shadcn/ui (`components.json`, estilo `new-york`, ícones `lucide-react`).
- **Gotcha do shadcn CLI**: `npx shadcn@latest add ...` criou os componentes numa pasta literal `@/components/ui/` (não resolveu o alias do `tsconfig`) — precisou mover manualmente para `src/components/ui/`. Rodar `ls` para confirmar o destino real sempre que usar o CLI novamente.
- **Gotcha do `go:embed`**: não aceita `..` no caminho, então o Vite builda direto dentro da árvore do pacote Go (`outDir: server/internal/webui/dist`, não `server/web/dist`) — ver `server/web/vite.config.ts` e `server/internal/webui/webui.go`. O diretório de saída é ignorado no Git exceto um `placeholder.txt` committado, só para o `go:embed`/`go build` nunca falharem num checkout limpo antes do `npm run build` ter rodado.
- Dois endpoints novos no backend, necessários para as telas de Configurações e Auditoria (não estavam no escopo original da Fase 2): `GET /api/config` (somente leitura, nunca expõe `JWTSecret` nem a chave privada WireGuard) e `GET /api/audit` (últimas 200 entradas). Cobertos por testes em `internal/api/config_handler_test.go` e `audit_handler_test.go`.
- Cliente HTTP único em `src/lib/api.ts` (token em `localStorage`, 401 limpa sessão e redireciona a `/login`), contexto de auth em `src/lib/auth-context.tsx`, polling simples via `usePollingData` (dashboard/dispositivos a cada 10s, usuários/auditoria mais devagar).
- QR code do convite (`qrcode.react`) codifica `{"invite_token": "..."}` — pensado para o cliente desktop (Fase 4) escanear no fluxo de enrollment.
- Validado localmente: `go build`/`go test` (backend) e `npm run build`/`npm run lint` (frontend) passam limpos; verificação manual via `httptest` confirmou que `/` e rotas SPA (ex.: `/users`) servem `index.html` e que `/api/rota-inexistente` continua devolvendo 404 JSON (teste descartado depois, não é parte da suíte permanente — depende do estado do build do painel).

## Fase 4 — Cliente Desktop MVP (Wails3)

- [x] Criar `client/` e inicializar com `wails3 init` (Go + React)
- [x] Configurar TailwindCSS + shadcn/ui no frontend do cliente
- [x] Implementar helper privilegiado (`internal/tunnel/`: `wireguard-go` + `wgctrl-go`)
- [x] Implementar TUN no Linux (dispositivo `wg` nativo do kernel)
- [x] Implementar TUN no Windows (`wintun` embutido via `go:embed`) — implementado e compila via cross-compile no Linux; **não testado em Windows real** (ver nota abaixo)
- [x] Implementar IPC GUI ↔ Helper (JSON-RPC via Unix Socket no Linux / Named Pipe no Windows)
- [x] Tela de enrollment (inserir código de convite gerado no painel)
- [x] Tela principal: Conectar/Desconectar, status (IP, latência, throughput, tempo conectado)
- [x] Ícone de bandeja (tray) básico
- [x] Instalação do serviço/helper (systemd unit no Linux / Windows Service no instalador) — unit systemd criada (`client/deploy/systemd/`); instalador do Windows Service fica para a Fase 7 (empacotamento)
- [x] Testar enrollment e conexão ponta a ponta no Linux
- [ ] Testar enrollment e conexão ponta a ponta no Windows — **pendente de validação manual pelo usuário** (ambiente de desenvolvimento é Linux, ver decisão de escopo no início da Fase 4)
- [x] Adicionar componente `client` ao `release-please-config.json` + `.release-please-manifest.json` (ver [`PLAN.md` §13.4](./PLAN.md#134-implantação-faseada-não-criar-workflow-ainda))

**Notas de implementação:**

- Arquitetura em duas partes no mesmo binário: GUI (Wails, sem privilégio) fala por IPC com um **helper privilegiado** (`xvpn-client --helper`, roda como serviço systemd só com `CAP_NET_ADMIN` — não root, ver `client/deploy/systemd/xvpn-client-helper.service`) que é o único a tocar na rede — ver `client/README.md`. Evita rodar WebView/GTK com qualquer privilégio elevado.
- Engine de túnel por plataforma (`internal/platform/`): Linux usa o WireGuard nativo do **kernel** via `netlink`+`wgctrl` (mesma dupla do `server/`); Windows usa `wireguard-go` (userspace) + driver `wintun`, já que o Windows não tem WireGuard no kernel.
- `wintun.dll` não é commitado (binário de terceiros) — `internal/platform/windows/wintun/` guarda só um placeholder para o `go:embed` nunca falhar num checkout limpo; `build/windows/fetch-wintun.ps1` baixa o `.dll` real (com verificação de SHA256) antes de um build Windows de verdade.
- IPC: socket Unix `0660` num grupo `xvpn` dedicado no Linux (mesmo padrão do socket do Docker); named pipe com SDDL restrito a "Authenticated Users" no Windows via `go-winio`.
- **Achado de MTU (reafirma o da Fase 1)**: o mesmo "PMTU black hole" apareceu no teste E2E do cliente (handshake OK, mas `curl` via TLS travava). Corrigido adicionando um campo `MTU` opcional em todo o caminho — `EnrollRequest` → `config.DeviceState` → `tunnel.Config` — exposto como opção avançada na tela de enrollment da GUI.
- **Achados de roteamento no teste E2E (Docker, Linux)**, todos em `internal/platform/linux/engine_linux.go`:
  1. Detecção da rota padrão via `netlink.RouteList` precisa tratar tanto `Dst == nil` quanto um `IPNet` explícito `0.0.0.0/0`, dependendo da versão do kernel/netlink.
  2. Em túnel completo (`AllowedIPs` contendo `0.0.0.0/0`), a rota adicionada via `xvpn0` **substitui** (não empilha com) a entrada da rota padrão original na tabela principal (`netlink.RouteReplace` é um `NLM_F_REPLACE`) — sem salvar e reaplicar essa rota original em `Disconnect`, a máquina ficava **sem rota padrão nenhuma** depois de desconectar. Corrigido capturando a rota original em `Connect` e restaurando-a explicitamente no `teardown`.
  3. Qualquer erro no meio de `Connect` (ex.: falha ao configurar uma rota) só desfazia a rota de exceção do IP do servidor, mas **não** removia a interface `xvpn0` já criada/configurada/up — deixando-a "half-configured" segurando a rota padrão mesmo com `Connect()` tendo retornado erro. Corrigido com rollback único via `defer`/flag `success`, que desfaz tudo (interface, rota de exceção, rota padrão original) em qualquer caminho de erro.
  4. O servidor sempre inclui `::/0` nas `AllowedIPs` do peer como blackhole anti-vazamento de IPv6 (o túnel só tem endereço IPv4). Em ambientes sem stack IPv6 na interface recém-criada (containers Docker restritos, kernels com `ipv6.disable=1`) adicionar essa rota falha com "no such device" — tratado como best-effort (não derruba o túnel IPv4): sem IPv6 utilizável já não há vazamento possível de qualquer forma.
- `client/frontend/bindings/` (TypeScript gerado pelo `wails3 generate bindings` a partir dos métodos Go expostos) é artefato de build, não é commitado — adicionado ao `.gitignore` e à tabela do `PLAN.md` §11.1 (o `client/.gitignore` gerado pelo `wails3 init` não incluía essa pasta).
- `cmd/devtool-helper` e `cmd/devtool-e2e`: o binário Wails principal linka `libX11`/GTK/WebKit2GTK mesmo em modo `--helper`, o que quebra em containers headless. Esses dois comandos mínimos (helper sem GUI + cliente IPC de CLI) isolam a lógica de rede para testar em Docker/CI sem esse problema.
- **Validação de ponta a ponta em Linux**: usando um container Docker (`ubuntu:24.04`, `--cap-add=NET_ADMIN --device /dev/net/tun`) rodando `devtool-helper`, feito enrollment real contra `https://vpn.officeempresa.com` (convite gerado via API), `connect` estabeleceu handshake com o servidor de produção, `curl https://ifconfig.me` de dentro do container retornou o IP público do VPS (`206.189.224.72`), e `disconnect` restaurou a rota padrão original do container. Ciclo connect→disconnect→connect repetido sem vazamento de estado.
- **Achado na instalação real da unit systemd (fora do Docker)**: `xvpn-client-helper.service` falhava no start com `226/NAMESPACE` — causa era `ReadWritePaths=/etc/xvpn-client` apontando para um diretório que ainda não existia na primeira instalação (`ProtectSystem=strict` + `ReadWritePaths` de um caminho inexistente falha a montagem do namespace nessa versão do systemd). Corrigido trocando por `StateDirectory=xvpn-client` (systemd cria `/var/lib/xvpn-client` automaticamente, já com o dono/permissão certos, antes do processo subir — mesmo mecanismo do `RuntimeDirectory` já usado para `/run/xvpn-client`). Também corrigiu o local convencionalmente certo para estado de serviço no Linux (`/var/lib`, não `/etc`, que é para config fornecida pelo administrador) — `internal/config/path_linux.go` atualizado de `/etc/xvpn-client/device.json` para `/var/lib/xvpn-client/device.json`.
- **Decisão de escopo Windows** (combinada no início da Fase 4): o ambiente de desenvolvimento é Linux, então o código Windows (`platform/windows`, `wintun`, named pipes) foi escrito multiplataforma e validado até onde o Linux permite (compila via cross-compile, `go vet`/`gofmt` limpos), mas o teste manual real num Windows fica para quando o usuário validar esta fase.

## Fase 5 — Compartilhamento de arquivos

- [x] Instalar e configurar Samba (`bind interfaces only = yes`, `interfaces` restrito à VPN — ver achado sobre nome vs. IP/CIDR abaixo)
- [x] Criar share inicial `[shared]` (`/srv/xvpn/shared`, grupo dedicado `xvpn-samba`) — `[home-<usuario>]` por pessoa não foi criado nesta fase, ver decisão de escopo abaixo
- [x] (Opcional) Sincronizar criação de usuário Samba com criação de usuário XVPN via painel — **decidido não fazer**, ver justificativa abaixo e na skill `samba-user-ops`
- [x] Instalar FileBrowser, `systemd` unit `xvpn-filebrowser.service`, bind exclusivo em `10.66.66.1:8081` — **trocado para o fork ativo FileBrowser Quantum**, ver achado abaixo
- [x] Botão no cliente desktop: "Abrir arquivos do servidor" (unidade de rede e/ou FileBrowser)
- [x] Validar externamente (fora da VPN) que Samba e FileBrowser são **inacessíveis** via `eth0`/IP público

**Notas de implementação:**

- **Achado crítico — projeto FileBrowser original arquivado**: ao instalar `filebrowser/filebrowser` (release `v2.63.23`), o próprio binário avisou no log que o repositório **será arquivado em 2026-09-01**, sem mais releases/correções de segurança — a última versão planejada já foi publicada. Trocado pelo fork ativo **FileBrowser Quantum** (`gtsteffaniak/filebrowser`, release `v1.5.1-stable`), que mantém a mesma arquitetura (binário Go único, `systemd` próprio, bind exclusivo em `10.66.66.1:8081`) com manutenção de segurança contínua.
- **Bug encontrado no FileBrowser Quantum `v1.5.1-stable`**: a variável de ambiente `FILEBROWSER_ADMIN_PASSWORD` reescreve a senha do admin **em texto puro** no banco a cada boot (o caminho de código usado — `validateUserInfo` → `Update` — não re-hasheia, diferente do caminho de criação inicial via `Save`), quebrando o login permanentemente assim que usada. Confirmado lendo o código-fonte da tag da release. Solução: **nunca** usar essa env var; definir/redefinir a senha do admin manualmente, com o serviço parado, via `filebrowser set -u admin,<senha> -a -c /etc/xvpn-filebrowser/config.yaml` (esse caminho de código hasheia corretamente) — documentado em `server/deploy/filebrowser/config.yaml` e no comentário da `xvpn-filebrowser.service`.
- **Achado — `cacheDir` do FileBrowser**: por padrão o cache (thumbnails etc.) vai para `./tmp`, relativo ao `WorkingDirectory` — que é o próprio `/srv/xvpn/shared`, poluindo o compartilhamento (a pasta `tmp` aparecia também via Samba). Corrigido apontando `server.cacheDir` para `/var/lib/xvpn-filebrowser/cache` (dentro do `StateDirectory` do systemd, fora do share).
- **Achado — Samba não faz bind correto por nome de interface WireGuard**: com `interfaces = wg0 lo`, o `smbd` subia normalmente mas só ficava escutando em `127.0.0.1:445` — nunca em `10.66.66.1`, mesmo com `bind interfaces only = yes`. Causa: a detecção automática de interface do Samba assume broadcast/netmask convencionais, e `wg0` é ponto-a-ponto (sem broadcast). Corrigido especificando IP/CIDR explícito: `interfaces = 10.66.66.1/24 127.0.0.1/8`. Só foi detectado testando via túnel real de outra máquina — o teste local (loopback) no próprio servidor não pega esse problema, por isso a Fase 5 exige validação a partir de um peer WireGuard de verdade, não só `localhost`.
- **Decisão de escopo — sem sincronização automática de usuário Samba com o painel**: o processo `xvpn-server` roda com privilégio mínimo (só `CAP_NET_ADMIN`, ver `PLAN.md` §6.1); dar a ele permissão para criar usuários de sistema/Samba aumentaria bastante a superfície de risco de qualquer bug no painel. Criação/remoção de usuário Samba ficou manual, via a nova skill `.cursor/skills/samba-user-ops/` (mesmo padrão da `wireguard-peer-ops` da Fase 1). Pode ser revisitado numa fase futura com um endpoint de admin dedicado e seu próprio hardening.
- **Decisão de escopo — só o share `[shared]`, sem `[home-<usuario>]` por pessoa**: como não há sincronização automática de usuário (ver acima), criar uma pasta pessoal por usuário exigiria o mesmo processo manual da skill `samba-user-ops` mais uma etapa extra de `mkdir`/`chown` por pessoa. Para o uso atual (rede privada pessoal/familiar), um único share compartilhado atende; pastas individuais ficam para quando houver de fato múltiplos usuários com necessidade de área privada.
- Validação via túnel real (não só loopback): a partir de uma segunda máquina já conectada à VPN (`10.66.66.2`), testado `smbclient`/`smbprotocol` (listar, subir e ler arquivo de volta) contra `\\10.66.66.1\shared` e `curl` contra `http://10.66.66.1:8081` — ambos funcionando. Fora do túnel (rota real via `eth0`/internet até o IP público `206.189.224.72`), as portas `445` e `8081` retornam recusado/timeout, tanto testado a partir de outra máquina quanto do próprio servidor contra seu IP público.
- Firewall: liberadas `445/tcp` e `8081/tcp` no `ufw`, mas escopadas só à interface `wg0` (`ufw allow in on wg0 to any port ...`) — o bind exclusivo em `10.66.66.1` já impede acesso via `eth0` por si só (defesa em profundidade, ver `AGENTS.md` invariante #2), o firewall é uma segunda camada, não a única.
- Botão no cliente: `client/internal/opener/` (novo pacote, arquivos `_linux.go`/`_windows.go` com `//go:build`) abre a unidade de rede (`smb://.../shared` via `xdg-open` no Linux, caminho UNC via `cmd /c start` no Windows) ou o FileBrowser no navegador padrão. Roda inteiramente no processo GUI sem privilégio — não precisa passar pelo helper, já que só abre um app externo.

## Fase 6 — Recursos avançados do cliente

- [x] Kill switch (`nftables` no Linux / Windows Filtering Platform no Windows) — Windows **não testado em hardware real**, ver decisão de escopo abaixo (mesmo padrão da Fase 4)
- [x] Reconexão automática com backoff exponencial
- [x] Ícone de bandeja completo (status visual, atalhos rápidos)
- [x] Auto-start no boot do sistema operacional (opcional, configurável)
- [x] Split-tunnel opcional (só `10.66.66.0/24` vs. full-tunnel `0.0.0.0/0`)
- [x] Página de diagnóstico no cliente (logs, teste de conectividade, exportar relatório)

**Notas de implementação:**

- **Preferências** (`kill_switch`, `split_tunnel`, `auto_reconnect`) persistidas em `config.DeviceState.Preferences` (mesmo arquivo de estado do enrollment, só o helper lê/escreve) e ajustáveis a qualquer momento pela GUI via `get_preferences`/`set_preferences` (IPC) — se o túnel já estiver conectado, `handleSetPreferences` reaplica a config imediatamente (reconecta com kill switch/split-tunnel novos sem exigir um disconnect manual). `auto_reconnect` vem `true` por padrão só para dispositivos enrollados a partir desta fase (dispositivos antigos mantêm o comportamento anterior até o usuário ligar manualmente).
- **Kill switch fail-closed**: Linux via `nftables` (tabela dedicada `xvpn_killswitch`, chain `output` com `policy drop` e exceções para loopback, a própria interface do túnel e o IP público do servidor — essa última exceção é o que permite o próprio WireGuard reconectar depois de uma queda); Windows via Windows Filtering Platform (`github.com/tailscale/wf`, sessão WFP **dinâmica** — se o helper morrer, o kernel remove os filtros automaticamente, nunca trava a máquina sem internet). Ativação e desativação são sempre fail-closed: se `enableKillSwitch` falhar, `Connect()` inteiro é desfeito (rollback); numa reconexão automática (ou uma tentativa de reconexão que falha), o kill switch **nunca** é desligado no meio do caminho — só um `Disconnect()` explícito do usuário (ou desligar a preferência) o remove de fato.
- **Reconexão automática** (`internal/helper/reconnect.go`): monitor com `time.Ticker` de 5s detecta queda comparando `engine.Status()`; ao detectar, tenta reconectar com backoff exponencial (1s, 2s, 4s, ... até um teto de 60s). Cancelado de forma limpa via `context.Context` em qualquer `Disconnect()` explícito (sem race: o `select` do backoff também escuta `ctx.Done()`).
- **Split-tunnel**: quando ativo, as `AllowedIPs` configuradas no peer local passam a ser só `10.66.66.0/24` (em vez do que o servidor concedeu, tipicamente `0.0.0.0/0`) — o resto do tráfego do dispositivo continua saindo direto pela rede local, sem tocar o túnel.
- **Ícone de bandeja dinâmico** (`internal/trayicons/`, ícones PNG gerados via Pillow e embutidos com `go:embed`): tooltip e ícone (verde/cinza/âmbar/vermelho) atualizados por um goroutine (`monitorTray` em `main.go`) que faz *polling* do `Status()` a cada poucos segundos, refletindo conectado/desconectado/reconectando/helper indisponível.
- **Auto-start** (`internal/autostart/`): entrada `.desktop` em `~/.config/autostart/` no Linux (`X-GNOME-Autostart-enabled=true`, reconhecido por GNOME/KDE/XFCE/COSMIC), chave `HKCU\...\Run` no Windows — sempre no espaço do usuário sem privilégio (nunca toca o helper), consistente com a separação de privilégio do `.cursor/rules/go-client.mdc`.
- **Página de diagnóstico**: `RunDiagnostics()` junta o status do helper com dois testes de conectividade ativos — painel web pela internet normal (`ServerBaseURL + /api/status`) e o próprio servidor dentro da VPN (`10.66.66.1:8081`, só com túnel ativo) — pensado para diferenciar "sem internet" de "internet ok, mas VPN não conecta". Nunca inclui a chave privada. Logs recentes do helper expostos via um ring buffer em memória (`internal/helper/logbuffer.go`, últimas ~200 linhas) — sem depender de `journalctl`/Visualizador de Eventos, que a GUI sem privilégio não acessaria de qualquer forma. Relatório exportável como JSON pela UI.
- **Validação de ponta a ponta em Linux (Docker + VPS real de produção)**: criado um peer de teste temporário (`10.66.66.50/32`, chave gerada localmente via `wgtypes.GeneratePrivateKey` — nunca no servidor) registrado manualmente via `wg set` (mesmo fluxo da skill `wireguard-peer-ops`), removido ao final. Testado num container Docker `--privileged` (ver achado de capabilities abaixo) rodando `devtool-helper` (mesmo binário sem GUI usado na Fase 4) com um `device.json` fabricado apontando para esse peer:
  - **Kill switch**: com o túnel conectado e kill switch ligado, tráfego que tenta contornar o túnel saindo explicitamente por `eth0` (`ping -I eth0`) é bloqueado (timeout); tráfego legítimo pelo túnel (inclusive para o próprio servidor, `10.66.66.1`, e para outro peer real da VPN) continua funcionando.
  - **Queda inesperada + kill switch**: simulada removendo a interface do túnel por fora (`ip link delete xvpn0`) sem passar por `Disconnect()` — o kill switch permanece bloqueando tráfego durante toda a janela até a reconexão automática (~1s de backoff) restabelecer o túnel com handshake novo.
  - **Reconexão automática**: detectada e executada dentro do intervalo esperado (monitor de 5s + backoff inicial de 1s), confirmada via `wg`/logs do helper (`"túnel caiu, tentando reconectar..."` → `"túnel reconectado automaticamente"`).
  - **Split-tunnel**: validado em container limpo — só a rota `10.66.66.0/24` passa a apontar para a interface do túnel, a rota padrão original (via `eth0`) permanece intocada; tráfego para fora da sub-rede da VPN sai direto, tráfego para dentro dela passa pelo túnel. `Disconnect()` a partir desse modo restaura a tabela de rotas exatamente ao estado anterior.
  - **Desconexão explícita**: kill switch desativado de fato (tabela `nftables` removida), interface do túnel removida, rota padrão original restaurada.
- **Bug encontrado e corrigido durante a validação**: `Status()` (Linux e Windows) reportava `KillSwitchActive=false` sempre que o túnel era detectado como caído (`!e.connected`, ou a interface do WireGuard não existia mais no kernel) — mesmo quando a tabela `nftables`/sessão WFP continuava genuinamente ativa e bloqueando tráfego nesse meio-tempo (proteção real intacta; era só o *reporte* de status que mentia). Corrigido em `engine_linux.go`/`engine_windows.go` para sempre refletir `e.killSwitchActive`/`e.killSwitch != nil` mesmo no caminho "desconectado" — importante para a página de diagnóstico e o ícone de bandeja não mostrarem um estado inconsistente logo no momento em que o kill switch mais importa.
- **Achado de capabilities do Docker**: as mesmas capabilities usadas na validação da Fase 4 (`--cap-add=NET_ADMIN --cap-add=NET_RAW`) foram suficientes para WireGuard/rotas, mas **não** para o kernel de fato aplicar os hooks do `nftables` (`policy drop` no `output` não bloqueava nada, apesar do `nft list ruleset` mostrar a tabela corretamente criada) — precisou `--privileged` para o teste do kill switch. Isso é uma particularidade do ambiente de teste em container (namespace de rede sem todas as capabilities do netfilter); não afeta o helper rodando direto no host (via `systemd`, com `CAP_NET_ADMIN` de verdade), só o arcabouço de teste em Docker.
- **Decisão de escopo — kill switch Windows não testado em hardware real**: implementado usando a mesma biblioteca WFP (`github.com/tailscale/wf`) usada em produção pelo cliente Windows da Tailscale, seguindo de perto o desenho de referência do `wireguard-windows` oficial (sessão dinâmica — falha do processo nunca deixa a máquina bloqueada permanentemente). Validado só via cross-compilation (`GOOS=windows`, `go vet`/`gofmt` limpos) — mesma decisão de escopo combinada no início da Fase 4, o teste manual em Windows real fica para quando o usuário validar esta fase.
- Utilitário `cmd/devtool-e2e` (criado na Fase 4) ganhou os comandos `get-prefs`/`set-prefs`/`logs` para exercitar os novos métodos IPC sem precisar de GUI.

## Fase 7 — Empacotamento e distribuição

- [x] Instalador Windows via NSIS (`.exe`) — gerado via `task windows:create:nsis:installer`; registro do helper como Windows Service fica pendente de validação em hardware real (mesmo padrão das Fases 4/6)
- [x] Empacotamento `.deb` para Linux — `nfpm` + `postinstall` cria grupo `xvpn`, usuário `xvpn-client-helper`, instala/enable a unit systemd
- [x] Empacotamento AppImage para Linux — portátil (GUI); instalação completa do helper continua sendo via `.deb`
- [x] Versionamento semântico no build (`build/scripts/resolve-version.sh` → ldflags + `XVPN_VERSION` no nfpm); changelog do componente segue o `release-please`
- [x] Página `/download` no portal (após login) com links para GitHub Releases e instruções por plataforma
- [ ] Testar instalação limpa em VM nova (Windows) — **pendente de validação manual pelo usuário**
- [ ] Testar instalação limpa em VM nova (Linux) — pacotes gerados localmente; teste em VM limpa pendente
- [ ] (Futuro/opcional) Avaliar certificado de assinatura de código para reduzir alertas do SmartScreen

**Notas de implementação:**

- Metadados de branding atualizados (`build/config.yml`, `build/windows/info.json`, `nfpm.yaml`): produto XVPN, homepage `https://vpn.officeempresa.com`.
- `VPNService.Version()` e `DiagnosticsReport.ClientVersion` expõem a versão embutida no binário.
- Artefatos (`*.deb`, `*.AppImage`, `*-installer.exe`, `wintun.dll`) permanecem fora do Git (`.gitignore` / `PLAN.md` §11.1); distribuição via GitHub Releases.

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
