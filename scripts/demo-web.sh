#!/usr/bin/env bash
# Builds and smoke-tests the web visualizer (demo/web).
set -euo pipefail
cd "$(dirname "$0")/.."

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
  if curl -fsS "http://127.0.0.1:${port}/ui/" >/dev/null 2>&1; then
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
