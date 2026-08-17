# API do xvpn-server

Issuer SSO: `https://xauth.ihuull.com` (leitura ainda aceita o issuer legado `https://xvpn.ihuull.com`). Tokens são **só JWE** (`dir` + `A256GCM`) com `aud` por app (`xvpn`, `xchat`, `xgroup`, `xdriver`). JWT HMAC é rejeitado.

Comunicação de app no desktop: `https://xchat.corp.ihuull.com` (intranet). Login web: `https://xauth.ihuull.com`. Portal/enroll: `https://xvpn.ihuull.com` (`/` portal; `/admin` operação). Loja: `https://marketplace.ihuull.com`. Drive: `https://xdriver.corp.ihuull.com` (só VPN; `xdriver.ihuull.com` não serve). Marketing xgroup: `https://xgroup.ihuull.com` (landing + perfil `/:user` com JWE; sem WS). App: `https://xgroup.corp.ihuull.com`.

Auth no browser: cookie `ihuull_session` (`Domain=.ihuull.com`, Secure, HttpOnly, SameSite=Lax), emitido só em `xauth.ihuull.com`. Desktop: `Authorization: Bearer <token>` em memória, sem cookie. Nunca na query do WebSocket.

## Auth e sessão

| Método | Path | Auth | Notas |
|---|---|---|---|
| POST | `/api/auth/login` | público + rate limit | Body `{username,password,aud?}`. Recusa `xbot`. Em `xauth` também grava o cookie de sessão. |
| POST | `/api/auth/logout` | público | Apaga o cookie `.ihuull.com`. |
| GET | `/api/auth/me` | Bearer ou cookie | Papel atual do banco (não só o claim). |
| POST | `/api/devices/enroll` | convite | Devolve IP, pubkey do server, `dns: ["10.66.66.1"]`. |
| GET | `/api/status` | público | Saúde, `api_version`, peers. |

## Identidade no túnel (só `10.66.66.1:8080`)

| Método | Path | Auth |
|---|---|---|
| GET | `/api/me` | IP do peer na subnet |
| POST | `/api/me/ssh-key` | IP do peer |

## Usuários e devices (RBAC)

CRUD em `/api/users`, convite, reset de senha, file-access, devices — `viewer+` lê, `admin+` escreve. Ver `PLAN.md` §6.7.

`User.products` (`core` \| `marketplace` \| `xgroup` \| `xdriver`): escopo de um `admin`. Lista vazia = irrestrito. `super_admin` ignora. Escritas de produto exigem o id na lista (`PUT /marketplace/apps/:id/access` → `marketplace`; `DELETE /devices/:id` e waitlist/config → `core`; file-access → `xdriver`). IAM (users/roles) não é produto.

## xchat / xgroup (`/api/social/*`)

Mesmo backend. Hostname de produto: `xchat.corp` (WS/mensagens) e `xgroup.corp` (rede). Paths internos permanecem `/api/social/...` e `/social/...` (alias `/xgroup/...`).

| Método | Path | Notas |
|---|---|---|
| GET/PATCH | `/api/social/profile` | Perfil xgroup. PATCH: `display_name`, `bio`, `avatar_url`, `banner_url` (`attachment:<id>`), `theme` (paleta: `primary`, `safe`, `xgroup`…) |
| GET | `/api/social/people` | Diretório |
| POST/DELETE | `/api/social/follow/:username` | |
| GET/POST | `/api/social/groups` | |
| GET/POST | `/api/social/threads` | DM |
| GET/POST | `/api/social/threads/:kind/:id/messages` | |
| POST | `/api/social/attachments` | 32 MiB, JWT |
| GET/POST | `/api/social/stories` | 24h |
| POST | `/api/social/acks` | entregue/lido |
| GET | `/api/social/feed` | Timeline (self + following; mural geral se vazio) |
| POST | `/api/social/posts` | Máx. 280 caracteres |
| GET | `/api/social/u/:username` | Perfil + `presence` visível (`invisible` vira `offline`) |
| GET | `/api/social/u/:username/posts` | Posts do perfil (`presence` do autor) |
| DELETE | `/api/social/posts/:id` | Autor ou admin |
| GET | `/api/ws` | Auth no **primeiro frame**, nunca `?token=` |

## XDriver (`xdriver.corp.ihuull.com` apenas)

| Método | Path | Notas |
|---|---|---|
| GET | `/api/driver/ls` | `root=home\|shared`, `path=` |
| POST | `/api/driver/mkdir` | |
| POST | `/api/driver/upload` | multipart, 2 GiB |
| GET | `/api/driver/download` | |
| DELETE | `/api/driver/rm` | path obrigatório |

Qualquer outro `Host` → 404. `xdriver.ihuull.com` não serve estas rotas.

## Marketplace (`marketplace.ihuull.com`)

| Método | Path | Auth |
|---|---|---|
| GET | `/api/marketplace/apps` | sessão + ACL + `network` |
| GET | `/api/marketplace/assets/:id/download` | sessão + ACL + `network` |
| POST | `/api/marketplace/sync` | `XVPN_PUBLISH_TOKEN` |

`visibility` (quem, ACL) ≠ `network` (onde). App `network: vpn` não lista nem baixa em `marketplace.ihuull.com` sem túnel; aparece quando o peer está na VPN (`10.66.66.0/24`). Host `*.corp` sozinho não basta. `network: public` aparece na loja pública com JWE (nunca anônimo).

ACL admin: `PUT /api/marketplace/apps/:id/access` no painel (`xvpn.ihuull.com/admin/marketplace`), só com escopo `marketplace` (ou admin irrestrito / `super_admin`).

## Hooks (xbot)

| Método | Path | Auth |
|---|---|---|
| POST | `/api/hooks/chat/broadcast` | `Authorization: Bearer $XBOT_TOKEN` |

Body: `{ "body": "texto", "group": "Sistema" }`. Cria/usa o grupo, adiciona todos os membros humanos, posta como `xbot`. Secret no Actions: `XBOT_TOKEN`. Nunca JWT de pessoa.

## Waitlist

`POST /api/waitlist` — único write público além de login/enroll. Rate limit por IP.

## Superfícies que **não** são esta API

- Samba `10.66.66.1:445`. XDriver web só em `xdriver.corp` (mesmo `xvpn-server`). `xdriver.ihuull.com` não serve o Drive. Sem FileBrowser.
- `ldpops.appapisip.com` — outro processo.
