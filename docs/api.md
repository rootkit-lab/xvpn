# API do xvpn-server

Issuer SSO: `https://xvpn.ihuull.com`. Tokens são **só JWE** (`dir` + `A256GCM`) com `aud` por app (`xvpn`, `xchat`, `xgroup`, `xdriver`). JWT HMAC é rejeitado.

Comunicação de app no desktop: `https://xchat.corp.ihuull.com` (intranet). Painel/enroll: `https://xvpn.ihuull.com`.

Auth: `Authorization: Bearer <token>` salvo no header, nunca na query do WebSocket.

## Auth e sessão

| Método | Path | Auth | Notas |
|---|---|---|---|
| POST | `/api/auth/login` | público + rate limit | Body `{username,password,aud?}`. Recusa `xbot`. |
| GET | `/api/auth/me` | JWT/JWE | Papel atual do banco (não só o claim). |
| POST | `/api/devices/enroll` | convite | Devolve IP, pubkey do server, `dns: ["10.66.66.1"]`. |
| GET | `/api/status` | público | Saúde, `api_version`, peers. |

## Identidade no túnel (só `10.66.66.1:8080`)

| Método | Path | Auth |
|---|---|---|
| GET | `/api/me` | IP do peer na subnet |
| POST | `/api/me/ssh-key` | IP do peer |

## Usuários e devices (RBAC)

CRUD em `/api/users`, convite, reset de senha, file-access, devices — `viewer+` lê, `admin+` escreve. Ver `PLAN.md` §6.7.

## xchat / xgroup (`/api/social/*`)

Mesmo backend. Hostname de produto: `xchat.corp` (WS/mensagens) e `xgroup.corp` (rede). Paths internos permanecem `/api/social/...` e `/social/...` (alias `/xgroup/...`).

| Método | Path | Notas |
|---|---|---|
| GET/PATCH | `/api/social/profile` | Perfil xgroup |
| GET | `/api/social/people` | Diretório |
| POST/DELETE | `/api/social/follow/:username` | |
| GET/POST | `/api/social/groups` | |
| GET/POST | `/api/social/threads` | DM |
| GET/POST | `/api/social/threads/:kind/:id/messages` | |
| POST | `/api/social/attachments` | 32 MiB, JWT |
| GET/POST | `/api/social/stories` | 24h |
| POST | `/api/social/acks` | entregue/lido |
| GET | `/api/ws` | Auth no **primeiro frame**, nunca `?token=` |

## Marketplace

| Método | Path | Auth |
|---|---|---|
| GET | `/api/marketplace/apps` | sessão |
| GET | `/api/marketplace/assets/:id/download` | sessão + ACL |
| POST | `/api/marketplace/sync` | `XVPN_PUBLISH_TOKEN` |

## Hooks (xbot)

| Método | Path | Auth |
|---|---|---|
| POST | `/api/hooks/chat/broadcast` | `Authorization: Bearer $XBOT_TOKEN` |

Body: `{ "body": "texto", "group": "Sistema" }`. Cria/usa o grupo, adiciona todos os membros humanos, posta como `xbot`. Secret no Actions: `XBOT_TOKEN`. Nunca JWT de pessoa.

## Waitlist

`POST /api/waitlist` — único write público além de login/enroll. Rate limit por IP.

## Superfícies que **não** são esta API

- Samba `10.66.66.1:445` e FileBrowser `10.66.66.1:8081` / `xdriver.corp` — sem fork.
- `ldpops.appapisip.com` — outro processo.
