---
name: samba-user-ops
description: Garante o usuário de sistema xvpn-shared (force user do share [shared]) e opera contas Samba legadas no VPS do XVPN. Os shares pessoais home-<user> e o [shared] em guest são sincronizados pelo painel (Fases 13/14) — esta skill não cria home-* e não é o caminho normal de acesso. Use quando precisar recriar xvpn-shared após reprovisionar o Samba, ou limpar contas smbpasswd antigas.
---

# Operações Samba no VPS (XVPN)

Pré-requisito: Samba instalado com `server/deploy/samba/smb.conf` aplicado em `/etc/samba/smb.conf`, `smbd` ativo, bind só em `wg0`/`lo`.

## Modelo atual (Fases 13/14)

- Shares pessoais `[home-<username>]`: provisionados pelo painel (`PUT /api/users/:id/file-access` → `xvpn-user-provision`), com `guest ok = yes` + `force user = <username>`. A VPN é a barreira de autenticação.
- Share `[shared]`: também `guest ok = yes`, com `force user = xvpn-shared` (conta de sistema no grupo `xvpn-samba`) e `force group = xvpn-samba`.
- O cliente desktop abre `smb-home` / `smb-shared` sem senha Samba.

**Não use esta skill para “dar acesso a arquivos” a um usuário XVPN** — ligue Samba/SFTP no painel. Contas `smbpasswd` manuais eram o modelo da Fase 5 e ficaram obsoletas para o fluxo normal.

## Garantir `xvpn-shared` (force user do [shared])

```bash
ssh root@206.189.224.72 'getent group xvpn-samba >/dev/null || groupadd xvpn-samba
getent passwd xvpn-shared >/dev/null || useradd --system --no-create-home --shell /usr/sbin/nologin -g xvpn-samba xvpn-shared
install -d -o root -g xvpn-samba -m 2770 /srv/xvpn/shared
id xvpn-shared'
```

## Scripts legados (contas smbpasswd)

Ainda úteis só para limpeza ou compatibilidade com clientes antigos que autentiquem de verdade:

```bash
.cursor/skills/samba-user-ops/scripts/add-user.sh <username> [usuario@host]
.cursor/skills/samba-user-ops/scripts/list-users.sh [usuario@host]
.cursor/skills/samba-user-ops/scripts/remove-user.sh <username> [usuario@host]
```

## Acesso com a VPN conectada

- Pessoal: `smb://10.66.66.1/home-<usuario>` ou botão “Meus arquivos” no cliente
- Compartilhado: `smb://10.66.66.1/shared` ou botão “Compartilhado”
- FileBrowser: `http://10.66.66.1:8081`
