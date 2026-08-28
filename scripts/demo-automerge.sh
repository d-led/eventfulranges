#!/usr/bin/env bash
# Runs the Automerge demo: two replicas converge via the Automerge sync protocol.
set -euo pipefail
cd "$(dirname "$0")/.."

cd demo/automerge
go test -race -count=1 ./...
go run .
