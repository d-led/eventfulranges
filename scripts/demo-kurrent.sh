#!/usr/bin/env bash
# Runs the KurrentDB-backed demo (requires kurrent-up.sh first).
set -euo pipefail
cd "$(dirname "$0")/.."

cd demo/kurrent
go test -race -count=1 ./...
go run .
