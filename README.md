<p align="center">
  <img src="./assets/logo.png" alt="XVPN" width="180">
</p>

# XVPN

Rede privada própria com exit node via VPS, painel web de administração e cliente desktop multiplataforma — **Go**, **Wails3**, **React + Tailwind + shadcn/ui**.

> Status: Fases **0–5** na `main` (produção). Fases **6–7** em PRs abertas; **Fase 8** (observabilidade/docs) nesta linha de trabalho. Checklist: [`ROADMAP.md`](./ROADMAP.md). Arquitetura: [`PLAN.md`](./PLAN.md).

---

## O que é

Dispositivos Windows/Linux entram numa rede privada cujo nó central é um VPS próprio e saem para a internet com o IP desse servidor (full-tunnel), com:

- **VPN WireGuard** com engine embarcada no cliente (kernel no Linux, `wireguard-go`+`wintun` no Windows).
- **Painel web** em `https://vpn.officeempresa.com` para usuários, dispositivos, convites e auditoria.
- **Cliente desktop** (Wails3): enrollment por convite, conectar/desconectar, bandeja; helper privilegiado separado da GUI.
- **Arquivos só na VPN**: Samba (`\\10.66.66.1\shared`) e FileBrowser Quantum (`http://10.66.66.1:8081`) — nunca na internet pública.

## Como operar (produção)

| Item | Valor |
|---|---|
| Painel / API | `https://vpn.officeempresa.com` |
| VPS | `206.189.224.72` (Ubuntu 26.04) |
| Sub-rede WireGuard | `10.66.66.0/24` (servidor `10.66.66.1`) |
| Serviço | `systemctl status xvpn-server` |
| Logs | `journalctl -u xvpn-server -f` (JSON estruturado a partir da Fase 8) |
| Saúde | `curl -sS https://vpn.officeempresa.com/api/status` |

Deploy do servidor: build embutindo o painel e substituir `/opt/xvpn/bin/xvpn-server` — ver [`server/README.md`](./server/README.md).

Cliente (dev/instalação): ver [`apps/xvpn-client/README.md`](./apps/xvpn-client/README.md). Empacotamento `.deb`/AppImage/NSIS está na Fase 7 (PR).

## Desenvolvimento local

```bash
# Servidor
cd server/web && npm ci && npm run build && cd ..
go test ./...
go run ./cmd/xvpn-server   # precisa de WG + env; ver server/README.md

# Cliente (Linux)
cd apps/xvpn-client
task build
# helper: sudo ./bin/xvpn-client --helper   (ou unit systemd — apps/xvpn-client/README.md)
./bin/xvpn-client
```

## Estrutura

```
xvpn/
├── PLAN.md / ROADMAP.md / SECURITY.md / CONTRIBUTING.md / AGENTS.md
├── apps/            # produtos distribuíveis ao usuário final
│   └── xvpn-client/ # Desktop Wails3 (GUI + --helper)
├── server/          # API Gin + painel React embutido (embed.FS)
├── .cursor/         # rules, hooks, skills (auditoria VPS, peers WG, PRs…)
└── assets/          # logo
```

`apps/` guarda o que é empacotado e distribuído; `server/` (e futuramente `shared/`)
ficam na raiz porque são plataforma, não produtos de catálogo.

## Observabilidade (Fase 8)

- **Servidor**: `log/slog` em JSON (`component=xvpn-server`); requests HTTP sem headers/corpo. Env: `XVPN_LOG_LEVEL`, `XVPN_LOG_FORMAT`, `GIN_MODE=release`.
- **Cliente**: `internal/applog` (JSON + ring em memória das últimas linhas).
- **Métricas básicas**: `GET /api/status` inclui `connected_peers`, `total_peers`, `uptime_seconds`, `receive_bytes_total`, `transmit_bytes_total` (agregado WireGuard).

## Segurança (invariantes)

1. Chave privada WireGuard **nunca** sai do dispositivo do cliente.
2. Samba/FileBrowser só em `wg0` (`10.66.66.1`), nunca em `0.0.0.0`/`eth0`.
3. Firewall padrão-nega; portas públicas só as de [`PLAN.md` §5](./PLAN.md#5-alocação-de-rede-portas-e-domínios-registro-para-não-colidir-com-landpages-ops).
4. Sem segredos no Git.

Auditoria rápida do VPS: skill `vps-security-audit` (`.cursor/skills/vps-security-audit/`).

## Stack

- **Servidor**: Go, Gin, GORM/SQLite, `wgctrl`+netlink, React/Vite/Tailwind/shadcn, Nginx, Samba, FileBrowser Quantum.
- **Cliente**: Go, Wails v3, React/Tailwind/shadcn, WireGuard kernel / `wireguard-go`+`wintun`.
- **Infra**: Ubuntu 26.04, systemd, ufw, fail2ban.

## Licença / visibilidade

Repositório **público** (branch protection na `main`). Segredos nunca commitados — ver [`SECURITY.md`](./SECURITY.md). Licença de código ainda não definida.
