#!/usr/bin/env bash
# Runs the standalone calendar demo, which imports the published module
# github.com/d-led/eventfulranges@v0.0.1 by version (no replace directive).
# GOWORK=off keeps it independent of the repository's root go.work, so the
# versioned import is resolved from the module proxy rather than the local
# checkout.
set -euo pipefail
cd "$(dirname "$0")/.."

cd examples/calendar
GOWORK=off go run .
