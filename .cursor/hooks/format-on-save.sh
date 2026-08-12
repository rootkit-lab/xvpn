#!/bin/bash
# Hook afterFileEdit — formata automaticamente arquivos Go e frontend
# após edição pelo agente. Best-effort: se a ferramenta não estiver
# instalada ou o arquivo ainda não existir num projeto configurado,
# simplesmente não faz nada (nunca bloqueia a edição).

input=$(cat)
file_path=$(echo "$input" | jq -r '.file_path // .path // empty' 2>/dev/null)

if [ -z "$file_path" ] || [ ! -f "$file_path" ]; then
  echo '{"continue": true}'
  exit 0
fi

case "$file_path" in
  *.go)
    if command -v gofmt >/dev/null 2>&1; then
      gofmt -w "$file_path" 2>/dev/null
    fi
    ;;
  *.ts|*.tsx|*.js|*.jsx|*.json|*.css)
    dir=$(dirname "$file_path")
    if [ -f "$dir/package.json" ] || [ -f "$(git -C "$dir" rev-parse --show-toplevel 2>/dev/null)/package.json" ]; then
      if command -v npx >/dev/null 2>&1; then
        npx --no-install prettier --write "$file_path" >/dev/null 2>&1
      fi
    fi
    ;;
esac

echo '{"continue": true}'
