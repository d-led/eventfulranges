#!/usr/bin/env bash
# The single quality gate: formatting, static analysis, unit tests with the
# 100% coverage gate, property tests, fuzz smoke tests, mutation testing, the
# embedded-UI builds, the WebAssembly build, and the Playwright end-to-end
# suites for every demo. Exits 0 only when everything is green.
#
# Deliberately excluded: the Kurrent (EventStore) integration test, which
# needs an external service started by scripts/kurrent-up.sh and has its own
# CI job. Everything else — library, every demo's Go tests, the web and paint
# UI builds, the browser-only WebAssembly engine, and all Playwright suites —
# runs here.
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

echo "== demos: web UI (unit + e2e over the Go server)"
./scripts/e2e-web.sh

echo "== demos: web UI (unit + e2e over the WebAssembly engine)"
./scripts/e2e-local.sh

echo "== demos: paint UI (unit + e2e)"
./scripts/e2e-paint.sh

echo "QUALITY GATE PASSED"
