#!/usr/bin/env bash
# Runs the networked demo: two HTTP peers converge.
set -euo pipefail
cd "$(dirname "$0")/.."

cd demo/network
go test -race -count=1 ./...
go run . -ports 18080,18081
