#!/usr/bin/env bash
# Property-based tests with more iterations than the default. Failure
# reproductions are written to build/rapid so they never pollute the tree.
set -euo pipefail
cd "$(dirname "$0")/.."

mkdir -p build/rapid
for pkg in $(go list ./...); do
  dir=$(go list -f '{{.Dir}}' "$pkg")
  if grep -Rqs --include='*_test.go' 'func TestProperty' "$dir"; then
    echo "property testing $pkg"
    go test -race -count=1 -run 'TestProperty' -rapid.checks=1000 -rapid.failfile=build/rapid/fail "$pkg"
  fi
done
