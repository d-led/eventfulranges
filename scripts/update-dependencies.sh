#!/usr/bin/env bash
# Upgrades every dependency in each Go module to the latest version, tidies the
# module files, and verifies the result with the race detector. The workspace
# holds two modules: the library at the root and the demo app in ./demo.
set -euo pipefail
cd "$(dirname "$0")/.."

for mod in "." "demo"; do
  echo "== upgrade $mod"
  (
    cd "$mod"
    go get -u ./...
    go get -u -t ./...
    go mod tidy
    go test -race -count=1 ./...
  )
done

# Refresh the workspace checksums after the module files changed.
go work sync

echo "dependencies upgraded"
