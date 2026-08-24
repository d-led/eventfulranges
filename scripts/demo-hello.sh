#!/usr/bin/env bash
# Runs the smallest demo: plain usage with no concurrency at all.
set -euo pipefail
cd "$(dirname "$0")/.."

cd demo/hello
go test -race -count=1 ./...
go run .
