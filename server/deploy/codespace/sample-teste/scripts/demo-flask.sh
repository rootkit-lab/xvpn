#!/usr/bin/env bash
# Sobe o canário Flask em 0.0.0.0:8080 para validar demo-<nome>.corp (Fase 57).
set -euo pipefail
cd "$(dirname "$0")/.."
export PORT="${PORT:-8080}"
echo "Flask em 0.0.0.0:${PORT} — abra https://demo-<nome>.corp.ihuull.com:${PORT}/ na VPN"
exec python3 web/flask/app.py
