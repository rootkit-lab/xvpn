---
name: samba-user-ops
description: Cria, lista ou remove usuários Samba manualmente no VPS do XVPN (share [shared], acessível só via wg0). Não há sincronização automática com o painel/usuários XVPN — é uma decisão de escopo da Fase 5 do ROADMAP, ver justificativa abaixo. Use quando o usuário pedir para dar acesso a arquivos do servidor pra alguém, ou remover esse acesso.
---

# Operações manuais de usuário Samba (XVPN)

Pré-requisito: Fase 5 do `ROADMAP.md` concluída — Samba instalado e configurado (`server/deploy/samba/smb.conf` aplicado em `/etc/samba/smb.conf` no VPS, serviço `smbd` ativo, bind restrito a `wg0`/`lo`).

## Por que não é sincronizado com o painel XVPN

O processo `xvpn-server` roda como usuário de sistema sem privilégio para criar contas Unix/Samba (least privilege — só tem `CAP_NET_ADMIN`, ver `PLAN.md` §6). Automatizar isso exigiria dar ao painel poder de criar usuários de sistema, o que aumenta bastante a superfície de risco de qualquer bug/RCE no painel. Por ora, a criação de usuário Samba é manual (via esta skill), independente da criação de usuário XVPN pelo painel. Isso pode ser revisitado numa fase futura se fizer sentido (ex.: endpoint de admin dedicado, com seu próprio hardening).

## Scripts disponíveis

### Criar/atualizar usuário

```bash
.cursor/skills/samba-user-ops/scripts/add-user.sh <username> [usuario@host]
```

Cria o usuário de sistema (sem shell, sem home — `--no-create-home --shell /usr/sbin/nologin`), adiciona ao grupo `xvpn-samba` (dono do compartilhamento `[shared]`), gera uma senha aleatória e cadastra no Samba (`smbpasswd`). Imprime a senha gerada **uma única vez** no terminal — copie e passe pro usuário por um canal seguro; não fica salva em nenhum arquivo.

Se o usuário já existir (de sistema), só atualiza a senha/conta Samba.

### Listar usuários Samba ativos

```bash
.cursor/skills/samba-user-ops/scripts/list-users.sh [usuario@host]
```

### Remover usuário

```bash
.cursor/skills/samba-user-ops/scripts/remove-user.sh <username> [usuario@host]
```

Remove a conta Samba e o usuário de sistema.

## Acesso ao compartilhamento

Com a VPN conectada (túnel WireGuard ativo):
- Windows: barra de endereço do Explorer → `\\10.66.66.1\shared`
- Linux (GVFS/Nautilus/Dolphin): `smb://10.66.66.1/shared`
- macOS: Finder → Ir → Conectar ao Servidor → `smb://10.66.66.1/shared`

Sem a VPN ativa, a conexão nem chega no servidor — `smbd` só escuta em `wg0`/`lo` (ver `PLAN.md` §3.4 e §5).
