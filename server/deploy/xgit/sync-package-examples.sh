#!/bin/bash
# Copia os exemplos embutidos para $HOME/Projects/x/packages (ou $1).
# Ficheiros *.src perdem o sufixo (hello.go.src → hello.go) para o tree
# local parecer um projeto real.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
SRC="$ROOT/server/internal/pkgexamples/fs"
DEST="${1:-$HOME/Projects/x/packages}"

if [ ! -d "$SRC" ]; then
  echo "fonte inexistente: $SRC" >&2
  exit 1
fi

copy_lang() {
  local from="$1"
  local to="$2"
  mkdir -p "$to"
  find "$from" -type f | while IFS= read -r f; do
    rel="${f#"$from"/}"
    dest_rel="${rel%.src}"
    mkdir -p "$to/$(dirname "$dest_rel")"
    cp -a "$f" "$to/$dest_rel"
  done
}

mkdir -p "$DEST"
copy_lang "$SRC/javascript" "$DEST/javascript"
copy_lang "$SRC/python" "$DEST/python"
copy_lang "$SRC/golang" "$DEST/go"
copy_lang "$SRC/rust" "$DEST/rust"
copy_lang "$SRC/generic" "$DEST/generic"
cp -a "$ROOT/server/deploy/xgit/packages/README.md" "$DEST/README.md"
rm -rf "$DEST/golang"
echo "ok $DEST ← $SRC"
