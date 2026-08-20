#!/bin/sh
set -eu
test -f hello.txt
grep -q hello hello.txt
