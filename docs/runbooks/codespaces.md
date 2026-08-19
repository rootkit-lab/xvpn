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

## Demo ports (Fase 56)

O botão **Ports → Forward a Port** do OpenVSCode tenta túnel Microsoft (internet). Não use.

Na VPN: `http://demo-<nome>.corp.ihuull.com:<porta>` chega no processo **dentro do container** (VIP `10.66.66.254` → DNAT). Nome no painel XCODESPACES → **Demo**. O app tem que escutar `0.0.0.0` (Vite: `--host`). Sem A público. Deploy do helper `xvpn-user-provision` **e** do `xvpn-server`.

## DX (Fase 51)

- Imagem `ihuull/codespace:1.98.2` (Go + Node + tema + Open VSX Go/ESLint/Prettier/Markdown/YAML + `xcs-analyze`). Create novo usa essa tag; codespace antigo fica na imagem em que nasceu até Recreate.
- Tema **ihuull Dark** + Welcome **XCODESPACES** (não o *Get Started* da Microsoft): `shared/vscode-theme` (gerar com `node shared/vscode-theme/gen.mjs`). `extension.js` abre o walkthrough ihuull no first-open; Machine settings escondem `SetupWeb`. Settings em `machine-settings.json` ao lado do volume (Machine do IDE), **não** em `.vscode/` do clone.
- Playground XGIT **`teste`**: fonte em `server/deploy/codespace/sample-teste/`. Re-semear o bare no VPS: `sudo -u xvpn ./server/deploy/codespace/seed-teste.sh`. Delete + Create do codespace (ou `git pull` no volume) para ver os arquivos. Checklist no README do repo.
- Se o Source Control listar `.cache` / `.openvscode-server`, o container ainda monta o clone no HOME — **Recreate** (start de container antigo não troca o mount).
- ENVs do projeto: Settings do repo → **Codespaces**. Entram no Create via `--env-file` ao lado do volume (não no argv). Mudança de ENV = Recreate.
- Assistente (GLM / generate commit / agente): **xadmin → Settings**. Key write-only no VPS. A extensão `ihuull.codespace` roda no **Node** do container — `fetch` relativo quebra (`Failed to parse URL`). Ela chama `https://cs-<id>.corp.ihuull.com/api/xcodespaces/llm/*` com o token Git de `.git/xvpn-credentials` (o cookie do browser não existe no extension host). O container **não** usa o dnsmasq da VPN (DNS da VPC `10.136`); o helper grava `--add-host` para `xgit`/`xcodespaces`/`cs-<id>` → `10.66.66.1`. Nginx desses hosts também `allow 172.17.0.0/16` (docker0) — sem isso o fetch vira 403/`fetch failed`. GLM-4.7+ devolve thinking em `reasoning_content` e `content` vazio — o proxy desliga thinking (exceto GLM-5.3) e lê os dois campos; toast **resposta vazia** sem isso.
- Chat nativo **CHAT / COPILOT EDITS** do OpenVSCode **não** é o produto (Fase 52). Machine settings escondem o command center e deixam a **secondary sidebar visível**. A view do agente entra no container nativo `workbench.panel.chat` (direita) — o 1.98 ignora `viewsContainers.secondarySidebar`. A extensão desinstala Copilot/Continue/Cline se o usuário instalar, fecha o Chat/Edits nativos e foca a auxiliary bar. Chrome: modos **Agent / Ask / Debug / Plan** + seletor de modelo (`GET /api/xcodespaces/llm/models`; override no `POST /chat` só se o ID estiver no catálogo do provedor). Timeline: Thinking, resumo expansível e cards de tool (sem `tool N…`). Composer `@` `#` `$` `/`; terminal espera o comando (`python3` + campo `env`). MCP think/memory/docs. Logs em `.cursor/agent/` (fallback `/tmp/xcs-agent`); Review/Stop. Workspace = clone `xgit.corp`, não GitHub/fork. `.cursor/hooks.json` só inspecionado (não executa o `command`). O GET também devolve `git_name`/`git_email`; a extensão (e o helper no Create/Start) grava `user.name`/`user.email` no clone — sem isso o Source Control mostra *configure your user.name*. Skills = `.cursor/skills`; agente = `AGENTS.md` (ou contrato ihuull se o clone não tiver); tools só no clone, com `glob` (write/term com **Aplicar**). Loop no container: teto **24** turns; no teto o agente resume o que já leu (Ask, sem tools) em vez de só pedir para reformular. Sem Copilot/Continue bakeados; sem `docker.sock`. Troca da extensão = rebuild da imagem + **Recreate** (start antigo não troca a layer). Troca do parse LLM / `tool_calls` / `/models` / `maxLLMChatMsgs` = só deploy do `xvpn-server`. Identidade no Create/Start = helper `xvpn-user-provision`.
