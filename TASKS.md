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
| Mongo CP (`:27017`) | control loopback | data **infra** (`wg0`); URI no control → `10.66.66.<data>` | Último cutover; `mongodump`; users **não** alcançam |
| Git bare | control | data infra | `xgit.corp` → Nginx no control → path no data |
| Docker / codespaces / registry | control | data infra | `cs-*` / `registry.corp` via proxy |
| Serviços `managed` | UI | `mesh_server_id` = data, bind infra | ≠ Mongo do CP |
| Samba / XDriver | control | **fica** no control | regra `samba`, não mover blobs |
| landpages-ops | control | **fica** | não é XVPN |

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

- [ ] Seed/`data` no Compute; Gerar enroll + bootstrap no `.100` (SSH do **laptop** — agente não alcança `66.29.147.100:22`)
- [ ] Peer em `10.66.66.0/24`; A `data.corp` → IP wg0 do data
- [ ] Baseline read-only (`ss`, `ufw`, disco) — `vps-security-audit` no control
- [ ] Preencher a tabela de inventário

### 67.3 — Migrar git + containers

- [ ] Runbook git (rsync/mirror; Nginx proxy; bind só wg0/loopback no data)
- [ ] Runbook Docker/registry/codespaces no data
- [ ] Smoke `xgit.corp` + um codespace
- [ ] Apagar bare/Docker no control **só depois** de validar

### 67.4 — Migrar Mongo (e limpar control)

- [ ] Dump + restore no data; `XVPN_MONGO_URI` → `10.66.66.<data>:27017` (auth)
- [ ] 27017 **não** em `0.0.0.0`/`eth0`; users **não** FORWARD até essa porta
- [ ] Parar Mongo (e daemons migrados) no control; documentar o que sobrou no `.72`
- [ ] Backup (`backup.sh`) no caminho novo

### 67.5 — xmonitor

- [ ] `port-domain-registry-check` + `PLAN.md` §5.2: `xmonitor.corp.ihuull.com`
- [ ] Seed DNS `xmonitor` → `10.66.66.1`; repo `xcorp/xmonitor`
- [ ] App intranet (`aud=xmonitor`), API `/api/xmonitor/` no monólito
- [ ] Checks v0: HTTP `*.corp`, handshake WG, Mongo no data (probe **da infra**, não de user net), disco/load via agent/token — sem SSH do control
- [ ] Alertas no Mongo + UI; xbot opcional depois

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
