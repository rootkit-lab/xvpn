#!/bin/bash
# Auditoria de segurança rápida do VPS de produção do XVPN.
# Uso: audit.sh [usuario@host]
set -euo pipefail

HOST="${1:-root@206.189.224.72}"

ssh -o ConnectTimeout=10 "$HOST" bash -s <<'REMOTE'
export LC_ALL=C
echo "===== SSH (config efetiva) ====="
sshd -T 2>/dev/null | grep -iE '^(passwordauthentication|permitrootlogin|kbdinteractiveauthentication) ' || echo "não foi possível ler sshd -T"
echo
echo "===== UFW ====="
ufw status verbose 2>/dev/null || echo "ufw indisponível ou inativo"
echo
echo "===== ip_forward ====="
cat /proc/sys/net/ipv4/ip_forward
echo
echo "===== Portas escutando (TCP/UDP) ====="
ss -tulnp 2>/dev/null || ss -tuln
echo
echo "===== Samba: bind de interfaces ====="
if [ -f /etc/samba/smb.conf ]; then
  grep -iE 'bind interfaces only|^[[:space:]]*interfaces' /etc/samba/smb.conf || echo "smb.conf existe mas sem restrição de interface configurada (RISCO)"
else
  echo "Samba ainda não instalado"
fi
echo
echo "===== WireGuard ====="
wg show 2>/dev/null || echo "wg não instalado ou sem interface ativa"
echo
echo "===== fail2ban ====="
systemctl is-active fail2ban 2>/dev/null || echo "fail2ban não instalado/ativo"
REMOTE
