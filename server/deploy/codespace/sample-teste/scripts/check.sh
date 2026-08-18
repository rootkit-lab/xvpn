#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
echo "== go version =="
go version
echo "== go test =="
go test ./...
echo "== node =="
node -v
node web/index.mjs
echo "ok — playground XCODESPACES"
