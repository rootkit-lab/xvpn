---
name: xbot-notify
description: Envia uma mensagem do xbot para o grupo Sistema via POST /api/hooks/chat/broadcast. Use após deploy/merge quando o usuário pedir para notificar no chat, ou ao rodar o workflow xbot-notify.
---

# Notificar no xchat (xbot)

Usuário de sistema `xbot` (role `xbot`, sem login, sem peer WG). Actions usam **client credentials** (`XBOT_TOKEN`), nunca JWT de humano.

## Uso local / agente

```bash
.cursor/skills/xbot-notify/scripts/notify.sh "mensagem"
```

Requer `XBOT_TOKEN` no ambiente. Default URL: `https://xvpn.ihuull.com` (o hook é no core; a mensagem aparece no xchat).

## Workflow

`.github/workflows/xbot-notify.yml` — `workflow_dispatch` com input `body`. Secret do repo: `XBOT_TOKEN` (mesmo valor de `XVPN_XBOT_TOKEN` no VPS).

Não commitar o token. Não usar a senha de um admin.
