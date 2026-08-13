#!/usr/bin/env bash
# Resolve a versão semântica do cliente para builds/pacotes.
# Ordem: VERSION (env) → .release-please-manifest.json → git describe → 0.0.0
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

if [[ -n "${VERSION:-}" ]]; then
  echo "${VERSION}"
  exit 0
fi

MANIFEST="${ROOT}/../.release-please-manifest.json"
if [[ -f "${MANIFEST}" ]]; then
  # Prefer jq; fallback to python for environments without jq.
  if command -v jq >/dev/null 2>&1; then
    ver="$(jq -r '.client // empty' "${MANIFEST}")"
  else
    ver="$(python3 -c "import json; print(json.load(open('${MANIFEST}')).get('client',''))" 2>/dev/null || true)"
  fi
  if [[ -n "${ver}" && "${ver}" != "null" ]]; then
    echo "${ver}"
    exit 0
  fi
fi

if command -v git >/dev/null 2>&1 && git -C "${ROOT}" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  if desc="$(git -C "${ROOT}" describe --tags --match 'client-v*' --always --dirty 2>/dev/null)"; then
    # client-v1.2.3-4-gabcdef → 1.2.3+4.gabcdef (semântico o bastante p/ nfpm)
    echo "${desc#client-v}"
    exit 0
  fi
fi

echo "0.0.0"
