#!/bin/bash
# Remove um peer da interface wg0 no servidor XVPN.
# Uso: remove-peer.sh <chave-publica-do-peer> [usuario@host]
set -euo pipefail

PUBKEY="${1:?Uso: remove-peer.sh <chave-publica-do-peer> [usuario@host]}"
HOST="${2:-root@206.189.224.72}"

ssh -o ConnectTimeout=10 "$HOST" "wg set wg0 peer '$PUBKEY' remove"
echo "Peer ${PUBKEY} removido de wg0 em ${HOST}."
