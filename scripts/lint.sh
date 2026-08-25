#!/usr/bin/env bash
# Static analysis for the core library and the demo module, each in its own
# module so the linter resolves their dependency graphs independently.
set -euo pipefail
cd "$(dirname "$0")/.."

go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.1 run ./...
(cd demo && go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.1 run ./...)
