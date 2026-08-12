#!/usr/bin/env bash
# Backup diário do xvpn.db via `sqlite3 .backup` (snapshot consistente mesmo
# com o banco em uso), com rotação de 7 dias. Instalado via cron no VPS
# (ver server/deploy/xvpn-backup.cron) — não decore isso na sua cabeça,
# é só um `crontab` apontando pra este script.
set -euo pipefail

DB_PATH="${XVPN_DB_PATH:-/opt/xvpn/data/xvpn.db}"
BACKUP_DIR="${XVPN_BACKUP_DIR:-/opt/xvpn/backups}"
RETENTION_DAYS="${XVPN_BACKUP_RETENTION_DAYS:-7}"

if [ ! -f "$DB_PATH" ]; then
  echo "xvpn-backup: banco não encontrado em $DB_PATH, nada a fazer." >&2
  exit 0
fi

mkdir -p "$BACKUP_DIR"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
dest="$BACKUP_DIR/xvpn-$timestamp.db"

sqlite3 "$DB_PATH" ".backup '$dest'"
gzip "$dest"

find "$BACKUP_DIR" -name 'xvpn-*.db.gz' -mtime "+$RETENTION_DAYS" -delete

echo "xvpn-backup: snapshot salvo em $dest.gz"
