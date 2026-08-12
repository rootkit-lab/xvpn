#!/bin/bash
# Gera um par de chaves WireGuard localmente e registra a chave pública
# como peer no servidor XVPN. A chave privada NUNCA é enviada ao servidor
# (ver invariante de segurança em SECURITY.md).
#
# Uso: add-test-peer.sh <ip-do-peer, ex: 10.66.66.2> [usuario@host]
set -euo pipefail

PEER_IP="${1:?Uso: add-test-peer.sh <ip-do-peer, ex: 10.66.66.2> [usuario@host]}"
HOST="${2:-root@206.189.224.72}"
SERVER_ENDPOINT="206.189.224.72:51820"

if ! command -v wg >/dev/null 2>&1; then
  echo "Erro: o comando 'wg' (wireguard-tools) precisa estar instalado localmente para gerar as chaves." >&2
  exit 1
fi

private_key=$(wg genkey)
public_key=$(echo "$private_key" | wg pubkey)

echo "Registrando peer no servidor ($HOST)..." >&2
ssh -o ConnectTimeout=10 "$HOST" "wg set wg0 peer '$public_key' allowed-ips '${PEER_IP}/32'"

server_public_key=$(ssh -o ConnectTimeout=10 "$HOST" "wg show wg0 public-key")

cat <<EOF

===== Peer registrado com sucesso no servidor =====
IP alocado ao peer: ${PEER_IP}/32
Chave pública do peer (registrada no servidor): ${public_key}

===== Configuração para o cliente de teste (NÃO commitar este bloco) =====
[Interface]
PrivateKey = ${private_key}
Address = ${PEER_IP}/24
DNS = 1.1.1.1

[Peer]
PublicKey = ${server_public_key}
Endpoint = ${SERVER_ENDPOINT}
AllowedIPs = 0.0.0.0/0, ::/0
PersistentKeepalive = 25

===== Comando executado no servidor (referência) =====
wg set wg0 peer ${public_key} allowed-ips ${PEER_IP}/32
EOF
