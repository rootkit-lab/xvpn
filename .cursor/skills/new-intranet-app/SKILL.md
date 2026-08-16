---
name: new-intranet-app
description: Checklist para criar um app novo da intranet ihuull (*.corp, JWE aud, gate VPN, marketplace.yaml único). Use antes de scaffold de app desktop ou hostname novo — evita duplicar xchat/xgroup/xdriver.
---

# App novo na intranet

App novo **só nasce** se não couber num slug já existente:

| Slug | Lockup / vitrine | Papel | Não refazer |
|---|---|---|---|
| xvpn | XVPN Client | Core, enroll, JWT/JWE, catálogo, “está conectado?” | — |
| xchat | XCHAT Client | Messenger | Segundo chat no painel |
| xgroup | XGROUP Social | Rede social (`/social`, `/xgroup`) | Misturar com xchat |
| xdriver | XDRIVER Drive | Arquivos (Samba + Drive nativo em `xdriver.corp`) | FileBrowser; expor arquivos na internet |
| marketplace | Marketplace Store | Loja pública | Segunda vitrine |

Nomes de UI: `shared/ui/react/products.ts` + `PLAN.md` §6.13. Slug minúsculo no código; `marketplace.yaml` `name` = `productDisplayName(slug)`.

Reuso obrigatório: identidade JWE (`aud` = slug), skill `desktop-app-ui`, gate VPN (`vpngate` / IPC do helper), `marketplace.yaml` único, blobs content-addressed, chrome de chat só via skill `chat-chrome`. Header global + logo do produto (`shared/ui/brand`). **Não** nascer segundo binário Go — API no `xvpn-server` (`PLAN.md` §6.13).

Todo app tem **três peças** (client só se precisar):

| Peça | Obrigatório | Onde |
|---|---|---|
| server | sim | handlers no monólito, prefixo `/api/<slug>/` ou paths já documentados |
| portal | sim | landing pública `<slug>.ihuull.com` e/ou app `<slug>.corp.ihuull.com` |
| client | não | `apps/<slug>/` Wails; um `marketplace.yaml`; `network: vpn` se exigir túnel |

`visibility` (ACL: global/restricted) ≠ `network` (public/vpn). Registrar hostname em `PLAN.md` §5 **antes** do Nginx.

## Checklist (nesta ordem)

1. Skill `port-domain-registry-check` + runbook [`docs/runbooks/cloudflare-dns.md`](../../../docs/runbooks/cloudflare-dns.md).
2. Escolher slug `[a-z0-9-]{2,20}`. Registrar em `PLAN.md` §5 **antes** de Nginx.
3. Hostname **só** intranet: `<slug>.corp.ihuull.com`. Registrar em `/admin/dns` (A → `10.66.66.1` ou outro `10.66.66.x`) e **Aplicar**. **Sem** A público.
4. Nginx: `listen 10.66.66.1:443` + `allow 10.66.66.0/24; deny all;`. Sem porta nova no ufw.
5. JWE `aud` = slug. Login pede esse `aud`. Token só em memória no desktop.
6. Gate: recusar API se helper `disconnected` ou se o gateway `10.66.66.1:443` não responder. DNS canônico é o dnsmasq da `wg0` (`PLAN.md` §5.4).
7. `apps/<slug>/marketplace.yaml` — um slug, sem segundo manifesto.
8. UI: alias `@xvpn/ui` + `shared/ui` (skill `desktop-app-ui`, `PLAN.md` §6.12). Não copiar `index.css` nem inventar paleta.
9. Documentar rotas em [`docs/api.md`](../../../docs/api.md).

Disco `apps/xvpn-chat` pode permanecer mesmo com slug `xchat` — não renomear módulo Wails sem necessidade.
