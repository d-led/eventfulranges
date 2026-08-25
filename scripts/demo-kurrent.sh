#!/usr/bin/env bash
# Runs the KurrentDB-backed demo (requires kurrent-up.sh first).
set -euo pipefail
cd "$(dirname "$0")/.."

export KURRENTDB_CONNECTION="${KURRENTDB_CONNECTION:-esdb://localhost:2113?tls=false}"

cd demo/kurrent
go test -tags kurrent -race -count=1 ./...
go run -tags kurrent .
