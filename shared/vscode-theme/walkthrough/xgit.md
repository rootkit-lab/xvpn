# XGIT e intranet

| O quê | Onde |
|---|---|
| Repositório | `https://xgit.corp.ihuull.com/<slug>` |
| Settings / ENVs | `xgit.corp/:slug/settings` → **Codespaces** |
| Console | `xadmin.corp` |
| Catálogo | `xcodespaces.corp` |
| Este IDE | `cs-<id>.corp.ihuull.com` |

Tudo isso só resolve no DNS interno (`10.66.66.1`). Fora da VPN o host não abre.

No terminal do codespace (repo **`teste`**):

```bash
go test ./...
node web/index.mjs
./scripts/check.sh
```

ENVs plaintext do Settings aparecem no `env` do container. `XCS_LLM_*` o proxy lê no servidor — a key **não** vem para cá.

Chat ihuull e generate commit entram na extensão `ihuull.codespace` (Fase 51.2/51.4). Sem Copilot, sem Continue, sem `docker.sock`.
