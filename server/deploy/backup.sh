#!/usr/bin/env bash
# Backup diário do xvpn.db via `sqlite3 .backup` (snapshot consistente mesmo
# com o banco em uso), com rotação de 7 dias, + espelho incremental dos
# blobs do marketplace (Fase 11, PLAN.md §6.8). Instalado via cron no VPS
# (ver server/deploy/xvpn-backup.cron) — não decore isso na sua cabeça,
# é só um `crontab` apontando pra este script.
set -euo pipefail

DB_PATH="${XVPN_DB_PATH:-/opt/xvpn/data/xvpn.db}"
BACKUP_DIR="${XVPN_BACKUP_DIR:-/opt/xvpn/backups}"
RETENTION_DAYS="${XVPN_BACKUP_RETENTION_DAYS:-7}"
MARKETPLACE_DIR="${XVPN_MARKETPLACE_DIR:-/opt/xvpn/data/marketplace}"
SOCIAL_MEDIA_DIR="${XVPN_SOCIAL_MEDIA_DIR:-/opt/xvpn/data/social}"
PACKAGES_DIR="${XVPN_PACKAGES_DIR:-/opt/xvpn/data/packages}"
MONGO_URI="${XVPN_MONGO_URI:-}"

if [ -n "$MONGO_URI" ] && command -v mongodump >/dev/null 2>&1; then
  mkdir -p "$BACKUP_DIR/mongo"
  timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
  dest="$BACKUP_DIR/mongo/xvpn-$timestamp"
  mongodump --uri="$MONGO_URI" --out="$dest"
  tar -C "$BACKUP_DIR/mongo" -czf "$dest.tar.gz" "$(basename "$dest")"
  rm -rf "$dest"
  find "$BACKUP_DIR/mongo" -name 'xvpn-*.tar.gz' -mtime "+$RETENTION_DAYS" -delete
  echo "xvpn-backup: mongodump salvo em $dest.tar.gz"
elif [ -z "$MONGO_URI" ] && [ ! -f "$DB_PATH" ]; then
  echo "xvpn-backup: banco não encontrado em $DB_PATH, nada a fazer." >&2
  exit 0
fi

mkdir -p "$BACKUP_DIR"

if [ -z "$MONGO_URI" ] && [ -f "$DB_PATH" ]; then
  timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
  dest="$BACKUP_DIR/xvpn-$timestamp.db"
  sqlite3 "$DB_PATH" ".backup '$dest'"
  gzip "$dest"
  find "$BACKUP_DIR" -name 'xvpn-*.db.gz' -mtime "+$RETENTION_DAYS" -delete
  echo "xvpn-backup: snapshot salvo em $dest.gz"
fi

# Blobs do marketplace são content-addressed (nome = sha256, nunca mutam depois
# de escritos — ver server/internal/marketplace/storage.go), então um espelho
# incremental (`--delete` só remove o que a API já removeu de fato) é
# suficiente e muito mais barato que copiar/gzipar tudo a cada rodada como o
# xvpn.db acima. Não roda `gzip` aqui: os assets já são binários compactados
# (.deb/.exe/.apk/...) na grande maioria dos casos.
if [ -d "$MARKETPLACE_DIR" ]; then
  if command -v rsync >/dev/null 2>&1; then
    mkdir -p "$BACKUP_DIR/marketplace"
    rsync -a --delete "$MARKETPLACE_DIR/" "$BACKUP_DIR/marketplace/"
    echo "xvpn-backup: marketplace espelhado em $BACKUP_DIR/marketplace/"
  else
    echo "xvpn-backup: rsync não encontrado, pulando backup do marketplace." >&2
  fi
else
  echo "xvpn-backup: $MARKETPLACE_DIR não existe ainda, pulando backup do marketplace."
fi

if [ -d "$SOCIAL_MEDIA_DIR" ]; then
  if command -v rsync >/dev/null 2>&1; then
    mkdir -p "$BACKUP_DIR/social"
    rsync -a --delete "$SOCIAL_MEDIA_DIR/" "$BACKUP_DIR/social/"
    echo "xvpn-backup: social media espelhado em $BACKUP_DIR/social/"
  fi
fi

if [ -d "$PACKAGES_DIR" ]; then
  if command -v rsync >/dev/null 2>&1; then
    mkdir -p "$BACKUP_DIR/packages"
    rsync -a --delete "$PACKAGES_DIR/" "$BACKUP_DIR/packages/"
    echo "xvpn-backup: packages XGIT espelhado em $BACKUP_DIR/packages/"
  fi
fi
