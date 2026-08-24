#!/usr/bin/env bash
# Static analysis: golangci-lint with complexity and duplicate-code checks.
set -euo pipefail
cd "$(dirname "$0")/.."

go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.1 run ./...
