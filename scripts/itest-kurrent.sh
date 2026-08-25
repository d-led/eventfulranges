#!/usr/bin/env bash
# Integration tests against a running KurrentDB (see kurrent-up.sh).
set -euo pipefail
cd "$(dirname "$0")/.."

mkdir -p build
export KURRENTDB_CONNECTION="${KURRENTDB_CONNECTION:-esdb://localhost:2113?tls=false}"
go test -tags kurrent -race -count=1 -coverprofile=build/coverage-kurrent.out ./...

total=$(go tool cover -func=build/coverage-kurrent.out | tail -n 1 | awk '{print $NF}')
echo "kurrent coverage: $total"
