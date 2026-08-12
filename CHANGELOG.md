# Changelog

Todas as mudanças relevantes deste projeto são documentadas aqui.

Formato baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.0.0/), versionamento seguirá [SemVer](https://semver.org/lang/pt-BR/) a partir da primeira release.

## [Unreleased]

### Added

- `PLAN.md` com arquitetura completa do projeto, diagnóstico real do VPS e decisões técnicas justificadas.
- `ROADMAP.md` com checklist de execução por fases (0 a 8).
- `README.md`, `CONTRIBUTING.md`, `SECURITY.md`.
- `AGENTS.md` com contexto e invariantes de segurança para agentes de IA.
- Configuração inicial do Cursor: rules (`.cursor/rules/`), hooks (`.cursor/hooks.json`) e skills (`.cursor/skills/`) específicas do projeto (auditoria de segurança do VPS, operações de peer WireGuard, checagem de registro de portas/domínios).
- Confirmação via DNS de que `vpn.officeempresa.com` e `ldpops.appapisip.com` apontam para `206.189.224.72`.
- `.gitignore` completo (segredos/chaves, artefatos de build, banco de dados local, arquivos de SO/IDE).
- Tabela de convenção de build (`PLAN.md` §11.1): o que é gerado, onde fica, e se é commitado.
- Hook real de pre-commit (`.githooks/pre-commit`), independente do editor, bloqueando commit de segredos e de artefatos de build — complementar (não substitui) o hook `.cursor/hooks.json`, que só protege ações do agente de IA dentro do Cursor.
- Repositório Git inicializado, com `core.hooksPath` configurado para `.githooks`.
- Repositório remoto criado no GitHub (`rootkit-lab/xvpn`), configurado para squash merge exclusivo e delete automático de branch após merge.
- Fluxo de trabalho GitHub Flow obrigatório documentado em `CONTRIBUTING.md` (branch → PR → squash merge), aplicado tecnicamente em dois níveis: hook local (`.githooks/pre-commit` bloqueia commit direto em `main`) e *branch protection* real no GitHub (PR obrigatório, sem push direto, sem force-push, histórico linear).
- Skills do Cursor para o fluxo de Git/GitHub do dia a dia: `start-task` (cria branch a partir de `main` atualizada), `ship-pr` (push + checklist + abertura de PR com título validado em Conventional Commits) e `release-status` (consulta PRs de release pendentes do `release-please`).
- Seção "13. Versionamento e releases" no `PLAN.md`: versionamento semântico independente por componente (`server`, `client`, `shared`), automatizado via [release-please](https://github.com/googleapis/release-please) a partir dos Conventional Commits, e contrato de compatibilidade `api_version` entre cliente e servidor.
- Seção "Versionamento" em `CONTRIBUTING.md`, com novo item no checklist pré-PR exigindo que o título do PR siga Conventional Commits (por causa do squash merge, o título vira o commit final analisado pelo `release-please`).
- Diretriz em `AGENTS.md` para criação proativa de novas Skills sempre que um comando/sequência de passos se repetir 3+ vezes numa sessão.
- Itens de checklist no `ROADMAP.md` (Fases 2 e 4) para configurar `release-please-config.json`/`.release-please-manifest.json`/workflow quando `server/` e `client/` existirem.
- **Fase 0 do `ROADMAP.md` concluída no VPS de produção**: usuário de sistema `xvpn`, pacotes base (`samba`, `fail2ban`; `nginx`/`certbot`/`unattended-upgrades` já vinham na imagem), `ufw` ativo (padrão-nega, liberando só `22/tcp`, `80/tcp`, `443/tcp`, `51820/udp`), `fail2ban` protegendo o SSH, server block Nginx + certificado TLS (Let's Encrypt) para `vpn.officeempresa.com`, renovação automática confirmada (`certbot.timer`).
- **Fase 1 do `ROADMAP.md` concluída — validação manual do túnel WireGuard**: interface `wg0` criada e configurada no VPS (`10.66.66.1/24`, porta `51820`), `ip_forward` habilitado, NAT/MASQUERADE via `ufw`/`nftables` + regra `ufw route allow in on wg0 out on eth0`. Túnel de teste ponta a ponta validado a partir de um container Docker isolado na máquina local (chave privada de teste gerada e mantida só no container, nunca no servidor): handshake confirmado nos dois lados, `10.66.66.1` alcançável (mesma rede), exit IP confirmado como `206.189.224.72`, download de 10 MB em 1.77s através do túnel. Peer e container de teste removidos ao final; auditoria de segurança pós-fase sem resíduos.
- **Fase 2 do `ROADMAP.md` concluída — control-plane API em Go**: módulo `server/` criado (Gin + GORM/SQLite + `wgctrl` + `golang-jwt` + Argon2id), com modelos `User`/`Device`/`InviteToken`/`AuditLog`, todos os endpoints da API (`auth/login`, CRUD de usuários, convite, `devices/enroll`, `devices` list/delete, `status`), reconciliação de peers na inicialização (`ReconcilePeers`) e testes automatizados dos handlers (SQLite em memória + fake de `wireguard.PeerManager`, sem depender de `CAP_NET_ADMIN`). Implantado em produção: `xvpn-server.service` (systemd, `CAP_NET_ADMIN`, `ProtectSystem=strict`), backup diário do `xvpn.db` via cron, componente `server` adicionado ao `release-please`. Validado ponta a ponta via `https://vpn.officeempresa.com`: enrollment de um dispositivo de teste refletiu imediatamente em `wg show wg0` no servidor, e a revogação removeu o peer na mesma hora.
- **Fase 3 do `ROADMAP.md` concluída — painel web (React + Tailwind + shadcn/ui)**: `server/web/` (Vite + React 19 + TypeScript + Tailwind v4 + shadcn/ui, estilo `new-york`) com as sete telas previstas — Login, Dashboard, Usuários (CRUD + convite com QR code via `qrcode.react`), Dispositivos (status/handshake/revogar), Compartilhamentos (placeholder — Fase 5), Configurações (somente leitura) e Auditoria. Build embutido no binário Go via `embed.FS` (`server/internal/webui/`), servindo tanto os arquivos estáticos quanto o *fallback* de rota SPA para o `react-router`. Dois endpoints novos no backend para alimentar o painel: `GET /api/config` (config de rede não sensível) e `GET /api/audit` (últimas entradas de auditoria), cobertos por testes automatizados. Cliente HTTP único em `src/lib/api.ts`, sessão via JWT em `localStorage` com logout automático em 401.

### Changed

- Repositório tornado **público** para viabilizar *branch protection* real na `main` (indisponível de graça para repositórios privados em conta pessoal no GitHub). Ver justificativa e mitigação em `SECURITY.md`.

### Fixed

- Corrigida no VPS a ambiguidade de configuração SSH (`PasswordAuthentication` divergente entre `50-cloud-init.conf` e `60-cloudimg-settings.conf`): criado `/etc/ssh/sshd_config.d/00-xvpn-hardening.conf` (nome escolhido a dedo para ordenar **antes** de `50-cloud-init.conf`, já que o `sshd_config` usa o primeiro valor obtido, não o último — ver `PLAN.md` §9). `PasswordAuthentication no` e `PermitRootLogin prohibit-password` confirmados efetivos via `sshd -T` e validados com uma segunda sessão SSH independente.

### Security

- Descoberto que o pacote `samba`, recém-instalado na Fase 0, inicia `smbd`/`nmbd` escutando em `0.0.0.0:139`/`0.0.0.0:445` por padrão — víolaria o invariante do `AGENTS.md` de nunca expor Samba publicamente. Mitigado imediatamente parando e desabilitando os dois serviços até a Fase 5, quando o `smb.conf` será restrito a `wg0`/`lo`.

### Known limitations

- Achado na Fase 1: caminhos de rede com MTU efetivo menor que o padrão do WireGuard (1420) — ex.: cliente já atrás de outra VPN, CGNAT restritivo, algumas redes móveis — causam um "black hole" de PMTU (handshake e pacotes pequenos passam, tráfego HTTP/TLS trava silenciosamente). O cliente desktop (Fase 4/6) precisará expor um ajuste manual de MTU em Configurações/Diagnóstico; adicionado à especificação em `PLAN.md` §7.2.

### Fixed

- Corrigidos no deploy da Fase 2: permissão de `/etc/wireguard/server.key` (era ilegível para o usuário `xvpn`, corrigido para `640 root:xvpn`), `XVPN_DB_PATH` relativo incompatível com o `ProtectSystem=strict` da unit systemd (fixado para caminho absoluto em `/opt/xvpn/data/`), e ausência do binário `sqlite3` (CLI) no VPS, necessário pelo script de backup mesmo já usando `mattn/go-sqlite3` via cgo no binário Go.
