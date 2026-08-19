---
name: land-pr
description: Espera o CI, resolve o bloqueio de merge e faz squash-merge da PR na main. Use quando o usuário disser continuar, mergear, land, ou depois de ship-pr — nunca reinvente gh pr view com statusCheckRollup (payload enorme, trava o agente).
---

# Land PR (squash na main)

Fecha o ciclo `start-task` → `ship-pr`. A `main` exige squash, conversas resolvidas e *não* aceita `--admin` sem o usuário pedir.

## Uso

```bash
.cursor/skills/land-pr/scripts/land-pr.sh [número]
```

Sem número, usa o PR da branch atual (`gh pr view --json number`).

## O que o script faz

1. Estado compacto: `mergeable` + `mergeStateStatus` + URL — **sem** `statusCheckRollup`.
2. Lista threads de review não resolvidas (a proteção `required_conversation_resolution` bloqueia o merge mesmo com CI verde).
3. `gh pr checks N --watch` até terminar.
4. Squash-merge `--delete-branch`.
5. `git checkout main && git pull --ff-only` e apaga a branch local (`-D`).

## Se o merge falhar com "base branch policy prohibits"

1. `mergeStateStatus: BLOCKED` + CI verde → quase sempre thread não resolvida. Corrija o finding ou resolva o thread se o commit já cobriu; rode o script de novo.
2. Security Reviewer `pending`/`in_progress` → espere. Não use `--admin`.
3. Não chame `gh pr view --json statusCheckRollup` (estoura contexto / timeout). Use `gh pr checks N`.

## Depois do merge (painel / servidor)

Se a PR alterou `server/` ou `server/web/`, rode a skill `deploy-xvpn-server`.
Se a PR for `chore(main): release xvpn-chat|xvpn-client|xvpn-server`, rode `marketplace-publish` / `release-status` — o catálogo **não** atualiza só com merge de feat.
