#!/bin/sh
set -eu
PYTHONPATH=src python3 -c "from hello_ihuull import greet; assert greet('py') == 'hello py'"
