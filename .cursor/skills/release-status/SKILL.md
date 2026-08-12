---
name: release-status
description: Consulta as PRs de release mantidas pelo release-please (uma por componente — server, client, shared) e mostra o changelog pendente de cada uma. Use quando o usuário perguntar "o que está pendente de release", "qual a próxima versão do server/client", ou antes de decidir mergear uma PR de release.
---

# Status de releases (release-status)

O [release-please](https://github.com/googleapis/release-please) (ver `PLAN.md` §13) mantém automaticamente uma Pull Request de release por componente (`server`, `client`, `shared`), sempre atualizada com o changelog acumulado desde a última versão publicada, com o label `autorelease: pending`. Mergear essa PR é o que efetivamente corta a tag/release do componente.

> **Nota**: até o workflow `.github/workflows/release-please.yml` ser criado (Fase 2 do `ROADMAP.md`, quando `server/` existir), este comando não vai encontrar nenhuma PR — isso é esperado, não um erro.

## Uso

```bash
.cursor/skills/release-status/scripts/release-status.sh
```

## O que o script faz

1. Lista todas as PRs abertas com o label `autorelease: pending` via `gh pr list --search "label:autorelease: pending" --state open`.
2. Para cada uma, mostra título, branch, e um trecho do corpo (que contém o changelog gerado automaticamente pelo release-please).
3. Se nenhuma PR de release estiver aberta, informa que não há releases pendentes (todos os componentes estão na última versão publicada, ou o workflow ainda não foi criado).

## Depois de consultar

- Revisar o changelog gerado antes de mergear — o release-please infere `feat`/`fix`/`!` a partir dos títulos de PR squash-mergeados (Conventional Commits), mas vale conferir se algum commit foi classificado errado.
- Mergear a PR de release (squash, como todas as outras) publica a tag `componente-vX.Y.Z`, cria a GitHub Release e atualiza o `CHANGELOG.md` do componente.
