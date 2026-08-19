#!/bin/bash
# Squash-merge de uma PR na main e sincroniza o clone local.
# Uso: land-pr.sh [número-do-PR]
set -euo pipefail

if [ $# -ge 1 ]; then
  pr="$1"
else
  pr="$(gh pr view --json number --jq .number)"
fi

if [ -z "$pr" ] || [ "$pr" = "null" ]; then
  echo "Erro: nenhum PR associado a esta branch. Passe o número: land-pr.sh 61" >&2
  exit 1
fi

echo "=== PR #$pr ==="
gh pr view "$pr" --json number,title,url,mergeable,mergeStateStatus \
  --jq '"\(.title)\n\(.url)\nmergeable=\(.mergeable) state=\(.mergeStateStatus)"'

echo
echo "=== Threads não resolvidas ==="
unresolved="$(gh api graphql -f query="
query {
  repository(owner: \"rootkit-lab\", name: \"xvpn\") {
    pullRequest(number: $pr) {
      reviewThreads(first: 40) {
        nodes { isResolved isOutdated comments(first: 1) { nodes { author { login } body } } }
      }
    }
  }
}" --jq '[.data.repository.pullRequest.reviewThreads.nodes[] | select(.isResolved==false)] | length')"
echo "não resolvidas: $unresolved"
if [ "$unresolved" != "0" ]; then
  gh api graphql -f query="
query {
  repository(owner: \"rootkit-lab\", name: \"xvpn\") {
    pullRequest(number: $pr) {
      reviewThreads(first: 40) {
        nodes { isResolved comments(first: 1) { nodes { author { login } body } } }
      }
    }
  }
}" --jq '.data.repository.pullRequest.reviewThreads.nodes[] | select(.isResolved==false) | .comments.nodes[0].body' | head -c 2000
  echo
  echo "Erro: resolva ou corrija os threads acima (branch protection exige conversation resolution)." >&2
  exit 2
fi

echo
echo "=== CI (watch) ==="
gh pr checks "$pr" --watch

echo
echo "=== Squash merge ==="
gh pr merge "$pr" --squash --delete-branch

echo
echo "=== Sync main ==="
git checkout main
git pull --ff-only
# squash gera SHA novo — -d falha; -D é o esperado
git branch -D "$(gh pr view "$pr" --json headRefName --jq .headRefName)" 2>/dev/null || true
git log -1 --oneline
git status -sb
