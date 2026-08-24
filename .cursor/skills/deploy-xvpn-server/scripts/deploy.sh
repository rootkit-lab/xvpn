#!/bin/bash
# Build local (cgo) + troca do binário no VPS. Uso: deploy.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../../.." && pwd)"
HOST="${XVPN_DEPLOY_HOST:-root@66.29.147.100}"
BIN_SERVER="/tmp/xvpn-server-new"
BIN_PROVISION="/tmp/xvpn-user-provision-new"
BIN_GIT_SHELL="/tmp/xvpn-git-shell-new"

echo "=== baseline produção (read-only) ==="
ssh -o ConnectTimeout=15 "$HOST" 'systemctl is-active xvpn-server; curl -sf -o /dev/null -w "api_status=%{http_code}\n" http://127.0.0.1:8080/api/status; ss -tulnp | grep -E "xvpn-server|:445 " || true; wg show wg0 | head -12'

echo "=== npm (chat + painel) ==="
(cd "$ROOT/apps/xvpn-chat/frontend" && npm ci)
(cd "$ROOT/server/web" && npm ci && npm run build)

echo "=== go build (server cgo + user-provision + git-shell) ==="
# overlay-apply vive no user-provision — deploy dos dois juntos (crash pós-#179).
(cd "$ROOT/server" && CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o "$BIN_SERVER" ./cmd/xvpn-server)
(cd "$ROOT/server" && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$BIN_PROVISION" ./cmd/xvpn-user-provision)
(cd "$ROOT/server" && CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o "$BIN_GIT_SHELL" ./cmd/xvpn-git-shell)
ls -lh "$BIN_SERVER" "$BIN_PROVISION" "$BIN_GIT_SHELL"
file "$BIN_SERVER" "$BIN_PROVISION" "$BIN_GIT_SHELL"

echo "=== upload + swap ==="
scp -o ConnectTimeout=15 "$BIN_SERVER" "$BIN_PROVISION" "$BIN_GIT_SHELL" "$HOST:/tmp/"
ssh -o ConnectTimeout=15 "$HOST" 'set -e
TS=$(date -u +%Y%m%dT%H%M%SZ)
cp -a /opt/xvpn/bin/xvpn-server /opt/xvpn/bin/xvpn-server.bak-$TS
cp -a /opt/xvpn/bin/xvpn-user-provision /opt/xvpn/bin/xvpn-user-provision.bak-$TS
if test -f /opt/xvpn/bin/xvpn-git-shell; then cp -a /opt/xvpn/bin/xvpn-git-shell /opt/xvpn/bin/xvpn-git-shell.bak-$TS; fi
install -o xvpn -g xvpn -m 0755 /tmp/xvpn-server-new /opt/xvpn/bin/xvpn-server
install -o root -g root -m 0755 /tmp/xvpn-user-provision-new /opt/xvpn/bin/xvpn-user-provision
install -o root -g root -m 0755 /tmp/xvpn-git-shell-new /opt/xvpn/bin/xvpn-git-shell
rm -f /tmp/xvpn-server-new /tmp/xvpn-user-provision-new /tmp/xvpn-git-shell-new
systemctl reset-failed xvpn-server || true
systemctl restart xvpn-server
sleep 4
echo "active=$(systemctl is-active xvpn-server)"
curl -sf -o /dev/null -w "api_local=%{http_code}\n" http://127.0.0.1:8080/api/status
curl -s -o /dev/null -w "ws_query=%{http_code}\n" "http://127.0.0.1:8080/api/ws?token=nope"
ss -tulnp | grep -E "xvpn-server|smbd" || true
wg show wg0 | head -16
journalctl -u xvpn-server -n 15 --no-pager | tail -15
'
rm -f "$BIN_SERVER" "$BIN_PROVISION" "$BIN_GIT_SHELL"

echo "=== público ==="
curl -sf -o /dev/null -w "public_api=%{http_code}\n" https://xvpn.ihuull.com/api/status
curl -s https://xvpn.ihuull.com/ 2>/dev/null | grep -oE 'assets/index-[A-Za-z0-9_-]+\.js' | head -1 || true
echo "deploy ok"
