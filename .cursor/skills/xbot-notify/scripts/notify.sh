#!/bin/bash
# POST /api/hooks/chat/broadcast como xbot.
set -euo pipefail
BODY="${1:?uso: notify.sh \"mensagem\"}"
BASE="${XVPN_HOOK_URL:-https://xvpn.ihuull.com}"
if [ -z "${XBOT_TOKEN:-}" ]; then
  echo "XBOT_TOKEN ausente" >&2
  exit 2
fi
curl -fsS -X POST "$BASE/api/hooks/chat/broadcast" \
  -H "Authorization: Bearer $XBOT_TOKEN" \
  -H "Content-Type: application/json" \
  -d "$(jq -n --arg b "$BODY" '{body:$b,group:"Sistema"}')"
echo
