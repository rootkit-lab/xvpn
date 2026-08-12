---
name: ship-pr
description: Envia (push) a branch de trabalho atual, roda o checklist pré-PR do CONTRIBUTING.md como lembrete, e abre o Pull Request no GitHub via gh pr create. Use quando o trabalho de uma branch feat/fix/chore/security/docs estiver pronto para revisão/merge. Como o repositório só permite squash merge, o título do PR vira o commit final na main e precisa seguir Conventional Commits.
---

# Abrir o Pull Request (ship-pr)

Fecha o ciclo iniciado com a skill `start-task`: envia a branch, lembra do checklist do [`CONTRIBUTING.md`](../../../CONTRIBUTING.md#antes-de-abrir-pr--finalizar-uma-tarefa) e abre o PR.

## Pré-requisito importante: título do PR = commit final

Este repositório só permite **squash merge** (configurado na branch protection). Isso significa que **o título do PR, não os commits individuais da branch, é o que vira a mensagem do commit em `main`** — e é esse commit que ferramentas como o `release-please` vão analisar para decidir a próxima versão. Portanto:

- O título do PR **precisa** seguir [Conventional Commits](https://www.conventionalcommits.org/): `<tipo>(<escopo opcional>): <descrição no imperativo>`.
- Use `!` depois do tipo/escopo (ex.: `feat(client)!: ...`) se a mudança for *breaking*.

## Uso

```bash
.cursor/skills/ship-pr/scripts/ship-pr.sh "<tipo>(<escopo>): <título Conventional Commits>" ["<corpo opcional do PR>"]
```

Exemplo:

```bash
.cursor/skills/ship-pr/scripts/ship-pr.sh "feat(server): adiciona endpoint de enrollment de dispositivos"
```

## O que o script faz

1. Confirma que você não está em `main`/`master` (se estiver, aborta — use `start-task` primeiro).
2. Confirma que não há mudanças não commitadas (se houver, aborta).
3. Imprime o checklist pré-PR do `CONTRIBUTING.md` na tela como lembrete manual — **o agente/você deve revisar item a item antes de continuar**, não é verificado automaticamente.
4. Dá `git push -u origin <branch-atual>`.
5. Abre o PR com `gh pr create --title "<título>" --body "<corpo>" --base main`, usando o título exatamente como passado (validando que segue o formato Conventional Commits antes de enviar).

## Depois de aberto

- Revise o diff completo na interface do GitHub (ou `gh pr view --web`) antes de mergear — esse é o ponto de checagem final, mesmo trabalhando sozinho (ver `CONTRIBUTING.md`).
- Faça o merge com `gh pr merge --squash --delete-branch`.
- Sincronize a `main` local: `git checkout main && git pull --ff-only`, e remova a branch local com `git branch -D <branch>` (use `-D` maiúsculo — o squash merge cria um SHA novo, então o Git não reconhece a branch local como "merged" para o `-d` minúsculo funcionar).
