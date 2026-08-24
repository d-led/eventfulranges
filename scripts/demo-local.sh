#!/usr/bin/env bash
# Runs the non-networked demo: goroutine replicas converge via in-memory sync.
set -euo pipefail
cd "$(dirname "$0")/.."

cd demo/local
go test -race -count=1 ./...
go run . -seed 42
