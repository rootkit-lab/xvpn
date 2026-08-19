# API do xvpn-server

Issuer SSO: `https://xauth.ihuull.com` (leitura ainda aceita o issuer legado `https://xvpn.ihuull.com`). Tokens são **só JWE** (`dir` + `A256GCM`) com `aud` por app (`xvpn`, `xchat`, `xgroup`, `xdriver`, `xadmin`, `xgit`). JWT HMAC é rejeitado.

Comunicação de app no desktop: `https://xchat.corp.ihuull.com` (intranet). Login web: `https://xauth.ihuull.com`. Portal/enroll: `https://xvpn.ihuull.com` (`/` portal; `/admin` → xadmin). Console: `https://xadmin.corp.ihuull.com` (só VPN; Fases 35+). Loja: `https://marketplace.ihuull.com` — schema em [`marketplace.md`](./marketplace.md). Drive: `https://xdriver.corp.ihuull.com` (só VPN; `xdriver.ihuull.com` não serve). Marketing xgroup: `https://xgroup.ihuull.com` (landing + perfil `/:user` com JWE; sem WS). App: `https://xgroup.corp.ihuull.com`.

Auth no browser: cookie `ihuull_session` (`Domain=.ihuull.com`, Secure, HttpOnly, SameSite=Lax), emitido só em `xauth.ihuull.com`. Desktop: `Authorization: Bearer <token>` em memória, sem cookie. Nunca na query do WebSocket.

## Auth e sessão

| Método | Path | Auth | Notas |
|---|---|---|---|
| POST | `/api/auth/login` | público + rate limit | Body `{username,password,aud?}`. Recusa `xbot`. Em `xauth` também grava o cookie de sessão. |
| POST | `/api/auth/logout` | público | Apaga o cookie `.ihuull.com`. |
| GET | `/api/auth/me` | Bearer ou cookie | Papel atual do banco (não só o claim). `xgit_enabled` se `ProjectMember` ou ACL do app `xgit`. |
| POST | `/api/devices/enroll` | convite | Devolve IP, pubkey do server, `dns: ["10.66.66.1"]`. |
| GET | `/api/status` | público | Saúde, `api_version`, peers. |

## Identidade no túnel (só `10.66.66.1:8080`)

| Método | Path | Auth |
|---|---|---|
| GET | `/api/me` | IP do peer na subnet |
| POST | `/api/me/ssh-key` | IP do peer |

## Usuários e devices (RBAC)

CRUD em `/api/users`, convite, reset de senha, file-access, devices — `viewer+` lê, `admin+` escreve. Ver `PLAN.md` §6.7.

`User.products` (`core` \| `marketplace` \| `xgroup` \| `xdriver` \| `forge` \| `compute` \| `dns` \| `managed`): escopo de um `admin` (Fase 35+). Lista vazia = irrestrito. `super_admin` ignora. Escritas de produto exigem o id na lista (`PUT /marketplace/apps/:id/access` → `marketplace`; `DELETE /devices/:id` e waitlist/config → `core`; file-access → `xdriver`). IAM (users/roles) não é produto.

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
| POST | `/api/social/posts/:id/star` | Toggle estrela |
| GET/POST | `/api/social/posts/:id/comments` | Comentários (280) |
| POST | `/api/social/posts/:id/repost` | Toggle repost (não o próprio) |
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
| GET | `/api/driver/download` | `inline=1` para visualizar no browser |
| PUT | `/api/driver/write` | texto até 2 MiB (`root`, `path`, `content`) |
| POST | `/api/driver/extract` | zip / tar.gz; recusa `..`; pasta nova ao lado |
| DELETE | `/api/driver/rm` | path obrigatório |

Qualquer outro `Host` → 404. `xdriver.ihuull.com` não serve estas rotas.

## Forge git (`xgit.corp.ihuull.com` apenas)

Smart HTTP (não é `/api`). Fora da VPN o Nginx recusa. Sem `git://`.

| Método | Path | Auth | Notas |
|---|---|---|---|
| GET | `/:slug/info/refs?service=git-upload-pack` | Basic (usuário + JWE) ou Bearer | clone/fetch |
| POST | `/:slug/git-upload-pack` | idem | |
| GET | `/:slug/info/refs?service=git-receive-pack` | idem | push: developer+; branch protegida = maintainer+ |
| POST | `/:slug/git-receive-pack` | idem | |
| GET | `/api/xgit/settings` | sessão | defaults do forge + `clone_host` |
| PATCH | `/api/xgit/settings` | admin + `forge` | visibility/network padrão, `allow_member_create` |
| GET | `/api/projects` | sessão | `?scope=all` (viewer+) lista todos; `?scope=mine` só `ProjectMember`. `?cards=1` acrescenta language, last_commit, spark e stars. Default: all se viewer+, senão mine. Member + `scope=all` → 403 |
| GET | `/api/xgit/overview` | sessão | perfil, populares, heatmap (1 ano) e activity (commits, repos, MRs + comentários XCHAT) |
| GET | `/api/xgit/stars` | sessão | repositórios com estrela |
| POST | `/api/projects/:slug/star` | sessão + ACL | toggle da estrela |
| POST | `/api/xgit/repos` | admin + `forge`, ou `member` se a flag permitir | mesmo create de `/api/projects` |
| GET | `/api/projects/:slug/tree` | sessão + ACL | `?ref=&path=` |
| GET | `/api/projects/:slug/blob` | sessão + ACL | `?path=` obrigatório, `?ref=` |
| GET | `/api/projects/:slug/commits` | sessão + ACL | `?ref=&path=&n=` |
| GET | `/api/projects/:slug/git` | sessão | `clone_url`, `exists`, `protected_branches` |
| POST | `/api/projects/:slug/git` | admin + `forge` | cria o bare se faltar |
| PUT | `/api/projects/:slug/protected-branches` | admin + `forge` | substitui a lista |
| GET | `/api/projects/:slug/branches` | sessão + ACL do projeto | heads do bare |
| GET | `/api/projects/:slug/issues` | sessão + ACL | `?status=&q=&author=&assignee=&label=&mentioned=&milestone=&sort=` (`me` em author/assignee/mentioned). Resposta: `items`, `open_count`, `closed_count` |
| POST | `/api/projects/:slug/issues` | reporter+ ou `forge` | abre issue + thread XCHAT + post XGROUP. `milestone` = número |
| GET | `/api/projects/:slug/issues/:n` | sessão + ACL | |
| PATCH | `/api/projects/:slug/issues/:n` | autor, maintainer+ ou `forge` | título, corpo, labels, assignees, milestone, open/closed |
| GET | `/api/projects/:slug/labels` | sessão + ACL | labels distintas das issues |
| GET | `/api/projects/:slug/milestones` | sessão + ACL | `?status=open\|closed` |
| POST | `/api/projects/:slug/milestones` | reporter+ ou `forge` | `title`, `description`, `due_on` (YYYY-MM-DD) |
| PATCH | `/api/projects/:slug/milestones/:n` | autor, maintainer+ ou `forge` | título, due, open/closed |
| GET | `/api/projects/:slug/work-projects` | sessão + ACL | boards do repo. `?status=&q=` |
| POST | `/api/projects/:slug/work-projects` | reporter+ ou `forge` | `template`: `kanban`/`board`/`table`/`bug`/`roadmap` |
| GET | `/api/projects/:slug/work-projects/:n` | sessão + ACL | inclui `items` |
| PATCH | `/api/projects/:slug/work-projects/:n` | autor, maintainer+ ou `forge` | título, open/closed |
| GET | `/api/projects/:slug/work-projects/:n/items` | sessão + ACL | |
| POST | `/api/projects/:slug/work-projects/:n/items` | reporter+ | draft (`title`) ou `issue`/`mr` |
| PATCH | `/api/projects/:slug/work-projects/:n/items/:id` | reporter+ | `column`, `position`, `title` |
| DELETE | `/api/projects/:slug/work-projects/:n/items/:id` | autor do item ou maintainer+ | |
| GET | `/api/projects/:slug/merge-requests` | sessão + ACL | `?status=open\|merged\|closed` |
| POST | `/api/projects/:slug/merge-requests` | developer+ ou `forge` | abre MR + thread XCHAT + post XGROUP |
| GET | `/api/projects/:slug/merge-requests/:iid` | sessão + ACL | `can_merge`, `checks_block` |
| PATCH | `/api/projects/:slug/merge-requests/:iid` | autor ou maintainer+ | título/descrição se aberto |
| POST | `/api/projects/:slug/merge-requests/:iid/merge` | maintainer+ se target protegida | bloqueia se CI do PR falhou |
| POST | `/api/projects/:slug/merge-requests/:iid/close` | autor, maintainer+ ou `forge` | |
| GET | `/api/projects/:slug/merge-requests/:iid/commits` | sessão + ACL | `base..head` |
| GET | `/api/projects/:slug/merge-requests/:iid/diff` | sessão + ACL | unified, teto 1 MiB |
| GET | `/api/projects/:slug/merge-requests/:iid/reviews` | sessão + ACL | |
| POST | `/api/projects/:slug/merge-requests/:iid/reviews` | reporter+ | `approve` / `request_changes` / `comment` |
| GET | `/api/xcodespaces` | sessão | `?slug=` lista workspaces do user |
| POST | `/api/xcodespaces` | sessão + ACL do repo | `slug`, `branch`, `kind`=`quick`\|`remote`. Remote exige developer+ |
| GET | `/api/xcodespaces/:id` | dono ou `forge` | `kind`, `status`, `runtime_url` |
| POST | `/api/xcodespaces/:id/start` | developer+ | liga o container (teto 1 em execução) |
| POST | `/api/xcodespaces/:id/stop` | dono ou `forge` | para o container; volume fica |
| DELETE | `/api/xcodespaces/:id` | dono ou `forge` | apaga worktree ou volume+container |
| GET | `/api/xcodespaces/:id/tree` | dono | `?path=` |
| GET | `/api/xcodespaces/:id/blob` | dono | `?path=` teto 2 MiB |
| PUT | `/api/xcodespaces/:id/contents` | developer+ | grava arquivo no worktree |
| POST | `/api/xcodespaces/:id/commit` | developer+ | commit no worktree; branch protegida → nova branch + PR |
| PUT | `/api/projects/:slug/contents` | developer+ | commit no bare; branch protegida sem push → nova branch + PR |
| GET | `/api/projects/:slug/archive` | sessão + ACL | ZIP da ref (`?ref=`) |
| GET | `/api/projects/:slug/jobs` | sessão + ACL | lista CI. `?workflow=ci&mr=N`. Inclui `workflows` |
| GET | `/api/projects/:slug/jobs/:n` | sessão + ACL | run enriquecido (title, event, jobs, can_*) |
| GET | `/api/projects/:slug/jobs/:n/log` | sessão + ACL | texto |
| GET | `/api/projects/:slug/jobs/:n/artifact` | sessão + ACL | blob |
| POST | `/api/projects/:slug/jobs/:n/cancel` | maintainer+ ou `forge` | também em `awaiting_approval` |
| POST | `/api/projects/:slug/jobs/:n/approve` | maintainer+ ou `forge` | `awaiting_approval` → `pending` |
| POST | `/api/projects/:slug/jobs/:n/rerun` | maintainer+ ou `forge` | run terminal → novo `pending` |
| GET | `/api/projects/:slug/runners` | sessão + ACL | peers `role=runner` do projeto (sem token) |
| POST | `/api/servers/:id/runner-token` | admin + `compute` | token uma vez; só `role=runner` |
| GET | `/api/ci/jobs/next` | VPN + token do runner | 204 se vazio |
| POST | `/api/ci/jobs/:id/log` | VPN + token | |
| POST | `/api/ci/jobs/:id/finish` | VPN + token | |
| POST | `/api/ci/jobs/:id/artifact` | VPN + token | multipart `file` |
| GET | `/api/services` | viewer+ | lista; `?project=` |
| GET | `/api/services/:slug` | viewer+ | sem senha |
| POST | `/api/services` | admin + `managed` | senha uma vez; DNS `svc-<slug>.corp` se bind wg0 |
| POST | `/api/services/:slug/apply` | admin + `managed` | reaplica (local) ou marca pending (malha) |
| POST | `/api/services/:slug/stop` | admin + `managed` | |
| POST | `/api/services/:slug/rotate` | admin + `managed` | senha uma vez |
| DELETE | `/api/services/:slug` | admin + `managed` | para + apaga DNS |
| GET | `/api/projects/:slug/services` | sessão + ACL | sem senha |
| POST | `/api/servers/:id/agent-token` | admin + `compute` | token uma vez; `mesh` ou `runner` |
| GET | `/api/svc/desired` | VPN + token do agent | estado a aplicar no peer |
| POST | `/api/svc/:id/status` | VPN + token | `ready` / `error` / `stopped` |

Outro `Host` nas rotas smart HTTP → 404.

## Marketplace (`marketplace.ihuull.com`)

| Método | Path | Auth |
|---|---|---|
| GET | `/api/marketplace/apps` | sessão + ACL + `network` |
| GET | `/api/marketplace/assets/:id/download` | sessão + ACL + `network` |
| POST | `/api/marketplace/sync` | `XVPN_PUBLISH_TOKEN` |

`visibility` (quem, ACL) ≠ `network` (onde). App `network: vpn` não lista nem baixa em `marketplace.ihuull.com` sem túnel; aparece quando o peer está na VPN (`10.66.66.0/24`). Host `*.corp` sozinho não basta. `network: public` aparece na loja pública com JWE (nunca anônimo).

ACL admin: `PUT /api/marketplace/apps/:id/access` no xadmin (tela **ACL**, não a vitrine), só com escopo `marketplace` (ou admin irrestrito / `super_admin`). Catálogo e ACL são telas distintas (`PLAN.md` §6.8, [`marketplace.md`](./marketplace.md)).

## Backups externos (xadmin, Fase 44)

`viewer+` lê; escrita exige `admin+` e produto `core`. Secret do destino nunca volta no GET.

| Método | Path | Notas |
|---|---|---|
| GET/PATCH | `/api/backups/settings` | retenção, include mongo/marketplace/git/social |
| GET/POST | `/api/backups/destinations` | `kind`: `sftp` `b2` `s3` `webdav` `drive` `xdriver` |
| PATCH/DELETE | `/api/backups/destinations/:id` | PATCH pode rotacionar `secret` |
| GET | `/api/backups/jobs` | últimos 40 |
| POST | `/api/backups/destinations/:id/run` | `{dry_run}`. restic no PATH; credenciais só no VPS |

Restore: [`docs/runbooks/backup-restore.md`](./runbooks/backup-restore.md). `backup.sh` local permanece.

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
