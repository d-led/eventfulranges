#!/usr/bin/env bash
# The single quality gate: formatting, static analysis, unit tests with the
# 100% coverage gate, property tests, fuzz smoke tests and mutation testing.
# Exits 0 only when everything is green.
set -euo pipefail
cd "$(dirname "$0")/.."

echo "== format"
./scripts/fmt.sh

echo "== lint"
./scripts/lint.sh

echo "== tests + coverage"
./scripts/test.sh

total=$(go tool cover -func=build/coverage.out | tail -n 1 | awk '{print $NF}')
echo "coverage: $total (strive for 100.0%)"

echo "== property tests"
./scripts/property.sh

echo "== fuzz smoke"
./scripts/fuzz.sh

echo "== mutation testing"
./scripts/mutation.sh

echo "QUALITY GATE PASSED"
