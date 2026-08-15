---
name: release-status
description: Consulta as PRs de release mantidas pelo release-please (uma por componente — server, client, chat) e mostra o changelog pendente de cada uma. Use quando o usuário perguntar "o que está pendente de release", "qual a próxima versão do server/client/chat", "por que o chat não aparece no marketplace", ou antes de decidir mergear uma PR de release.
---

# Status de releases (release-status)

O [release-please](https://github.com/googleapis/release-please) (ver `PLAN.md` §13) mantém automaticamente uma Pull Request de release por componente (`server`, `xvpn-client`, `xvpn-chat`), sempre atualizada com o changelog acumulado desde a última versão publicada, com o label `autorelease: pending`. Mergear essa PR é o que efetivamente corta a tag/release do componente.

O catálogo (`/my/marketplace`, Apps do cliente) **só** lista o que tem GitHub Release com asset — ver skill `marketplace-publish`. Feat na `main` não publica o `xvpn-chat` sozinho.

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
- Mergear a PR de release com a skill `land-pr` (não `--admin`). A tag dispara `release-chat.yml` / `release-client.yml` e o sync do marketplace.
