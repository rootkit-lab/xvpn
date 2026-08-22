# TASKS — Redes overlay + cutover data + xmonitor

> Branch: `feat/data-cutover-xmonitor`
> PR: https://github.com/rootkit-lab/xvpn/pull/179
> Fase: **67**

## Objetivo

Organizar a VPN em **redes overlay** (infra ≠ usuários), com o xadmin criando redes e regras de participação. Só depois o VPS **data** entra na rede de infra e recebe git/Docker/Mongo. Fechar com **xmonitor**.

## Por que redes vêm antes do cutover

Hoje **tudo** é `10.66.66.0/24`: notebooks, malha, VIP `.254`, Samba, futuro Mongo no data. Qualquer peer alcança qualquer `/32`. Colocar Mongo/git no data **nessa** faixa expõe o banco a todo cliente.

Isolar primeiro. Migrar depois.

## Nós (não mudam de papel)

| Nó | Público | Overlay | Fica com |
|---|---|---|---|
| **control** (VPS principal) | `206.189.224.72` | `10.66.66.1` na rede **infra** | Hub WG (`51820`), Nginx (`*.corp` + públicos), `xvpn-server`, dnsmasq `:53`, Samba `:445`, landpages-ops, enroll |
| **data** | `66.29.147.100` | `/32` na rede **infra** (não na de usuários) | Git bare, Docker/codespaces/registry, Mongo do CP e gerenciados |

Um `wg0` no hub. Duas (ou N) **faixas**. Sem segundo listen público. Sem `10.10` / `10.136`.

## Plano de endereços (escrever em `PLAN.md` §5.3 nesta fase)

| Rede | Kind | CIDR | Quem | Apagável |
|---|---|---|---|---|
| `infra` | `infra` | `10.66.66.0/24` (já em produção) | control `.1`, data/runners, VIP `.254` | **não** |
| `users` | `users` | `10.66.80.0/24` (primeira do pool) | devices no enroll | **não** |
| *(criadas no xadmin)* | `custom` | fatias de `10.66.80.0/20` (`10.66.81.0/24`…) | times / projetos / labs | sim, se vazia |

- Pool de redes de usuário: **`10.66.80.0/20`**. Não alargar `10.66.66.0/24`. `10.66.67.0/24`–`10.66.79.0/24` ficam livres (PLAN já cogitou `.67`).
- Enroll **device** → IP em `users`. Enroll **mesh** (`data`) → IP em `infra`.
- Sem legado: seed **move** device de usuário em `10.66.66.x` para `users`. Cliente reconecta.
- Cliente (users): `AllowedIPs` = CIDR(s) das redes das quais é membro **+** destinos liberados por regra **+** `0.0.0.0/0` só se a rede home tiver `exit=true`.
- Malha (data): `AllowedIPs` = CIDR **infra** apenas (split). Internet pelo `wan0`.

## Modelo (xadmin → Redes)

Produto VPN (não Compute, não forge).

| Entidade | Campos essenciais |
|---|---|
| `OverlayNetwork` | slug, name, kind (`infra`/`users`/`custom`), cidr, `system`, `exit` (NAT no hub; só users por default), hub = control |
| `NetworkMember` | network + sujeito (`user` / `device` / `mesh_server`) + papel `member`/`operator` |
| `NetworkRule` | src_net → dst_net, action allow/deny, proto/portas opcionais, `system` |

Semântica:

- **Mesma rede:** membros falam entre si (L3). `infra↔infra` = control e data se veem; Mongo no data **não** precisa de regra extra para o `xvpn-server` no control.
- **Entre redes:** **nega** no FORWARD do hub, salvo `NetworkRule` allow.
- Participar de outra rede interna = **membro** nessa rede **ou** regra que alcance o CIDR dela. Os dois: membership dá IP/rota; regra dá atravessar sem ser membro.

Seed de regras (sistema, editáveis com audit):

| De | Para | Portas | Motivo |
|---|---|---|---|
| `users` (e customs com “acesso corp”) | `infra` | TCP `443`, UDP/TCP `53` | `*.corp` + dnsmasq. Sem isso o painel some |
| `users` | `infra` | TCP `445` | Samba atual — regra nomeada `samba`, dá para revogar |
| `*` | `infra` | TCP `27017` / `27018` | **não** no seed |

Internet (`exit`): só rede `users` com `exit=true`. `infra` e `custom` default `exit=false`.

UI: `/admin/networks` — criar custom (CIDR do pool ou /28 automático), membros, regras. Compute continua cadastrando **hosts**; a rede do peer mesh é `infra`.

Apply no hub: `AllocateIP` por `network_id`; reconciliar peers; gerar FORWARD (nft/ufw) a partir das regras. Sem SSH do control → data. Sem chave privada no painel.

## Inventário de migração (preencher no 67.2 com evidência do VPS)

| Carga | Hoje | Destino | Notas |
|---|---|---|---|
| Mongo CP (`:27017`) | **não em uso** — SQLite `/opt/xvpn/data/xvpn.db` no control; sem `XVPN_MONGO_URI` | data infra (quando ativar) | 67.4 **adiado**; `mongod` nunca rodou em prod |
| Git bare | NFS do **data** (`10.66.66.2:/opt/xvpn/data/git`); control monta em `/opt/xvpn/data/git` | data infra | cutover 2026-08-22; forge/auth no control |
| Docker / codespaces / registry | registry no **data** (`10.66.66.2:5000`); codespaces Docker ainda no control | data infra | registry via Nginx proxy; cs-apply pendente no data |
| Serviços `managed` | UI | `mesh_server_id` = data (`nc-ph-3726`), bind infra | mesh peer `10.66.66.2`, handshake OK |
| Samba / XDriver | control `10.66.66.1:445` (+ `127.0.0.1`) | **fica** no control | regra overlay `samba`; blobs não migram |
| landpages-ops | control `127.0.0.1:3002` | **fica** | não é XVPN |
| **data (malha)** | `66.29.147.100` / `wg0` `10.66.66.2/32` | — | git 5.1M + registry Docker; NFS export; ufw ativo; codespaces Docker pendente |
| **control (hub)** | `206.189.224.72` / `wg0` `10.66.66.1/24` + rota `10.66.80.0/20` | — | overlay nft + `xvpn-overlay-routes.service`; audit 2026-08-22 OK (SSH key-only, ufw, Samba só wg0) |

## Checklist (ordem obrigatória)

### 67.1 — Redes no xadmin + isolamento no hub

- [x] Modelos `OverlayNetwork` / `NetworkMember` / `NetworkRule` + seed `infra` (`10.66.66.0/24`) e `users` (`10.66.80.0/24`)
- [x] Validar CIDR: dentro do pool, sem overlap, sem `10.10`/`10.136`
- [x] `AllocateIP` do device → `users`; mesh enroll → `infra`
- [x] API + UI `/admin/networks`: CRUD custom, membros (user/device/server), regras
- [x] Reconciliar WG + FORWARD (default deny entre CIDRs; allow seed 443/53 + 445)
- [x] Enroll/GET config devolve `AllowedIPs` pela membership + regras (não mais “todo mundo é 10.66.66.0/24”)
- [x] Testes: user não alcança `:27017` no data; member+regra alcança a outra rede; mesh não ganha `0.0.0.0/0`
- [x] `PLAN.md` §5.3 (pool `10.66.80.0/20` + kinds) e §6.16 (peer data ∈ infra)
- [x] Deploy produção (#179+#180): `xvpn-server` + `xvpn-user-provision` (`overlay-apply`); nginx/Samba aceitam `10.66.80.0/20`; nft overlay ativo

### 67.2 — Data online **na infra**

- [x] Seed/`data` no Compute; enroll + bootstrap no `.100` (`nc-ph-3726`, pubkey `PA9YL8ON…`, handshake OK)
- [x] Peer `10.66.66.2/32` na infra; A `data.corp.ihuull.com` → `10.66.66.2` (`/etc/xvpn/dnsmasq-records.hosts`; resolve no laptop via VPN)
- [x] Baseline read-only — `vps-security-audit` no control (2026-08-22) + `ss`/`df` no data
- [x] Tabela de inventário preenchida (evidência acima)

### 67.3 — Migrar git + containers

- [x] Runbook [`docs/runbooks/data-node-cutover.md`](docs/runbooks/data-node-cutover.md) (git NFS + registry no data; codespaces Docker ainda no control)
- [x] Registry no data (`10.66.66.2:5000`); Nginx control → proxy; `xvpn-registry` local desligado
- [x] Git bare no disco do data; control monta NFS `10.66.66.2:/opt/xvpn/data/git` → `xvpn-server` inalterado
- [x] Smoke VPN: `xgit.corp` 200, `registry.corp/v2/` 401 (auth OK), `cs-b9d28a36a274.corp` 302 (ativo)
- [ ] Codespaces: `cs-apply` ainda no control (Docker local) — migrar containers no marco seguinte
- [ ] Apagar `git.bak-pre-data-*` no control após período de validação (registry local já removido)

### 67.4 — Migrar Mongo (e limpar control)

**Adiado** — produção usa **SQLite** (`/opt/xvpn/data/xvpn.db`); `mongod` inativo, sem `:27017`, sem `XVPN_MONGO_URI` no env (verificado 2026-08-22). Cutover Mongo → data só quando `XVPN_MONGO_URI` for ativado (runbook `docs/runbooks/mongodb.md`).

- [x] Verificado: nada a migrar nem parar no control
- [ ] *(futuro)* `mongod` no data (`bindIp` wg0/loopback); URI `10.66.66.2:27017`; overlay sem FORWARD users→27017
- [ ] *(futuro)* `xvpn-migrate-mongo` + `backup.sh` com `mongodump`

### 67.5 — xmonitor

- [x] `port-domain-registry-check` + `PLAN.md` §5.2: `xmonitor.corp.ihuull.com`
- [x] Seed DNS `xmonitor` → `10.66.66.1`; repo `xcorp/xmonitor`
- [x] App intranet (`aud=xmonitor`), API `/api/xmonitor/` no monólito
- [x] Checks v0: HTTP `*.corp`, handshake WG, Mongo no data (skipped se down), registry/git no data
- [x] UI dashboard (`xmonitor-page.tsx`); métricas de nó via `POST /api/xmonitor/report` (agent token)
- [x] Deploy produção + smoke `https://xmonitor.corp.ihuull.com/` na VPN (200, API dashboard OK)
- [ ] Cron no data para report de disco/load (opcional v0)
- [ ] Alertas Mongo + xbot (adiado — SQLite `monitor_checks`)

### 67.6 — Docs / skills

- [x] `PLAN.md` §6.16 (dois nós + redes) · `ROADMAP.md` Fase 67
- [x] `docs/areas/networks.md` (compute/xmonitor/runbook no 67.2+)
- [ ] Skills: `data-node-ops`; `new-intranet-app` lista xmonitor; `deploy-xvpn-server` não mexe no data
- [x] `AGENTS.md`: control vs data; redes infra/users; nunca Mongo/git na eth0
- [ ] Sem segredos no Git

## Fora de escopo

- Segundo listen WG / `wg1` / porta pública nova
- Full-mesh (spokes sem passar no hub)
- Mover Samba, blobs XDriver, landpages-ops
- Providers além de BitLaunch + register manual
- SSH control → data; chave privada no painel
- Nagios/Zabbix de terceiro
- IPv6 overlay

## Critério de saída

1. xadmin cria rede custom + regra; user sem membership/regra **não** alcança o CIDR da infra além de 443/53/445 do seed.
2. `data` é peer **infra**; git/Docker (67.3) e Mongo (67.4) só no bind da infra.
3. Control sem os daemons migrados; `xmonitor.corp` na VPN mostra os nós.
4. Docs/PLAN descrevem duas faixas e os dois nós, sem contradizer §5.

## Notas para o agente

- Ordem: **67.1 → 67.2 → 67.3 → 67.4 → 67.5 → 67.6**. Sem Mongo no data antes das redes e do peer estável.
- PRs pequenos (um marco). Atualizar este `TASKS.md` no mesmo PR.
- Skills: trabalho nesta branch → `ship-pr` → `land-pr` → `deploy-xvpn-server` se `server/`.
- Produção nos dois IPs — read-only primeiro.
