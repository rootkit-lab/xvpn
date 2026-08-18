# XCODESPACES remoto (Fase 50)

## O que é

- **Editor rápido** (`xcodespaces.corp/:id`): Monaco no worktree (Fase 49).
- **Codespace remoto** (`cs-<id>.corp.ihuull.com`): clone + container Docker + openvscode-server. Shell só no container.

## Operação no VPS

1. Instalar Docker (socket Unix; **não** pôr o user `xvpn` no grupo `docker`).
2. Build da imagem DX (raiz do clone): `docker build -f server/deploy/codespace/Dockerfile -t ihuull/codespace:1.98.2 .` (FROM `gitpod/openvscode-server:1.98.2`). Sem `docker.sock`.
3. Deploy do `xvpn-server` **e** do `xvpn-user-provision` (subcomando `cs-apply`).
4. Nginx: server block `~^cs-[a-f0-9]+\.corp\.ihuull\.com$` em `server/deploy/nginx/corp.conf` — `nginx -t && systemctl reload nginx`.
5. Catch-all `*.corp` no dnsmasq já resolve `cs-<id>.corp`. Sem A público.

## Disco e teto

- Clone: `/opt/xvpn/data/codespaces/<user>/<slug>/<id>/workspace` → no container: `/home/workspace/project` (HOME do IDE ≠ clone)
- Settings do workbench: `/opt/xvpn/data/codespaces/<user>/<slug>/<id>/machine-settings.json` (fora do Git)
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

## DX (Fase 51)

- Imagem `ihuull/codespace:1.98.2` (Go + Node + tema). Create novo usa essa tag; codespace antigo fica na imagem em que nasceu até Recreate.
- Tema **ihuull Dark** + Welcome XCODESPACES: `shared/vscode-theme` (gerar com `node shared/vscode-theme/gen.mjs`). Settings em `machine-settings.json` ao lado do volume (Machine do IDE), **não** em `.vscode/` do clone.
- Se o Source Control listar `.cache` / `.openvscode-server`, o container ainda monta o clone no HOME — **Recreate** (start de container antigo não troca o mount).
- ENVs do projeto: Settings do repo → **Codespaces**. Entram no Create via `--env-file` ao lado do volume (não no argv). `XCS_LLM_*` o proxy lê no servidor; a key **não** vai ao container. Mudança de ENV = Recreate.
- Chat / generate commit: ainda pendentes (51.4). Sem Copilot/Continue; sem `docker.sock`.
