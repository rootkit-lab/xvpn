---
name: port-domain-registry-check
description: Verifica se uma nova porta, hostname público ou *.corp colide com o registro do PLAN.md §5 ou com o que já escuta no VPS. Use antes de server block Nginx, systemd, dnsmasq, Mongo, ou app de intranet no host compartilhado com landpages-ops.
---

# Checagem de registro de portas/domínios (XVPN / ihuull)

O VPS `206.189.224.72` hospeda o XVPN e o `landpages-ops`. Há **dois planos de DNS**: público (`xvpn.ihuull.com`, landings) e intranet (`*.corp.ihuull.com` → `10.66.66.1`, só com VPN). Antes de reservar porta, hostname ou server block, confirme que não colide com o documentado nem com o que está rodando.

## Uso

```bash
.cursor/skills/port-domain-registry-check/scripts/check.sh [usuario@host]
```

O script:
1. Extrai a seção 5 do `PLAN.md` (tabelas público / corp / portas).
2. Roda `ss -tulnp` no servidor via SSH.
3. Imprime os dois lado a lado para comparação manual.

Leia também [`docs/runbooks/cloudflare-dns.md`](../../../docs/runbooks/cloudflare-dns.md) antes de criar registro na zona `ihuull.com`.

## Procedimento ao adicionar um serviço novo

1. Rode o script (registro × realidade).
2. Escolha porta/hostname que não apareça em nenhuma das duas listas.
3. Classifique o hostname:
   - **Público** (`xvpn.ihuull.com`, landing): A no Cloudflare, DNS only se for API/WS, backend em `127.0.0.1`.
   - **Intranet** (`<slug>.corp.ihuull.com`): **sem** A público; só dnsmasq em `wg0:53`; Nginx `listen 10.66.66.1:443`.
4. Adicione a linha em `PLAN.md` §5 **antes** de configurar o serviço.
5. Depois de subir, rode o script de novo: bind exatamente como planejado (nem `0.0.0.0` a mais, nem porta faltando).

App desktop/intranet novo: skill `new-intranet-app` (checklist de slug, JWE `aud`, gate VPN).

## Regras importantes

- Backends HTTP do core atrás do Nginx público: `127.0.0.1:<porta>`, nunca `0.0.0.0`.
- Intranet, Samba, FileBrowser, dnsmasq, Mongo: nunca `0.0.0.0` nem `eth0`.
  - Samba / FileBrowser / xdriver: `10.66.66.1`
  - DNS interno: `10.66.66.1:53` somente
  - Mongo: `127.0.0.1:27017` somente — sem ufw
- **Não criar** A público para `corp` / `*.corp` / `xchat.corp` / `xgroup.corp` / `xdriver.corp` / `xadmin.corp` / `xgit.corp`.
- Não reutilize `8080` (API), `8081` (FileBrowser), `51820/udp` (WireGuard), `27017` (Mongo), `53` (dnsmasq) para outro serviço.
- `ldpops.appapisip.com` não é desta plataforma — não tome a porta dele.
