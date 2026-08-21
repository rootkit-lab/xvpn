# Área — Compute (malha)

Control-plane e nós de workload da VPN. **Não** é o forge (código mora no XGIT).

## Nós

| Host | IP público | Papel | Notas |
|---|---|---|---|
| `control` | `206.189.224.72` | `control` | `xvpn-server`, Nginx, WireGuard `10.66.66.1`, landpages-ops |
| `data` | `66.29.147.100` | `mesh` (manual) | Mongo, git bare, containers — alivia o control. Fase 66 |
| cripto-prod | `65.38.120.203` | `external` | Inventário só — sem enroll |

## Cadastro

- **BitLaunch:** Compute → Novo VPS (API + cloud-init).
- **Manual:** Compute → Cadastrar VPS existente → `POST /api/servers/register`. Devolve `enroll_token` + script bootstrap. Chave SSH **nunca** no servidor/API.
- Enroll: chave WireGuard gerada **no host**; só a pública em `POST /api/servers/enroll` (`xvpn.ihuull.com`).

## Invariantes

- Peer só em `10.66.66.0/24`. Sem `10.10` / `10.136`.
- Mongo/git/Docker no nó data: bind **só `wg0`** (ou loopback), nunca `0.0.0.0`/`eth0`.
- Hostnames `*.corp` dos produtos continuam em `10.66.66.1`; Nginx no control faz proxy ao peer quando o backend migrar.
- Token BitLaunch só no VPS (Compute → Configurações).

## UI / API

- `/admin/servers`, `/admin/compute/settings`
- Escopo produto: `compute`
- Docs API: [`../api.md`](../api.md)

## Relacionado

- Serviços gerenciados (`managed`) instalam software **em cima** de um `MeshServer` — não substituem Compute.
- Código da plataforma: XGIT `xcorp/xvpn` — ver [`xgit.md`](./xgit.md).
