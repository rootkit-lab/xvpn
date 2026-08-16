#!/bin/sh
# postinstall do pacote xvpn-client (.deb/.rpm): cria grupo/usuário do helper,
# habilita o serviço systemd e adiciona o usuário que instalou ao grupo xvpn
# (mesmo modelo do grupo docker — ver internal/ipc/transport_linux.go).
set -eu

# --- integração desktop ---
if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database -q /usr/share/applications || true
fi
if command -v update-mime-database >/dev/null 2>&1; then
  update-mime-database -n /usr/share/mime || true
fi
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
  gtk-update-icon-cache -f -t /usr/share/icons/hicolor >/dev/null 2>&1 || true
fi

# --- grupo + usuário do helper ---
if ! getent group xvpn >/dev/null 2>&1; then
  groupadd --system xvpn
fi

if ! getent passwd xvpn-client-helper >/dev/null 2>&1; then
  useradd --system --no-create-home --shell /usr/sbin/nologin \
    --gid xvpn --comment "XVPN client helper" xvpn-client-helper
fi

# A GUI precisa do grupo xvpn para o socket 0660. Instalador gráfico
# (GNOME Software / pkexec) muitas vezes não define SUDO_USER — então
# também entra todo usuário humano local (uid >= 1000, com shell).
add_xvpn() {
  u="$1"
  [ -z "$u" ] && return 0
  [ "$u" = "root" ] && return 0
  id "$u" >/dev/null 2>&1 || return 0
  usermod -aG xvpn "$u" || true
}

INSTALL_USER="${SUDO_USER:-}"
if [ -z "$INSTALL_USER" ] || [ "$INSTALL_USER" = "root" ]; then
  if [ -n "${PKEXEC_UID:-}" ]; then
    INSTALL_USER="$(getent passwd "$PKEXEC_UID" | cut -d: -f1 || true)"
  fi
fi
add_xvpn "$INSTALL_USER"

getent passwd | awk -F: '$3 >= 1000 && $3 < 65534 && $7 !~ /(nologin|false)/ { print $1 }' | while read -r u; do
  add_xvpn "$u"
done

echo "XVPN: grupo 'xvpn' atualizado. Saia da sessão e entre de novo (ou newgrp xvpn)"
echo "     antes de abrir o cliente — senão a GUI não alcança o helper."

# --- systemd ---
if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
  systemctl enable xvpn-client-helper.service >/dev/null 2>&1 || true
  systemctl restart xvpn-client-helper.service >/dev/null 2>&1 || \
    systemctl start xvpn-client-helper.service >/dev/null 2>&1 || true
fi

exit 0
