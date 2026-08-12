#!/bin/bash
# Envia a branch atual e abre um Pull Request com título em Conventional Commits.
# Uso: ship-pr.sh "<tipo>(<escopo>): <título>" ["<corpo opcional>"]
set -euo pipefail

if [ $# -lt 1 ]; then
  echo 'Uso: ship-pr.sh "<tipo>(<escopo>): <título Conventional Commits>" ["<corpo opcional>"]' >&2
  exit 1
fi
title="$1"
body="${2:-}"

current_branch=$(git rev-parse --abbrev-ref HEAD)

if echo "$current_branch" | grep -qE '^(main|master)$'; then
  echo "Erro: você está em '$current_branch'. Crie uma branch de trabalho primeiro (skill start-task)." >&2
  exit 1
fi

if [ -n "$(git status --porcelain)" ]; then
  echo "Erro: há mudanças não commitadas. Commite tudo antes de abrir o PR." >&2
  git status --short >&2
  exit 1
fi

conventional_pattern='^(feat|fix|chore|docs|refactor|test|security|perf)(\([a-zA-Z0-9_-]+\))?!?: .+'
if ! echo "$title" | grep -qE "$conventional_pattern"; then
  echo "Erro: título '$title' não parece seguir Conventional Commits." >&2
  echo "Formato esperado: <tipo>(<escopo opcional>)[!]: <descrição> — tipos: feat, fix, chore, docs, refactor, test, security, perf" >&2
  exit 1
fi

cat <<'EOF'
=====================================================================
Checklist pré-PR (CONTRIBUTING.md § Antes de abrir PR / finalizar uma tarefa)
Confira cada item ANTES de continuar (Ctrl+C para abortar e ajustar):
  [ ] gofmt/go vet limpos no código Go alterado
  [ ] Lint/format do frontend (eslint/prettier) limpos quando aplicável
  [ ] Nenhum segredo (chave privada, token, senha) commitado
  [ ] Nenhum artefato de build commitado (ver PLAN.md §11.1)
  [ ] Nenhuma porta/serviço novo exposto sem registro em PLAN.md §5
  [ ] Checkboxes relevantes do ROADMAP.md atualizados
  [ ] Se mexeu em Samba/FileBrowser/firewall: rodou vps-security-audit
=====================================================================
EOF

echo "Enviando branch '$current_branch'..."
git push -u origin "$current_branch"

echo "Abrindo Pull Request..."
if [ -n "$body" ]; then
  gh pr create --title "$title" --body "$body" --base main
else
  gh pr create --title "$title" --body "" --base main
fi
