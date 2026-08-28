#!/usr/bin/env bash
# Runs the pure-Go Automerge demo: two replicas converge via the sync protocol.
set -euo pipefail
cd "$(dirname "$0")/.."

cd demo/go-automerge
go test -race -count=1 ./...
go run .
