---
name: marketplace-publish
description: Explica e executa a publicação de um app do catálogo (xvpn-chat, xvpn-client). Use quando o usuário não vê o chat/app no marketplace, Apps do cliente desktop, ou /my/marketplace — o sync ignora source:build sem GitHub Release.
---

# Publicar no marketplace

O painel e o cliente desktop leem `GET /api/marketplace/apps`. O sync (`marketplace-sync.yml` / `release-chat.yml` / `release-client.yml`) **pula** apps `source: build` se a URL da release 404 (sem `.deb`/`.exe`). Por isso o **XVPN Chat some do catálogo até existir tag `xvpn-chat-v*`**.

Não invente linha no banco. Não espere o feat na `main` sozinho popular o Apps.

## Checklist

1. `release-status` — PR `chore(main): release xvpn-chat|xvpn-client X.Y.Z` com label `autorelease: pending`.
2. Land essa PR (`land-pr`) **depois** dos feats que devem entrar na versão (a primeira release acumula tudo desde `0.0.0`).
3. A tag dispara `release-chat.yml` / `release-client.yml` → GitHub Release + `POST /api/marketplace/sync`.
   Se a tag veio do release-please (`GITHUB_TOKEN`), o `on: push: tags` **não roda**. Dispare `release-chat.yml` via `workflow_dispatch` com a tag (`xvpn-chat-v0.1.1`). Sem `.deb`/`.exe` na Release o sync **pula** o app — foi o caso da `xvpn-chat-v0.1.0`.
4. Confirme: o app aparece em `/my/marketplace` e no Apps do `xvpn-client` (filtrado pela plataforma do SO).

## Se o catálogo ainda estiver vazio

- Tag existe mas sync falhou → `workflow_dispatch` em `marketplace-sync.yml`.
- Sem PR de release → o `release-please` ainda não viu um `feat`/`fix` naquele pacote; não crie a tag na mão.
- Cliente desktop só lista assets da plataforma atual (Linux não mostra `.exe`).
