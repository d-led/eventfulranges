#!/usr/bin/env bash
# Development loop for the browser-only (WebAssembly) visualizer: builds the
# bundle once, then rebuilds it whenever the UI or the Go engine changes, and
# serves demo/web/dist-local with caching disabled. Open the printed URL;
# reload the page after a rebuild. Ctrl+C stops it.
#
# Not to be run alongside scripts/e2e-local.sh or scripts/serve-local.sh: they
# build and serve the same directory, and a rebuild mid-test makes them flaky.
set -euo pipefail
cd "$(dirname "$0")/.."

cd demo/web/ui-src
exec node serve-local.mjs "$@"
