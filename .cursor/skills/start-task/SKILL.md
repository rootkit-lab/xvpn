---
name: start-task
description: Cria uma branch de trabalho a partir da main atualizada, seguindo a convenção de nomenclatura do CONTRIBUTING.md (feat/, fix/, chore/, security/, docs/). Use sempre que uma nova tarefa não-trivial for iniciada neste projeto, antes de editar qualquer arquivo — nunca edite/commite diretamente na main.
---

# Iniciar uma tarefa (start-task)

Este projeto segue [GitHub Flow](../../../CONTRIBUTING.md) com `main` protegida (local via `.githooks/pre-commit`, remoto via *branch protection* no GitHub). Toda tarefa começa criando uma branch — nunca editando direto em `main`.

## Uso

```bash
.cursor/skills/start-task/scripts/start-task.sh <tipo>/<descricao-curta>
```

Onde `<tipo>` é um de: `feat`, `fix`, `chore`, `security`, `docs` (ver `CONTRIBUTING.md` para o significado de cada um).

Exemplo:

```bash
.cursor/skills/start-task/scripts/start-task.sh feat/enrollment-endpoint
```

## O que o script faz

1. Verifica se há mudanças não commitadas na branch atual — se houver, aborta e avisa (evita misturar trabalho de tarefas diferentes na mesma branch por engano).
2. Muda para `main` e atualiza via `git pull --ff-only` (falha alto e claro se `main` local divergiu do remoto, em vez de silenciosamente mesclar).
3. Cria e troca para a nova branch a partir da `main` atualizada.

## Depois de criar a branch

1. Rode a skill **`tasks`** (ou `.cursor/skills/tasks/scripts/init-tasks.sh "título"`) e preencha o [`TASKS.md`](../../../TASKS.md) da branch.
2. Trabalhe normalmente, commits pequenos em [Conventional Commits](https://www.conventionalcommits.org/).
3. Quando terminar, use a skill `ship-pr` para enviar e abrir o Pull Request (atualize o campo PR no `TASKS.md`).
