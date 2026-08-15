#!/bin/bash
# Build local (cgo) + troca do binário no VPS. Uso: deploy.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../../.." && pwd)"
HOST="${XVPN_DEPLOY_HOST:-root@206.189.224.72}"
BIN_LOCAL="/tmp/xvpn-server-new"

echo "=== baseline produção (read-only) ==="
ssh -o ConnectTimeout=15 "$HOST" 'systemctl is-active xvpn-server; curl -sf -o /dev/null -w "api_status=%{http_code}\n" http://127.0.0.1:8080/api/status; ss -tulnp | grep -E "xvpn-server|:445 " || true; wg show wg0 | head -12'

echo "=== npm (chat + painel) ==="
(cd "$ROOT/apps/xvpn-chat/frontend" && npm ci)
(cd "$ROOT/server/web" && npm ci && npm run build)

echo "=== go build (cgo) ==="
(cd "$ROOT/server" && CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o "$BIN_LOCAL" ./cmd/xvpn-server)
ls -lh "$BIN_LOCAL"
file "$BIN_LOCAL"

echo "=== upload + swap ==="
scp -o ConnectTimeout=15 "$BIN_LOCAL" "$HOST:/tmp/xvpn-server-new"
ssh -o ConnectTimeout=15 "$HOST" 'set -e
TS=$(date -u +%Y%m%dT%H%M%SZ)
cp -a /opt/xvpn/bin/xvpn-server /opt/xvpn/bin/xvpn-server.bak-$TS
install -o xvpn -g xvpn -m 0755 /tmp/xvpn-server-new /opt/xvpn/bin/xvpn-server
rm -f /tmp/xvpn-server-new
systemctl restart xvpn-server
sleep 3
echo "active=$(systemctl is-active xvpn-server)"
curl -sf -o /dev/null -w "api_local=%{http_code}\n" http://127.0.0.1:8080/api/status
curl -s -o /dev/null -w "ws_query=%{http_code}\n" "http://127.0.0.1:8080/api/ws?token=nope"
ss -tulnp | grep -E "xvpn-server|smbd" || true
wg show wg0 | head -16
'
rm -f "$BIN_LOCAL"

echo "=== público ==="
curl -sf -o /dev/null -w "public_api=%{http_code}\n" https://vpn.officeempresa.com/api/status
curl -s https://vpn.officeempresa.com/ | grep -oE 'assets/index-[A-Za-z0-9_-]+\.js' | head -1
echo "deploy ok"
