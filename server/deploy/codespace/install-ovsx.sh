#!/bin/bash
# Instala VSIX só do Open VSX (allowlist). Sem Marketplace Microsoft.
set -euo pipefail
EXTS=(
  golang.go
  dbaeumer.vscode-eslint
  esbenp.prettier-vscode
  yzhang.markdown-all-in-one
  redhat.vscode-yaml
)
BIN="${OPENVSCODE_BIN:-/home/.openvscode-server/bin/openvscode-server}"
DIR="${TMPDIR:-/tmp}/ihuull-ovsx"
mkdir -p "$DIR"
for id in "${EXTS[@]}"; do
  pub="${id%%.*}"
  name="${id#*.}"
  meta="$(curl -fsSL "https://open-vsx.org/api/${pub}/${name}/latest")"
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
  "$BIN" --install-extension "$DIR/${id}.vsix"
done
rm -rf "$DIR"
