#!/usr/bin/env bash
# Rebuilds the embedded web UI: installs the JS dependencies and bundles
# demo/web/ui-src into demo/web/dist (generated; go:embedded with -tags embed).
# The dist/ output is gitignored except for the directory marker and the
# "run the build" placeholder index.html.
set -euo pipefail
cd "$(dirname "$0")/.."

cd demo/web
go generate .
