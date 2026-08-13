#!/bin/bash
# Cria um usuário Samba manualmente no VPS do XVPN (sem sync automático com
# o painel — ver ROADMAP.md Fase 5, item marcado como opcional).
# Uso: add-user.sh <username> [usuario@host]
set -euo pipefail

USERNAME="${1:?uso: add-user.sh <username> [usuario@host]}"
HOST="${2:-root@206.189.224.72}"

if ! [[ "$USERNAME" =~ ^[a-z][a-z0-9_-]{1,31}$ ]]; then
  echo "Nome de usuário inválido: '$USERNAME' (use minúsculas, números, - ou _, começando com letra)." >&2
  exit 1
fi

PASSWORD=$(openssl rand -base64 18)

ssh -o ConnectTimeout=10 "$HOST" "bash -s" <<EOF
set -euo pipefail
getent group xvpn-samba >/dev/null || groupadd xvpn-samba

if id "$USERNAME" >/dev/null 2>&1; then
  echo "Usuário do sistema '$USERNAME' já existe — só adicionando/atualizando ao Samba." >&2
else
  useradd --system --no-create-home --shell /usr/sbin/nologin -G xvpn-samba "$USERNAME"
fi

printf '%s\n%s\n' "$PASSWORD" "$PASSWORD" | smbpasswd -a -s "$USERNAME"
smbpasswd -e "$USERNAME"
EOF

cat <<EOF

Usuário Samba criado/atualizado: $USERNAME
Senha (gerada, copie agora — não fica salva em nenhum lugar): $PASSWORD

Compartilhamento: \\\\10.66.66.1\\shared (Windows) ou smb://10.66.66.1/shared (Linux/macOS),
só acessível com o túnel WireGuard ativo.
EOF
