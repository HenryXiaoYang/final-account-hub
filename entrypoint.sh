#!/bin/sh
mkdir -p /app/data/venvs
chown -R appuser:appgroup /app/data
exec su-exec appuser "$@"
