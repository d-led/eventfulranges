#!/usr/bin/env bash
# Mutation testing with gremlins. Fails if the mutant kill rate drops below 80%.
set -euo pipefail
cd "$(dirname "$0")/.."

if ! command -v gremlins >/dev/null 2>&1; then
  echo "installing gremlins"
  go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0
fi

mkdir -p build/reports
gremlins unleash --threshold-efficacy 0.8 --output build/reports/gremlins.json ./...
