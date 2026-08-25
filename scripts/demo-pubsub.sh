#!/usr/bin/env bash
# Runs the in-process pub/sub demo: replicas converge over a pub/sub bus.
set -euo pipefail
cd "$(dirname "$0")/.."

cd demo/pubsub
go test -race -count=1 ./...
go run .
