#!/usr/bin/env bash
# Runs the core library tests with coverage and the demo module tests without
# coverage. The demos live in their own module (demo/go.mod), so the 100%
# coverage gate stays scoped to library code only.
set -euo pipefail
cd "$(dirname "$0")/.."

mkdir -p build

echo "== library tests (with coverage)"
go test -race -count=1 -coverprofile=build/coverage.out ./...

echo
echo "== demo smoke tests"
(cd demo && go test -race -count=1 ./...)

echo
echo "=== coverage report ==="
go tool cover -func=build/coverage.out
