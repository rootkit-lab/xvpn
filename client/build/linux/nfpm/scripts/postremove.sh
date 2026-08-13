#!/bin/sh
# postremove: limpa unit cache. Não remove o usuário/grupo xvpn nem
# /var/lib/xvpn-client — estado (chave privada do dispositivo) e grupo
# compartilhado podem ser reutilizados numa reinstalação. O admin pode
# apagar manualmente se quiser wipe completo.
set -eu

if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
  systemctl reset-failed xvpn-client-helper.service >/dev/null 2>&1 || true
fi

if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database -q /usr/share/applications || true
fi

exit 0
