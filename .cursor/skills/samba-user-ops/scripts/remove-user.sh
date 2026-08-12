#!/bin/bash
# Remove um usuário Samba do VPS do XVPN (conta Samba + usuário de sistema).
# Uso: remove-user.sh <username> [usuario@host]
set -euo pipefail

USERNAME="${1:?uso: remove-user.sh <username> [usuario@host]}"
HOST="${2:-root@206.189.224.72}"

ssh -o ConnectTimeout=10 "$HOST" "bash -s" <<EOF
set -euo pipefail
smbpasswd -x "$USERNAME" 2>/dev/null || echo "Aviso: '$USERNAME' não tinha conta Samba ativa." >&2
if id "$USERNAME" >/dev/null 2>&1; then
  userdel "$USERNAME"
  echo "Usuário de sistema '$USERNAME' removido."
else
  echo "Usuário de sistema '$USERNAME' já não existia." >&2
fi
EOF
