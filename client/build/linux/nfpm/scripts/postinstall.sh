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

# Usuário que rodou sudo apt/dpkg — a GUI dele precisa falar com o socket.
INSTALL_USER="${SUDO_USER:-}"
if [ -z "$INSTALL_USER" ] || [ "$INSTALL_USER" = "root" ]; then
  # dpkg sem sudo direto (ex.: root logado) — tenta o dono do display mais comum.
  if [ -n "${PKEXEC_UID:-}" ]; then
    INSTALL_USER="$(getent passwd "$PKEXEC_UID" | cut -d: -f1 || true)"
  fi
fi
if [ -n "$INSTALL_USER" ] && [ "$INSTALL_USER" != "root" ]; then
  if id "$INSTALL_USER" >/dev/null 2>&1; then
    usermod -aG xvpn "$INSTALL_USER" || true
    echo "XVPN: usuário '$INSTALL_USER' adicionado ao grupo 'xvpn'."
    echo "     Faça logout/login (ou 'newgrp xvpn') para a GUI falar com o helper."
  fi
fi

# --- systemd ---
if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
  systemctl enable xvpn-client-helper.service >/dev/null 2>&1 || true
  systemctl restart xvpn-client-helper.service >/dev/null 2>&1 || \
    systemctl start xvpn-client-helper.service >/dev/null 2>&1 || true
fi

exit 0
