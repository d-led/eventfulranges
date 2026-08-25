#!/usr/bin/env bash
# Builds the web UI and smoke-tests the visualizer (demo/web) without a browser.
set -euo pipefail
cd "$(dirname "$0")/.."

./scripts/build-web.sh

cd demo/web
go test -race -count=1 ./...

bin=$(mktemp)
pid=""
trap 'rm -f "$bin"; [ -z "$pid" ] || kill "$pid" 2>/dev/null || true' EXIT

go build -o "$bin" .

port=18080
"$bin" -addr "127.0.0.1:${port}" &
pid=$!

ready=0
for _ in $(seq 1 50); do
  if curl -fsSL "http://127.0.0.1:${port}/ui/" >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 0.1
done

if [ "$ready" -ne 1 ]; then
  echo "web server did not start" >&2
  exit 1
fi

echo "web demo smoke test OK"
