#!/bin/bash
# Lista as PRs de release abertas pelo release-please, com changelog pendente.
set -euo pipefail

echo "Consultando PRs de release (label: autorelease: pending)..."
echo

prs=$(gh pr list --state open --search "label:\"autorelease: pending\"" --json number,title,headRefName,body,url)

count=$(echo "$prs" | grep -c '"number"' || true)

if [ "$count" -eq 0 ]; then
  echo "Nenhuma PR de release pendente."
  echo "Isso significa: todos os componentes estão na última versão publicada, ou o workflow"
  echo "release-please ainda não foi criado (ver ROADMAP.md, Fases 2 e 4)."
  exit 0
fi

echo "$prs" | python3 -c '
import json, sys
prs = json.load(sys.stdin)
for pr in prs:
    print("=" * 70)
    print(f"#{pr[\"number\"]} — {pr[\"title\"]}")
    print(f"Branch: {pr[\"headRefName\"]}")
    print(f"URL: {pr[\"url\"]}")
    print()
    body = pr.get("body") or ""
    print(body[:1500])
    if len(body) > 1500:
        print("... (truncado, ver URL para o changelog completo)")
    print()
'
