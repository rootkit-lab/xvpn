# Área — XGIT (forge)

Repos e CI da intranet. Host app: `xgit.corp.ihuull.com`. Console (todos os repos): `xadmin.corp` → XGIT.

## O que é / o que não é

| É | Não é |
|---|---|
| Código, PRs, Issues, Actions, Packages | Cadastro de VPS / inventário de IP |
| ACL org/time/`ProjectMember` | Papel IAM de plataforma (`User.Role`) |
| Clone só na VPN | GitLab CE / A público |

## Repos de produto (org `xcorp`)

| Slug | Conteúdo |
|---|---|
| `xvpn` | Monorepo da plataforma (`server/`, `shared/`, painel xadmin, API codespace). Seed Fase 66 |
| `xvpn-client` | Cliente desktop VPN (`apps/xvpn-client`) |
| `xchat` | Messenger desktop (`apps/xvpn-chat`) |
| `hello-*` / `teste` | Exemplos / canário |

xadmin, xgit (app), xcodespaces, marketplace, xgroup, xdriver são **produtos/hosts** — não exigem um `.git` cada um. O código do control-plane vive em `xcorp/xvpn`.

## Invariantes

- Path canónico `<org>/<slug>`. Sem path plano.
- Bare em `/opt/xvpn/data/git/<org>/<slug>.git` (cutover futuro pode morar no nó `data`).
- Escopo produto no xadmin: `forge`.

## Docs

- `PLAN.md` §6.15 · [`../api.md`](../api.md) · skill `new-intranet-app` para app novo.
