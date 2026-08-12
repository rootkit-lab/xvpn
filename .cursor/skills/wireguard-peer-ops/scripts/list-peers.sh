#!/bin/bash
# Lista peers e status da interface wg0 no servidor XVPN.
# Uso: list-peers.sh [usuario@host]
set -euo pipefail

HOST="${1:-root@206.189.224.72}"

ssh -o ConnectTimeout=10 "$HOST" "wg show all"
