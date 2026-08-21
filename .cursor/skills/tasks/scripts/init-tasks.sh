#!/bin/bash
# Scaffold TASKS.md na branch atual. Uso: init-tasks.sh "título curto"
set -euo pipefail

title="${1:-}"
if [ -z "$title" ]; then
  echo "Uso: init-tasks.sh \"título curto\"" >&2
  exit 1
fi

branch="$(git rev-parse --abbrev-ref HEAD)"
if [ "$branch" = "main" ] || [ "$branch" = "master" ]; then
  echo "Erro: não crie TASKS.md em main/master — use start-task primeiro." >&2
  exit 1
fi

if [ -f TASKS.md ]; then
  echo "TASKS.md já existe — não sobrescrevo. Edite à mão ou remova antes." >&2
  exit 1
fi

cat > TASKS.md <<EOF
# TASKS — ${title}

> Branch: \`${branch}\`
> PR: _(abrir com ship-pr)_
> Fase: _

## Objetivo

_

## Contexto

_

## Checklist

- [ ] Implementação
- [ ] Testes Go/UI relevantes passam
- [ ] \`PLAN.md\` / \`ROADMAP.md\` / docs da área atualizados (se arquitetura)
- [ ] Sem segredos no Git

## Fora de escopo

- _

## Critério de saída

- _

## Notas para o agente

- Skills: \`start-task\` → trabalho → \`ship-pr\` → \`land-pr\` → \`deploy-xvpn-server\` se server/
- Nunca commit em \`main\`. Nunca \`git commit --no-verify\` sem confirmação.
EOF

echo "Criado TASKS.md na branch ${branch}."
