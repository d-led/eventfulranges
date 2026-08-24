#!/usr/bin/env bash
# Property-based tests with more iterations than the default.
set -euo pipefail
cd "$(dirname "$0")/.."

go test -race -count=1 -run 'TestProperty' -rapid.checks=1000 ./...
