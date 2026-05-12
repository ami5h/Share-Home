#!/bin/sh
set -e
chown -R appuser:appgroup /data 2>/dev/null || true
exec su appuser -s /bin/sh -c "$*"
