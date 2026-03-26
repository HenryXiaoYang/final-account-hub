#!/bin/sh
set -eu

mkdir -p /app/data/venvs
chown -R appuser:appgroup /app/data
exec gosu appuser "$@"
