# XCODESPACES remoto (Fase 50)

## O que é

- **Editor rápido** (`xcodespaces.corp/:id`): Monaco no worktree (Fase 49).
- **Codespace remoto** (`cs-<id>.corp.ihuull.com`): clone + container Docker + openvscode-server. Shell só no container.

## Operação no VPS

1. Instalar Docker (socket Unix; **não** pôr o user `xvpn` no grupo `docker`).
2. `docker pull gitpod/openvscode-server:1.98.2`
3. Deploy do `xvpn-server` **e** do `xvpn-user-provision` (subcomando `cs-apply`).
4. Nginx: server block `~^cs-[a-f0-9]+\.corp\.ihuull\.com$` em `server/deploy/nginx/corp.conf` — `nginx -t && systemctl reload nginx`.
5. Catch-all `*.corp` no dnsmasq já resolve `cs-<id>.corp`. Sem A público.

## Disco e teto

- Clone: `/opt/xvpn/data/codespaces/<user>/<slug>/<id>/workspace`
- Bare do forge **não** é montado no container.
- 1 codespace em execução; ~1,5 GiB / 1 vCPU; idle-stop 30 min (volume fica).
- Delete apaga container + volume.

## Se o VS Code não abrir

- `docker ps -a --filter label=xvpn.codespace`
- Porta só em `127.0.0.1:19000–19007` (`ss -tlnp | grep 1900`)
- Host fora da VPN não resolve `cs-*`
- `docker.sock` **não** deve existir dentro do container
- Host `cs-*` não serve a API de controle (`/api/xcodespaces`, `/api/projects`); só SSO (`/api/auth/session`, `/api/auth/redeem`) passa no Gin
- openvscode exige connection token; o proxy injeta o cookie `vscode-tkn` (não `?tkn=` — isso 302-loop) e remove `ihuull_session`/`Authorization` antes do container
- Token Git do codespace vale só para o slug daquele workspace e some no stop/idle-stop

## DX (Fase 51 — pendente)

Imagem, tema ihuull, chat próprio (GLM e outros via proxy), generate commit e ENVs no Settings do repo: `ROADMAP.md` Fase 51 e `PLAN.md` §3.6. Sem Copilot/Continue na imagem; sem `docker.sock`; key de LLM não entra no container.
