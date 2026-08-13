#!/bin/bash
# Lista contas Samba ativas no VPS do XVPN.
# Uso: list-users.sh [usuario@host]
set -euo pipefail

HOST="${1:-root@206.189.224.72}"

ssh -o ConnectTimeout=10 "$HOST" "pdbedit -L -v 2>/dev/null || pdbedit -L"
