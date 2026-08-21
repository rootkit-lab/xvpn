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
| GET | `/:org/:slug/info/refs?service=git-upload-pack` | Basic (usuário + JWE) ou Bearer | clone/fetch. Path plano `/<slug>` não existe. |
| POST | `/:org/:slug/git-upload-pack` | idem | |
| GET | `/:org/:slug/info/refs?service=git-receive-pack` | idem | push: developer+; branch protegida = maintainer+ |
| POST | `/:org/:slug/git-receive-pack` | idem | |
| GET | `/api/xgit/settings` | sessão | defaults do forge + `clone_host` |
| PATCH | `/api/xgit/settings` | admin + `forge` | visibility/network padrão, `allow_member_create` |
| GET | `/api/orgs/:org` | sessão | home da org: times, repos visíveis (ACL), templates abertos no time `workflows` |
| GET | `/api/orgs/:org/teams/:team/members` | viewer+, OrgMember ou membro do time/pai | 404 se o caller não vê o time |
| POST | `/api/orgs/:org/teams/:team/members` | owner/admin da org ou admin+ | `{user_id}` — `viewer` é só leitura |
| DELETE | `/api/orgs/:org/teams/:team/members/:userID` | idem | remove do time |
| GET | `/api/projects` | sessão | `?scope=all` (viewer+) lista todos; `?scope=mine` = ProjectMember ∪ OrgMember ∪ OrgTeamMember. `?org=&team=` filtra (`team=root` = sem time). `?cards=1` acrescenta language, last_commit, spark e stars. Member + `scope=all` → 403 |
| GET | `/api/xgit/overview` | sessão | perfil, populares, heatmap (1 ano) e activity (commits, repos, MRs + comentários XCHAT) |
| GET | `/api/xgit/stars` | sessão | repositórios com estrela |
| GET | `/api/ci/workflow-templates` | sessão | galeria New workflow (`?category=publish&q=`). Categorias + cards. Publish: npm/pypi/generic/maven/nuget/gem |
| POST | `/api/projects/:org/:slug/workflows` | developer+ ou `forge` | `{template_id}` grava `.xvpn-ci.sh`. Placeholders `{{REPO}}` viram `<org>/<slug>`. Sem interpolar JWE |
| GET | `/api/xgit/packages` | sessão | packages visíveis (ACL do projeto). `project_slug` = `<org>/<slug>`. Exemplos `xcorp/hello-*` no boot (45.3) |
| GET | `/api/projects/:org/:slug/packages` | sessão + ACL | lista + `can_publish` |
| POST | `/api/projects/:org/:slug/packages` | developer+ ou `forge` | multipart `name`, `version`, `kind` (`generic`/`npm`/`pypi`/`maven`/`nuget`/`rubygems`), `file` (≤64 MiB) |
| GET | `/api/projects/:org/:slug/packages/:id/download` | sessão + ACL | blob da versão |
| PUT | `/api/packages/:org/:slug/npm/*pkg` | developer+ ou `forge` | `npm publish` (manifest + `_attachments` base64). Registry: `https://xgit.corp.ihuull.com/api/packages/:org/:slug/npm/` |
| GET | `/api/packages/:org/:slug/npm/*pkg` | sessão + ACL | packument npm (`versions`, `dist-tags.latest`, `dist.tarball`) |
| POST | `/api/packages/:org/:slug/pypi` | developer+ ou `forge` | twine (`name`, `version`, `content`). Nome PEP 503 |
| GET | `/api/packages/:org/:slug/pypi/simple/` | sessão + ACL | índice PEP 503 (HTML) ou PEP 691 (`Accept: application/vnd.pypi.simple.v1+json`) |
| GET | `/api/packages/:org/:slug/pypi/simple/:name/` | sessão + ACL | ficheiros + `#sha256=`. Host só `xgit.corp` / `xadmin.corp` |
| PUT | `/api/packages/:org/:slug/maven/*` | developer+ ou `forge` | `mvn deploy` (layout Maven2). SNAPSHOT ok. Checksums `.md5`/`.sha1` gerados no GET |
| GET | `/api/packages/:org/:slug/maven/*` | sessão + ACL | artefacto, `maven-metadata.xml` e hashes |
| GET | `/api/packages/:org/:slug/nuget/index.json` | sessão + ACL | service index NuGet v3 |
| PUT/POST | `/api/packages/:org/:slug/nuget` | developer+ ou `forge` | `dotnet nuget push` (multipart `package`/`file` ou body raw) |
| GET | `/api/packages/:org/:slug/nuget/flat/:name/index.json` | sessão + ACL | `{versions}` |
| GET | `/api/packages/:org/:slug/nuget/flat/:name/:version/:file` | sessão + ACL | download `.nupkg` |
| POST | `/api/packages/:org/:slug/rubygems` | developer+ ou `forge` | `gem push` (também `/rubygems/api/v1/gems`) |
| GET | `/api/packages/:org/:slug/rubygems/gems/:filename` | sessão + ACL | download `.gem` |
| GET | `/api/registry/token` | sessão + host `registry.corp` | Docker token (`?scope=repository:<org>/<slug>:pull,push`). JWE amarrado ao repo |
| GET | `/api/registry/auth` | sessão ou token de packages | `auth_request` Nginx (`X-Original-URI` `/v2/<org>/<slug>/…`). GET=pull; PUT=push (developer+) |
| GET | `/api/projects/:org/:slug/wiki` | sessão + ACL | lista páginas (`refs/xgit/wiki`) |
| GET/PUT | `/api/projects/:org/:slug/wiki/:page` | GET=ACL; PUT=developer+ | Home.md = `#1`. Preview GFM no XGIT |
| GET | `/api/projects/:org/:slug/pages` | sessão + ACL | `{url, published}` |
| POST | `/api/projects/:org/:slug/pages` | developer+ ou JWE `aud=packages` | `source=docs\|public` ou multipart `file` (tar.gz/zip). Disco `/opt/xvpn/data/pages` |
| GET | `https://pages.corp.ihuull.com/:org/:slug/` | só VPN | `index.html` estático. Sem A público |
| GET | `/api/projects/:org/:slug/security` | sessão + ACL | findings (`SecAlert`) + policy `SECURITY.md` + empty states |
| POST | `/api/projects/:org/:slug/security/report` | reporter+ | issue `restricted` (só maintainer + autor) |
| GET | `/api/projects/:org/:slug/agents` | sessão + ACL | codespaces do repo; `filter=mine|attention|active|completed`. Maintainer vê todos |
| POST | `/api/projects/:org/:slug/star` | sessão + ACL | toggle da estrela |
| POST | `/api/xgit/repos` | admin + `forge`, ou `member` se a flag permitir | `{org,slug,…}` — org obrigatória (`xcorp`). Sem path plano. |
| POST | `/api/projects` | idem | `{org,slug}` obrigatórios |
| GET | `/api/projects/:org/:slug/tree` | sessão + ACL | `?ref=&path=` |
| GET | `/api/projects/:org/:slug/blob` | sessão + ACL | `?path=` obrigatório, `?ref=` |
| GET | `/api/projects/:org/:slug/commits` | sessão + ACL | `?ref=&path=&n=` |
| GET | `/api/projects/:org/:slug/git` | sessão | `clone_url`, `exists`, `protected_branches` |
| POST | `/api/projects/:org/:slug/git` | admin + `forge` | cria o bare se faltar |
| PUT | `/api/projects/:org/:slug/protected-branches` | admin + `forge` | substitui a lista |
| GET | `/api/projects/:org/:slug/branches` | sessão + ACL do projeto | heads do bare |
| GET | `/api/projects/:org/:slug/issues` | sessão + ACL | `?status=&q=&author=&assignee=&label=&mentioned=&milestone=&sort=` (`me` em author/assignee/mentioned). Resposta: `items`, `open_count`, `closed_count` |
| POST | `/api/projects/:org/:slug/issues` | reporter+ ou `forge` | abre issue + thread XCHAT + post XGROUP. `milestone` = número |
| GET | `/api/projects/:org/:slug/issues/:n` | sessão + ACL | |
| PATCH | `/api/projects/:org/:slug/issues/:n` | autor, maintainer+ ou `forge` | título, corpo, labels, assignees, milestone, open/closed |
| GET | `/api/projects/:org/:slug/labels` | sessão + ACL | labels distintas das issues |
| GET | `/api/projects/:org/:slug/milestones` | sessão + ACL | `?status=open\|closed` |
| POST | `/api/projects/:org/:slug/milestones` | reporter+ ou `forge` | `title`, `description`, `due_on` (YYYY-MM-DD) |
| PATCH | `/api/projects/:org/:slug/milestones/:n` | autor, maintainer+ ou `forge` | título, due, open/closed |
| GET | `/api/projects/:org/:slug/work-projects` | sessão + ACL | boards do repo. `?status=&q=` |
| POST | `/api/projects/:org/:slug/work-projects` | reporter+ ou `forge` | `template`: `kanban`/`board`/`table`/`bug`/`roadmap` |
| GET | `/api/projects/:org/:slug/work-projects/:n` | sessão + ACL | inclui `items` |
| PATCH | `/api/projects/:org/:slug/work-projects/:n` | autor, maintainer+ ou `forge` | título, open/closed |
| GET | `/api/projects/:org/:slug/work-projects/:n/items` | sessão + ACL | |
| POST | `/api/projects/:org/:slug/work-projects/:n/items` | reporter+ | draft (`title`) ou `issue`/`mr` |
| PATCH | `/api/projects/:org/:slug/work-projects/:n/items/:id` | reporter+ | `column`, `position`, `title` |
| DELETE | `/api/projects/:org/:slug/work-projects/:n/items/:id` | autor do item ou maintainer+ | |
| GET | `/api/projects/:org/:slug/merge-requests` | sessão + ACL | `?status=open\|merged\|closed` |
| POST | `/api/projects/:org/:slug/merge-requests` | developer+ ou `forge` | abre MR + thread XCHAT + post XGROUP |
| GET | `/api/projects/:org/:slug/merge-requests/:iid` | sessão + ACL | `can_merge`, `checks_block` |
| PATCH | `/api/projects/:org/:slug/merge-requests/:iid` | autor ou maintainer+ | título/descrição se aberto |
| POST | `/api/projects/:org/:slug/merge-requests/:iid/merge` | maintainer+ se target protegida | bloqueia se CI do PR falhou |
| POST | `/api/projects/:org/:slug/merge-requests/:iid/close` | autor, maintainer+ ou `forge` | |
| GET | `/api/projects/:org/:slug/merge-requests/:iid/commits` | sessão + ACL | `base..head` |
| GET | `/api/projects/:org/:slug/merge-requests/:iid/diff` | sessão + ACL | unified, teto 1 MiB |
| GET | `/api/projects/:org/:slug/merge-requests/:iid/reviews` | sessão + ACL | |
| POST | `/api/projects/:org/:slug/merge-requests/:iid/reviews` | reporter+ | `approve` / `request_changes` / `comment` |
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
| PUT | `/api/projects/:org/:slug/contents` | developer+ | commit no bare; branch protegida sem push → nova branch + PR |
| GET | `/api/projects/:org/:slug/archive` | sessão + ACL | ZIP da ref (`?ref=`) |
| GET | `/api/projects/:org/:slug/jobs` | sessão + ACL | lista CI. `?workflow=ci&mr=N`. Inclui `workflows` |
| GET | `/api/projects/:org/:slug/jobs/:n` | sessão + ACL | run enriquecido (title, event, jobs, can_*) |
| GET | `/api/projects/:org/:slug/jobs/:n/log` | sessão + ACL | texto |
| GET | `/api/projects/:org/:slug/jobs/:n/artifact` | sessão + ACL | blob |
| POST | `/api/projects/:org/:slug/jobs/:n/cancel` | maintainer+ ou `forge` | também em `awaiting_approval` |
| POST | `/api/projects/:org/:slug/jobs/:n/approve` | maintainer+ ou `forge` | `awaiting_approval` → `pending` |
| POST | `/api/projects/:org/:slug/jobs/:n/rerun` | maintainer+ ou `forge` | run terminal → novo `pending` |
| GET | `/api/projects/:org/:slug/runners` | sessão + ACL | peers `role=runner` do projeto (sem token) |
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
| GET | `/api/projects/:org/:slug/services` | sessão + ACL | sem senha |
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

## Compute (malha)

`viewer+` lê; escrita exige `admin+` e produto `compute`. Ver [`areas/compute.md`](./areas/compute.md).

| Método | Path | Notas |
|---|---|---|
| GET | `/api/servers` | lista + contas BitLaunch (token hint) |
| POST | `/api/servers/import` | upsert control-plane + sync BitLaunch |
| POST | `/api/servers/register` | VPS já existente (manual). Body: `hostname`, `ipv4`, `role?`, `notes?`. **Rejeita** `ssh_private_key` / `private_key`. Resposta inclui `enroll_token` + `bootstrap` uma vez. Seed: nó `data` (`66.29.147.100`) |
| POST | `/api/servers` | cria no BitLaunch + cloud-init |
| POST | `/api/servers/enroll` | público + rate limit; só pubkey WG |
| GET/PATCH/DELETE | `/api/servers/:id` | destroy não chama BitLaunch se `bitlaunch_id` for `manual-*` / `local-*`; nó `data` é protegido |

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

## Packages Maven / NuGet / RubyGems (Fase 59)

Host só `xgit.corp` / `xadmin.corp` (`RequirePackagesHost`). Auth = Bearer JWE ou Basic (usuário + senha = JWE). O runner recebe `packages_token` no claim e exporta `XVPN_PACKAGES_TOKEN` — o `.xvpn-ci.sh` **não** interpola o token.

Maven (`settings.xml` + `pom.xml`):

```xml
<server><id>xgit</id><username>xgit</username><password>${env.XVPN_PACKAGES_TOKEN}</password></server>
```

```xml
<distributionManagement>
  <repository>
    <id>xgit</id>
    <url>https://xgit.corp.ihuull.com/api/packages/xcorp/hello-mvn/maven</url>
  </repository>
</distributionManagement>
```

`mvn deploy` faz PUT no layout Maven2 (`…/com/ihuull/hello-mvn/0.1.0/hello-mvn-0.1.0.jar`). SNAPSHOT (`0.1.0-SNAPSHOT`) é aceite. `maven-metadata.xml` e `.sha1`/`.md5` são gerados no GET.

NuGet: `dotnet nuget push *.nupkg --source https://xgit.corp.ihuull.com/api/packages/<org>/<slug>/nuget/index.json --api-key "$XVPN_PACKAGES_TOKEN"`.

RubyGems: `GEM_HOST_API_KEY="$XVPN_PACKAGES_TOKEN" gem push --host https://xgit.corp.ihuull.com/api/packages/<org>/<slug>/rubygems *.gem`.

## Superfícies que **não** são esta API

- Samba `10.66.66.1:445`. XDriver web só em `xdriver.corp` (mesmo `xvpn-server`). `xdriver.ihuull.com` não serve o Drive. Sem FileBrowser.
- `ldpops.appapisip.com` — outro processo.
