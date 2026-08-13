---
name: vps-security-audit
description: Executa uma auditoria rápida de segurança e rede no VPS de produção do XVPN (206.189.224.72) — verifica configuração SSH, firewall (ufw), ip_forward, portas escutando e escopo de rede do Samba/WireGuard. Use quando o usuário pedir para checar a segurança do servidor, antes/depois de mudanças de firewall ou rede, ou periodicamente conforme o checklist da Fase 8 do ROADMAP.
---

# Auditoria de segurança do VPS (XVPN)

Roda os mesmos checks read-only usados no diagnóstico inicial do projeto (documentados em `PLAN.md` §1) contra o servidor de produção, para detectar regressões de configuração.

## Uso

```bash
.cursor/skills/vps-security-audit/scripts/audit.sh [usuario@host]
```

Se `usuario@host` não for informado, usa `root@206.189.224.72` por padrão.

## O que o script verifica

1. Configuração efetiva do SSH (`sshd -T`) — `passwordauthentication`, `permitrootlogin`, `kbdinteractiveauthentication`.
2. Status do `ufw` (deve estar ativo, com regras restritas ao registrado em `PLAN.md` §5).
3. `net.ipv4.ip_forward` (deve ser `1` só depois da Fase 1 do roadmap; antes disso, `0` é esperado).
4. Todas as portas TCP/UDP escutando no servidor (`ss -tulnp`) — comparar manualmente com a tabela de `PLAN.md` §5.
5. Configuração de bind de interfaces do Samba (`bind interfaces only`, `interfaces`) — deve estar restrito a `wg0 lo`.
6. Status da interface WireGuard (`wg show`) — peers ativos e último handshake.

## Como interpretar o resultado

Depois de rodar o script, verifique este checklist contra a saída (não assuma — confirme item a item):

- [ ] `passwordauthentication` = `no`
- [ ] `permitrootlogin` = `prohibit-password` (ou `no`)
- [ ] `ufw` ativo, com **apenas** as portas esperadas (`22/tcp`, `80/tcp`, `443/tcp`, `51820/udp`)
- [ ] Nenhuma porta de Samba (`139`, `445`) ou FileBrowser (`8081`) aparecendo em `ss -tulnp` vinculada a `0.0.0.0` ou ao IP público `206.189.224.72` — só devem aparecer vinculadas a `10.66.66.1`/`127.0.0.1` ou não aparecer se ainda não instaladas
- [ ] `smb.conf` (se já instalado) tem `interfaces = 10.66.66.1/24 127.0.0.1/8` e `bind interfaces only = yes`. **Não** use `interfaces = wg0 lo` (nome de interface) — o Samba não detecta corretamente interfaces ponto-a-ponto/sem broadcast como o WireGuard por esse método, e o `smbd` silenciosamente cai só em `127.0.0.1` (achado na Fase 5, ver `ROADMAP.md`)
- [ ] Nenhum serviço inesperado escutando em porta não documentada em `PLAN.md` §5

Se algum item falhar, reporte ao usuário explicitamente qual invariante de `SECURITY.md` foi violada antes de corrigir, para que a correção seja intencional e não silenciosa.
