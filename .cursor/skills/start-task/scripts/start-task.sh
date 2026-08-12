#!/bin/bash
# Cria uma branch de trabalho a partir da main atualizada.
# Uso: start-task.sh <tipo>/<descricao-curta>
set -euo pipefail

if [ $# -lt 1 ]; then
  echo "Uso: start-task.sh <tipo>/<descricao-curta> (ex: feat/enrollment-endpoint)" >&2
  exit 1
fi
branch_name="$1"

if ! echo "$branch_name" | grep -qE '^(feat|fix|chore|security|docs)/[a-z0-9-]+$'; then
  echo "Erro: '$branch_name' não segue a convenção <tipo>/<descricao-curta>." >&2
  echo "Tipos válidos: feat, fix, chore, security, docs (ver CONTRIBUTING.md)." >&2
  exit 1
fi

if [ -n "$(git status --porcelain)" ]; then
  echo "Erro: há mudanças não commitadas na branch atual. Commite, stash ou descarte antes de iniciar uma nova tarefa." >&2
  git status --short >&2
  exit 1
fi

current_branch=$(git rev-parse --abbrev-ref HEAD)
default_branch="main"

echo "Atualizando '$default_branch'..."
git checkout "$default_branch"
if ! git pull --ff-only; then
  echo "Erro: '$default_branch' local divergiu do remoto de um jeito que não dá pra fast-forward." >&2
  echo "Resolva manualmente antes de continuar (não vamos mesclar automaticamente)." >&2
  exit 1
fi

if git show-ref --verify --quiet "refs/heads/$branch_name"; then
  echo "Erro: a branch '$branch_name' já existe localmente." >&2
  exit 1
fi

git checkout -b "$branch_name"
echo
echo "Branch '$branch_name' criada a partir de '$default_branch' atualizada."
echo "Branch anterior era: '$current_branch'."
