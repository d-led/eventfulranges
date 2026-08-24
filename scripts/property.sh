#!/usr/bin/env bash
# Property-based tests with more iterations than the default. Failure
# reproductions are written to build/rapid so they never pollute the tree.
set -euo pipefail
cd "$(dirname "$0")/.."

mkdir -p build/rapid
failfile="$(pwd)/build/rapid/fail"

for pkg in $(go list ./...); do
  dir=$(go list -f '{{.Dir}}' "$pkg")
  if grep -qs -- 'func TestProperty' "$dir"/*_test.go 2>/dev/null; then
    echo "property testing $pkg"
    # Custom -rapid.* flags belong to the test binary and must follow the
    # package argument, otherwise `go test` forwards them incorrectly.
    go test -race -count=1 -run 'TestProperty' "$pkg" -rapid.checks=1000 -rapid.failfile="$failfile"
  fi
done
