#!/usr/bin/env bash
# Benchmarks. Informational only; not part of the quality gate.
set -euo pipefail
cd "$(dirname "$0")/.."

go test -run '^$' -bench=. -benchmem ./...
