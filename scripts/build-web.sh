#!/usr/bin/env bash
# Rebuilds the embedded web UI: installs the JS dependencies and bundles
# demo/web/ui-src into demo/web/dist (generated; go:embedded with -tags embed).
# dist/ is gitignored except for the directory marker, so rebuilding never
# dirties tracked files.
set -euo pipefail
cd "$(dirname "$0")/.."

cd demo/web
go generate .
