#!/usr/bin/env bash
# Starts the local KurrentDB for development and integration tests.
set -euo pipefail
cd "$(dirname "$0")/.."

docker compose up -d

echo -n "waiting for KurrentDB"
for _ in $(seq 1 60); do
  if curl -fsS http://localhost:2113/health/live >/dev/null 2>&1; then
    echo
    echo "KurrentDB is up"
    exit 0
  fi
  echo -n .
  sleep 1
done
echo
echo "KurrentDB did not become ready in time" >&2
exit 1
