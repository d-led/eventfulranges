#!/usr/bin/env bash
# Smoke-runs every fuzz target for a short time. Each target is fuzzed on its
# own because `-fuzz` must match exactly one target per invocation.
set -euo pipefail
cd "$(dirname "$0")/.."

for pkg in $(go list ./...); do
  dir=$(go list -f '{{.Dir}}' "$pkg")
  targets=$(grep -ho 'func Fuzz[A-Za-z0-9_]*' "$dir"/*_test.go 2>/dev/null | awk '{print $2}' || true)
  for name in $targets; do
    echo "fuzzing $pkg $name"
    go test -race -run '^$' -fuzz="^${name}$" -fuzztime=30s "$pkg"
  done
done
