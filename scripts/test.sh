#!/usr/bin/env bash
# Runs all unit tests with the race detector and prints the coverage report.
set -euo pipefail
cd "$(dirname "$0")/.."

mkdir -p build
go test -race -count=1 -coverprofile=build/coverage.out ./...
echo
echo "=== coverage report ==="
go tool cover -func=build/coverage.out
