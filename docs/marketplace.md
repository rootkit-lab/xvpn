# Marketplace e projetos

Fonte de arquitetura: [`PLAN.md` §6.8, §6.10, §6.15](../PLAN.md). API: [`docs/api.md`](./api.md). Console: `xadmin.corp.ihuull.com` (Fase 35+). Loja: `https://marketplace.ihuull.com`.

O marketplace **não** é só uma loja de `.deb`. Cada `slug` é (ou vira) um **projeto** no xadmin. Esta página documenta o manifesto, as duas dimensões de acesso, o sync e o que a vitrine pública mostra.

## Catálogo ≠ ACL

| Superfície | Quem escreve | O quê |
|---|---|---|
| `apps/<pasta>/marketplace.yaml` + `POST /api/marketplace/sync` | CI / `XVPN_PUBLISH_TOKEN` | Metadados, `kind`, `visibility`, `network`, assets |
| `PUT /api/marketplace/apps/:id/access` | admin com escopo `marketplace` | Lista de users de um app `restricted` |
| Loja `marketplace.ihuull.com` | ninguém (leitura) | Vitrine filtrada por sessão + ACL + `network` + `kind` |

Publicar pelo painel foi removido na Fase 16. Esconder o botão não basta — a API de CRUD manual não existe. A ACL **não** vai no Git: entra e sai gente num ritmo que não é PR.

Telas no xadmin (Fase 36): **Catálogo** (origem, versões, kind) e **ACL** (quem acessa `restricted`).

## Manifesto `marketplace.yaml`

Validado no CI por [`.github/scripts/validate-marketplace-manifests.py`](../.github/scripts/validate-marketplace-manifests.py). Pasta em `apps/` **sem** este arquivo é ignorada pelo sync.

| Campo | Obrigatório | Valores |
|---|---|---|
| `slug` | sim | `[a-z0-9-]{2,20}` — identidade no DB e no projeto. A pasta pode diferir (`apps/xvpn-chat` → `xchat`) |
| `name` | sim | Vitrine = `productDisplayName(slug)` quando o slug é produto conhecido |
| `description` | sim | string |
| `visibility` | sim | `global` \| `restricted` — **quem** |
| `network` | sim | `public` \| `vpn` — **onde** |
| `kind` | sim (Fase 36) | ver abaixo |
| `source` | sim | `build` \| `external` |
| `channel` | sim | `stable` \| `beta` |
| `assets[]` | sim | `platform`, `arch`, `url` (+ `sha256` se `external`) |
| `icon_url`, `source_path`, `version`, `changelog`, `shared_ui` | não | `shared_ui: true` = UI obrigada a `@xvpn/ui` |

`visibility` ≠ `network`. App `restricted` + `network: public` aparece na loja só para quem está em `AppAccess`. App `global` + `network: vpn` some da loja sem túnel.

### `kind`

| kind | O que é | Loja pública? |
|---|---|---|
| `desktop` | Wails / instalador (hoje `xvpn-client`, `xchat`) | se `network: public` |
| `web` | portal/SPA | se `network: public` |
| `service` | API no monólito ou processo na malha | não |
| `library` | pacote compartilhado (`shared/ui` é o primeiro) | não |
| `infra` | unit/nginx/dnsmasq | não |
| `docs` | runbook/wiki no XDRIVER | não |
| `container` | imagem (registry na malha, Fase 45+) | não |

Qualquer projeto interno cabe num `kind`. **Não** há lista de pastas do laptop no PLAN — o projeto nasce no xadmin quando alguém o criar.

Projeto **sem** `marketplace.yaml` (só metadado) não entra no sync da loja. `apps/` continua fonte da verdade **só** para artefatos distribuíveis.

### `source`

| `source` | Versão / SHA | Uso |
|---|---|---|
| `build` | CI, a partir da GitHub Release (`${VERSION}`) | Este monorepo |
| `external` | `url` + `sha256` no YAML | Binário de terceiro, sem commitar o arquivo |

`source: build` sem release ainda: o sync **pula** o app com aviso, não falha o job.

## Sync

`POST /api/marketplace/sync` — corpo = lista **completa** de manifestos (full sync). Token `XVPN_PUBLISH_TOKEN` (tempo constante) ou JWE `super_admin`. Sem a variável, a rota **não é registrada**.

- Upsert por `slug`. Segunda execução sem mudança = noop.
- Slug que sumiu → `ArchivedAt`, nunca hard-delete.
- Fetch de asset: só `https`; anti-SSRF no IP resolvido (sem loopback/privado).
- Storage content-addressed em `/opt/xvpn/data/marketplace/blobs/<sha256>`. Máx. 2 GiB/arquivo.

Workflows: `marketplace-sync.yml`, `release-client.yml`, `release-chat.yml`. Tag do release-please **não** dispara `on.push.tags` — use `workflow_dispatch`. Skill: `marketplace-publish`.

## Auth da loja

Download **nunca** anônimo. JWE (cookie `.ihuull.com` ou Bearer). `network: vpn` exige peer na `10.66.66.0/24`. Host `*.corp` sozinho não basta.

## Relação com o forge

A partir da Fase 37 o mesmo `slug` é o projeto: membros, regras, issues (XGROUP), review (XCHAT), arquivos (XDRIVER), git (`xgit.corp`, Fase 40). Releases continuam sendo `AppVersion` / `AppAsset`. Ver `PLAN.md` §6.15.
