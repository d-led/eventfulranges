#!/usr/bin/env bash
# Runs all unit tests with the race detector and prints the library coverage
# report. The demo programs are smoke-tested here but excluded from the 100%
# coverage gate, which targets library code only.
set -euo pipefail
cd "$(dirname "$0")/.."

mkdir -p build

lib=$(go list ./... | grep -v '/demo/')
demo=$(go list ./... | grep '/demo/' || true)

echo "== library tests (with coverage)"
# shellcheck disable=SC2086
go test -race -count=1 -coverprofile=build/coverage.out $lib

echo
echo "== demo smoke tests"
if [ -n "$demo" ]; then
  # shellcheck disable=SC2086
  go test -race -count=1 $demo
fi

echo
echo "=== coverage report ==="
go tool cover -func=build/coverage.out
