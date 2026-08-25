#!/usr/bin/env bash
# Rebuilds the embedded web UI: installs the JS dependencies and bundles
# demo/web/ui-src into demo/web/dist (which is committed and go:embedded).
set -euo pipefail
cd "$(dirname "$0")/.."

cd demo/web
go generate .
