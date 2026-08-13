#!/bin/sh
# preremove: para o helper antes de remover arquivos do pacote.
set -eu

if command -v systemctl >/dev/null 2>&1; then
  systemctl stop xvpn-client-helper.service >/dev/null 2>&1 || true
  systemctl disable xvpn-client-helper.service >/dev/null 2>&1 || true
fi

exit 0
