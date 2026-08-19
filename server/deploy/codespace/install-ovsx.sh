#!/bin/bash
# Instala VSIX só do Open VSX (allowlist). Sem Marketplace Microsoft.
# id ou id@version — Prettier latest exige VS Code > 1.98.
set -euo pipefail
EXTS=(
  golang.go
  dbaeumer.vscode-eslint
  esbenp.prettier-vscode@11.0.0
  yzhang.markdown-all-in-one
  redhat.vscode-yaml
)
BIN="${OPENVSCODE_BIN:-/home/.openvscode-server/bin/openvscode-server}"
DIR="${TMPDIR:-/tmp}/ihuull-ovsx"
mkdir -p "$DIR"
for spec in "${EXTS[@]}"; do
  id="${spec%%@*}"
  ver=""
  if [ "$spec" != "$id" ]; then
    ver="${spec#*@}"
  fi
  pub="${id%%.*}"
  name="${id#*.}"
  if [ -n "$ver" ]; then
    meta="$(curl -fsSL "https://open-vsx.org/api/${pub}/${name}/${ver}")"
  else
    meta="$(curl -fsSL "https://open-vsx.org/api/${pub}/${name}/latest")"
  fi
  url="$(printf '%s' "$meta" | node -e '
let s=""; process.stdin.on("data", d => s += d);
process.stdin.on("end", () => {
  const j = JSON.parse(s);
  const u = j.files && j.files.download;
  if (!u || !String(u).startsWith("https://open-vsx.org/")) process.exit(2);
  process.stdout.write(u);
});
')"
  curl -fsSL "$url" -o "$DIR/${id}.vsix"
  if ! "$BIN" --install-extension "$DIR/${id}.vsix"; then
    "$BIN" --install-extension "$DIR/${id}.vsix" --force
  fi
done
rm -rf "$DIR"
