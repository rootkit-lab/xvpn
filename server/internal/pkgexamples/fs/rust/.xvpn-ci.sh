#!/bin/sh
set -eu
if command -v cargo >/dev/null 2>&1; then
  cargo test
else
  echo "cargo ausente — skip (exemplo generic)"
fi
