---
name: deploy-xvpn-server
description: Compila o painel+binário xvpn-server (cgo) e troca o binário no VPS de produção com backup e health check. Use quando o usuário pedir deploy, após mergear uma PR que altera server/ ou server/web, ou disser continuar depois do land-pr do painel. Não abre porta, não mexe em Nginx/Samba/firewall.
---

# Deploy do xvpn-server (produção)

VPS real: `root@206.189.224.72`. `go-sqlite3` exige **cgo** — nunca `CGO_ENABLED=0`.

## Uso

```bash
.cursor/skills/deploy-xvpn-server/scripts/deploy.sh
```

O script: baseline read-only → `npm ci` (chat frontend + `server/web`) → `npm run build` → `CGO_ENABLED=1 go build` → backup em `/opt/xvpn/bin/xvpn-server.bak-*` → `install` + `systemctl restart` → health.

## Regras

- Nginx/Samba/ufw **não** mudam neste deploy (WS já está em `/api/ws`).
- Health mínimo: `systemctl is-active`, `GET /api/status` 200 (local e `https://vpn.officeempresa.com`), `GET /api/ws?token=` → 400, Samba só `10.66.66.1:445` / `127.0.0.1:445`, handshake WireGuard intacto.
- Não use senhas de transcripts antigos para testar login.
- Se o serviço não subir, restaure o `.bak-*` mais recente e `systemctl restart xvpn-server`.
