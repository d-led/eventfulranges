#!/usr/bin/env bash
# Integration tests against a running KurrentDB (see kurrent-up.sh).
set -euo pipefail
cd "$(dirname "$0")/.."

mkdir -p build
go test -tags kurrent -race -count=1 -coverprofile=build/coverage-kurrent.out ./...

total=$(go tool cover -func=build/coverage-kurrent.out | tail -n 1 | awk '{print $NF}')
if [ "$total" != "100.0%" ]; then
  echo "kurrent coverage gate failed: total is $total, want 100.0%" >&2
  exit 1
fi
echo "kurrent coverage gate passed: $total"
