#!/usr/bin/env bash
# Resolve a versão semântica do cliente para builds/pacotes.
# Ordem: VERSION (env) → .release-please-manifest.json → git describe → 0.0.0
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

if [[ -n "${VERSION:-}" ]]; then
  echo "${VERSION}"
  exit 0
fi

# ROOT é apps/xvpn-client/, então o manifest da raiz do repo fica dois níveis acima.
# A chave do componente no manifest é o caminho do pacote, não o nome dele.
MANIFEST="${ROOT}/../../.release-please-manifest.json"
COMPONENT_PATH="apps/xvpn-client"
if [[ -f "${MANIFEST}" ]]; then
  # Prefer jq; fallback to python for environments without jq.
  if command -v jq >/dev/null 2>&1; then
    ver="$(jq -r --arg key "${COMPONENT_PATH}" '.[$key] // empty' "${MANIFEST}")"
  else
    ver="$(python3 -c "import json,sys; print(json.load(open(sys.argv[1])).get(sys.argv[2],''))" "${MANIFEST}" "${COMPONENT_PATH}" 2>/dev/null || true)"
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
