#!/usr/bin/env bash
# Smoke-runs every fuzz target for a short time.
set -euo pipefail
cd "$(dirname "$0")/.."

for pkg in $(go list ./...); do
  dir=$(go list -f '{{.Dir}}' "$pkg")
  if grep -Rqs --include='*_test.go' 'func Fuzz' "$dir"; then
    echo "fuzzing $pkg"
    go test -run '^$' -fuzz=Fuzz -fuzztime=30s "$pkg"
  fi
done
