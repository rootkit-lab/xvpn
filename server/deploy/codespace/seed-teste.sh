#!/usr/bin/env bash
# Re-semeia o bare XGIT `teste` com o playground em sample-teste/.
# No VPS: sudo -u xvpn server/deploy/codespace/seed-teste.sh
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
SAMPLE="${SAMPLE_DIR:-$SCRIPT_DIR/sample-teste}"
BARE="${BARE_PATH:-/opt/xvpn/data/git/xcorp/teste.git}"
WORKDIR="${WORKDIR:-$(mktemp -d /tmp/seed-teste.XXXXXX)}"

cleanup() { rm -rf "$WORKDIR"; }
trap cleanup EXIT

if [[ ! -d "$SAMPLE" ]]; then
  echo "sample-teste ausente: $SAMPLE" >&2
  exit 1
fi
if [[ ! -d "$BARE" ]]; then
  echo "bare ausente: $BARE" >&2
  exit 1
fi

git clone --branch main "$BARE" "$WORKDIR/repo"
cd "$WORKDIR/repo"

# Não apaga o histórico — só atualiza a árvore.
find . -mindepth 1 -maxdepth 1 ! -name .git -exec rm -rf {} +
cp -a "$SAMPLE"/. .
chmod +x scripts/check.sh

git add -A
if git diff --cached --quiet; then
  echo "teste já está semeado; nada a commitar"
  exit 0
fi

git -c user.name='xbot' -c user.email='xbot@ihuull.com' commit -m "$(cat <<'EOF'
feat: playground Go+Node para validar o XCODESPACES

EOF
)"
git push origin HEAD:main
echo "push ok → $BARE"
