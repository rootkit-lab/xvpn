# TASKS — Migração single-node (`66.29.147.100`)

> Branch: `feat/mydevices-self-invite`
> Fase: **68**
> Runbook: [`docs/runbooks/single-node-migration.md`](docs/runbooks/single-node-migration.md)

## Objetivo

**Um único VPS** em produção: `66.29.147.100` (BitLaunch). O `206.189.224.72` (DigitalOcean) será cancelado após validação. Cloudflare (API / xadmin DNS) reponta todos os A públicos para o IP novo.

## Por que agora

Fase 67 moveu git/registry para o data, mas o hub (WG, Nginx, `xvpn-server`, dnsmasq, Samba) ainda está no control. Manter dois VPS sem necessidade aumenta custo e complexidade (NFS, proxy registry, dois firewalls).

## Inventário (control → data)

| Carga | Tamanho aprox. | Já no `.100`? |
|---|---|---|
| `xvpn.db` + config `/etc/xvpn` | ~1 MiB | não |
| marketplace blobs | ~247 MiB | não |
| codespaces worktrees | ~19 MiB | parcial |
| social, packages, pages | ~200 KiB | não |
| git bare | ~5 MiB | **sim** (disco local) |
| registry | ~4 KiB | **sim** (Docker) |
| Nginx + certs Let's Encrypt | — | não |
| WG hub key + peers | — | não (hoje é peer) |
| Samba + unix accounts | — | não |
| dnsmasq zona corp | — | não |
| landpages-ops | — | não (decidir) |

## Checklist

### 68.1 — Preparar host (sem cutover)

- [x] Pacotes: nginx, certbot+dns-cloudflare, dnsmasq, samba, docker, wireguard
- [x] Rsync `/opt/xvpn/data`, binários, `/opt/xvpn/xvpn-server.env`, letsencrypt, samba, dnsmasq do control
- [x] Registry `127.0.0.1:5000` + `corp.conf` local
- [x] Nginx sites (sem landpages); listen `66.29.147.100` nos públicos
- [x] `nginx -t` OK + ufw alvo (22/80/443/51820)
- [x] Smoke local (`127.0.0.1:8080/api/status`) — cutover 2026-08-23

### 68.2 — Cloudflare

- [ ] TTL 300 s nas zonas `ihuull.com` / `ihuu.com`
- [x] PATCH todos os A `206.189.224.72` → `66.29.147.100` (DNS only) — 9 registros, 2026-08-23
- [x] Verificar: sem A `*.corp`; TXT `corp` intacto
- [x] `dig +short xvpn.ihuull.com @1.1.1.1` → `66.29.147.100`

### 68.3 — Cutover (janela)

- [x] Control `.72` inacessível (já fora do ar)
- [x] Hub WG novo em `.100` (`10.66.66.1/24`, `:51820`)
- [x] `xvpn-server` + sudoers + nginx 80/443 + dnsmasq/samba só em `10.66.66.1`
- [x] Smoke no IP direto (`66.29.147.100`) e `*.corp` local (xadmin/xgit OK)
- [x] Cloudflare DNS → `.100`
- [x] Smoke público: `https://xvpn.ihuull.com/api/status`
- [x] Exit VPN (internet): rota `10.66.80.0/20 dev wg0` + NAT `iifname wg0` (runbook § exit)
- [ ] Clientes: **reenroll** — endpoint `66.29.147.100:51820` + nova pubkey hub

### 68.4 — Limpeza

- [ ] 7–14 dias estável → cancelar `.72`
- [ ] Atualizar `AGENTS.md`, `PLAN.md`, runbooks Cloudflare
- [ ] xmonitor probes (registry local, sem NFS)
- [ ] MeshServer `data`: IP público único

### 68.5 — landpages-ops

- [x] **Fora do single-node** — `ldpops.appapisip.com` permanece no `.72` até rehospedar ou cancelar separadamente
- [ ] Ao cancelar `.72`, `landpages-ops` deixa de funcionar (aceito)

## Critério de saída

1. Só `66.29.147.100` responde em 80/443/51820.
2. VPN, intranet `*.corp` e domínios públicos funcionam com DNS Cloudflare no IP novo.
3. Sem NFS entre nós; git/registry locais.
4. Docs refletem host único.

## Notas para o agente

- Ordem: **68.1 prep → 68.2 DNS → 68.3 cutover → 68.4 docs**. DNS pode ser testado antes de ligar WG público.
- Produção nos dois IPs até o cutover — read-only primeiro.
- Não cancelar `.72` no mesmo dia do cutover.
