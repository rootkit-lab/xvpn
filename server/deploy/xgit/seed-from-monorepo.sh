#!/bin/bash
# Importa apps/ do monorepo para bare XGIT vazios. Uso no VPS:
#   seed-from-monorepo.sh /opt/xvpn/data/git [src-clone]
# Sem segundo argumento clona https://github.com/rootkit-lab/xvpn (público).
set -euo pipefail

GIT_ROOT="${1:-/opt/xvpn/data/git}"
SRC="${2:-}"
OWNER="${XVPN_SEED_NAME:-xbot}"
EMAIL="${XVPN_SEED_EMAIL:-xbot@corp.ihuull.com}"

cleanup=""
if [ -z "$SRC" ]; then
  SRC="$(mktemp -d /tmp/xvpn-monorepo-XXXX)"
  cleanup="$SRC"
  git clone --depth 1 https://github.com/rootkit-lab/xvpn.git "$SRC"
fi

seed_one() {
  local slug="$1"
  local prefix="$2"
  local bare="$GIT_ROOT/${slug}.git"
  local work
  if [ ! -d "$bare" ]; then
    echo "skip $slug — bare inexistente"
    return 0
  fi
  if git --git-dir="$bare" rev-parse --verify -q refs/heads/main >/dev/null 2>&1; then
    echo "skip $slug — main já tem commits"
    return 0
  fi
  if [ ! -d "$SRC/$prefix" ]; then
    echo "erro: $SRC/$prefix não existe" >&2
    return 1
  fi
  work="$(mktemp -d /tmp/xgit-seed-${slug}-XXXX)"
  git -C "$work" init --initial-branch=main
  cp -a "$SRC/$prefix/." "$work/"
  git -C "$work" add -A
  if git -C "$work" diff --cached --quiet; then
    echo "skip $slug — nada para commitar"
    rm -rf "$work"
    return 0
  fi
  git -C "$work" -c user.name="$OWNER" -c user.email="$EMAIL" commit -m "chore: import ${prefix} do monorepo"
  git -c safe.directory='*' --git-dir="$bare" fetch "$work" '+refs/heads/main:refs/heads/main'
  rm -rf "$work"
  echo "ok $slug ← $prefix"
}

seed_one xvpn-client apps/xvpn-client
seed_one xchat apps/xvpn-chat
# xcorp/xvpn (plataforma) é seed do boot do xvpn-server (Fase 66) — bare fica
# vazio até um mirror/import separado; não misturar com apps/ aqui.

if [ -n "$cleanup" ]; then
  rm -rf "$cleanup"
fi
