#!/bin/sh
set -eu
if command -v mvn >/dev/null 2>&1; then
  mvn -q -B -DskipTests package
else
  echo "mvn ausente — skip compile"
fi
